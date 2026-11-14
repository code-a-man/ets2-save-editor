package bsii

import (
	"fmt"
	"strconv"
	"strings"
)

// Decode turns a raw BSII binary payload (signature "BSII") into plain
// SiiNunit text, byte-for-byte in the same textual conventions
// a plain-text SII parser already parses. See the package doc comment
// for provenance; see README.md for how
// this was verified.
func Decode(raw []byte) ([]byte, error) {
	c := &cursor{data: raw}

	if _, err := c.u32(); err != nil { // signature, already checked by the caller
		return nil, fmt.Errorf("reading BSII signature: %w", err)
	}
	version, err := c.u32()
	if err != nil {
		return nil, fmt.Errorf("reading BSII version: %w", err)
	}
	if version != bsiiVersion1 && version != bsiiVersion2 && version != bsiiVersion3 {
		return nil, fmt.Errorf("unsupported BSII version %d", version)
	}

	structures := make(map[uint32]structureBlock)

	// Phase 1: structure (schema) blocks, block_type == 0, until the
	// first non-zero block_type — which is the first data block and
	// ends this phase.
	var blockType uint32
	for {
		if c.remaining() <= 0 {
			return nil, fmt.Errorf("unexpected end of file while reading structure blocks")
		}
		blockType, err = c.u32()
		if err != nil {
			return nil, err
		}
		if blockType != 0 {
			break
		}
		decl, err := readStructureBlock(c)
		if err != nil {
			return nil, err
		}
		if !decl.Valid {
			// Matches the reference decoder: an invalid structure-block
			// marker doesn't end this phase, it just doesn't register
			// a structure — keep reading more structure blocks.
			continue
		}
		if _, exists := structures[decl.StructureID]; !exists {
			structures[decl.StructureID] = structureBlock{
				StructureID:    decl.StructureID,
				Name:           decl.Name,
				Fields:         decl.Fields,
				OrdinalStrings: decl.OrdinalStrings,
			}
		}
	}

	// Phase 2: data blocks. blockType already holds the first one's
	// structure id (read at the end of phase 1); subsequent iterations
	// read a fresh one.
	var decoded []decodedBlock
	var orderPos uint32
	firstIteration := true
	for {
		if c.remaining() <= 0 {
			break
		}
		if firstIteration {
			firstIteration = false
		} else {
			blockType, err = c.u32()
			if err != nil {
				return nil, err
			}
		}

		if blockType == 0 {
			valid, err := c.boolean()
			if err != nil {
				return nil, err
			}
			if !valid {
				break
			}
			// validity==true with block_type==0 falls through to the
			// structure lookup below in the reference implementation,
			// which finds it only if an earlier *invalid* structure
			// placeholder happened to be recorded under id 0. This port
			// doesn't track invalid placeholders (see readStructureBlock)
			// so this combination fails the lookup below instead —
			// neither real fixture this was built against ever hits
			// this path; see README.md.
		}

		orderPos++
		block, ok := structures[blockType]
		if !ok {
			return nil, fmt.Errorf("data block at position %d references unknown structure id %d", orderPos, blockType)
		}

		unitID, err := decodeID(c)
		if err != nil {
			return nil, err
		}

		db := decodedBlock{OrderPos: orderPos, StructureID: block.StructureID, Name: block.Name, ID: unitID.Value}
		for _, f := range block.Fields {
			text, err := decodeField(c, f.Type, version, block.OrdinalStrings, f.Name)
			if err != nil {
				return nil, fmt.Errorf("unit %s (%s), field %s: %w", unitID.Value, block.Name, f.Name, err)
			}
			db.Fields = append(db.Fields, decodedField{Name: f.Name, Type: f.Type, Text: text})
		}
		decoded = append(decoded, db)
	}

	return serializeDocument(decoded), nil
}

// readFieldDecl is one field-type declaration line read while parsing
// a structure block: a type id, a name (unless the type is the 0
// terminator), and — only for the block's first OrdinalString field —
// that field's string lookup table.
type readFieldDecl struct {
	Type           dataType
	Name           string
	OrdinalStrings map[uint32]string
}

func readFieldDecl_(c *cursor) (readFieldDecl, error) {
	t, err := c.u32()
	if err != nil {
		return readFieldDecl{}, err
	}
	result := readFieldDecl{Type: dataType(t)}
	if t != 0 {
		name, err := c.utf8String()
		if err != nil {
			return readFieldDecl{}, err
		}
		result.Name = name
	}
	if dataType(t) == typeOrdinalString {
		m, err := readOrdinalStringList(c)
		if err != nil {
			return readFieldDecl{}, err
		}
		result.OrdinalStrings = m
	}
	return result, nil
}

func readOrdinalStringList(c *cursor) (map[uint32]string, error) {
	n, err := c.u32()
	if err != nil {
		return nil, err
	}
	m := make(map[uint32]string, n)
	for i := uint32(0); i < n; i++ {
		ordinal, err := c.u32()
		if err != nil {
			return nil, err
		}
		s, err := c.utf8String()
		if err != nil {
			return nil, err
		}
		m[ordinal] = s
	}
	return m, nil
}

type structDecl struct {
	Valid          bool
	StructureID    uint32
	Name           string
	Fields         []fieldDef
	OrdinalStrings map[uint32]string
}

func readStructureBlock(c *cursor) (structDecl, error) {
	valid, err := c.boolean()
	if err != nil {
		return structDecl{}, err
	}
	if !valid {
		return structDecl{Valid: false}, nil
	}

	structureID, err := c.u32()
	if err != nil {
		return structDecl{}, err
	}
	name, err := c.utf8String()
	if err != nil {
		return structDecl{}, err
	}

	decl := structDecl{Valid: true, StructureID: structureID, Name: name}
	for {
		fd, err := readFieldDecl_(c)
		if err != nil {
			return structDecl{}, err
		}
		if fd.Type == 0 {
			break
		}
		decl.Fields = append(decl.Fields, fieldDef{Name: fd.Name, Type: fd.Type})
		if fd.Type == typeOrdinalString && decl.OrdinalStrings == nil {
			decl.OrdinalStrings = fd.OrdinalStrings
		}
	}
	return decl, nil
}

// decodeField decodes one field's value(s) from the cursor and returns
// it already formatted as SII text — mirrors
// DecryptTruck's load_data_block_local dispatch, merged with its
// bsii_serializer formatting so each case is defined once. See
// README.md for the per-type nil-
// sentinel/quoting asymmetries this preserves deliberately (e.g.
// ArrayOfUInt32 has a "nil" sentinel, ArrayOfUInt64 does not, even
// though their scalar forms both do).
func decodeField(c *cursor, t dataType, version uint32, ordinals map[uint32]string, fieldName string) ([]string, error) {
	switch t {
	case typeUTF8String:
		s, err := c.utf8String()
		if err != nil {
			return nil, err
		}
		return []string{formatUTF8Scalar(s)}, nil

	case typeArrayOfUTF8String:
		return decodeArray(c, func(c *cursor) (string, error) {
			s, err := c.utf8String()
			if err != nil {
				return "", err
			}
			return formatUTF8ArrayElem(s), nil
		})

	case typeEncodedString, typeOrdinalString:
		var s string
		var err error
		if t == typeOrdinalString {
			s, err = decodeOrdinalString(c, ordinals, fieldName)
		} else {
			s, err = c.encodedString()
		}
		if err != nil {
			return nil, err
		}
		return []string{formatOrdinalOrEncodedScalar(fieldName, s)}, nil

	case typeArrayOfEncodedString:
		return decodeArray(c, func(c *cursor) (string, error) { return c.encodedString() })

	case typeSingle:
		f, err := c.f32()
		if err != nil {
			return nil, err
		}
		return []string{singleToString(f)}, nil

	case typeArrayOfSingle:
		return decodeArray(c, func(c *cursor) (string, error) {
			f, err := c.f32()
			return singleToString(f), err
		})

	case typeVectorOf2Single:
		v, err := c.vec2()
		if err != nil {
			return nil, err
		}
		return []string{vec2ToString(v)}, nil

	case typeArrayOfVectorOf2Single:
		return decodeArray(c, func(c *cursor) (string, error) {
			v, err := c.vec2()
			return vec2ToString(v), err
		})

	case typeVectorOf3Single:
		v, err := c.vec3()
		if err != nil {
			return nil, err
		}
		return []string{vec3ToString(v)}, nil

	case typeArrayOfVectorOf3Single:
		return decodeArray(c, func(c *cursor) (string, error) {
			v, err := c.vec3()
			return vec3ToString(v), err
		})

	case typeVectorOf3Int32:
		v, err := c.int32Vec3()
		if err != nil {
			return nil, err
		}
		return []string{int32Vec3ToString(v)}, nil

	case typeArrayOfVectorOf3Int32:
		return decodeArray(c, func(c *cursor) (string, error) {
			v, err := c.int32Vec3()
			return int32Vec3ToString(v), err
		})

	case typeVectorOf4Single:
		v, err := c.vec4()
		if err != nil {
			return nil, err
		}
		return []string{vec4ToString(v)}, nil

	case typeArrayOfVectorOf4Single:
		return decodeArray(c, func(c *cursor) (string, error) {
			v, err := c.vec4()
			return vec4ToString(v), err
		})

	case typeVectorOf8Single:
		if version == bsiiVersion1 {
			v, err := c.vec7()
			if err != nil {
				return nil, err
			}
			return []string{vec7ToString(v)}, nil
		}
		v, err := c.vec8()
		if err != nil {
			return nil, err
		}
		return []string{vec8ToString(v)}, nil

	case typeArrayOfVectorOf8Single:
		if version == bsiiVersion1 {
			return decodeArray(c, func(c *cursor) (string, error) {
				v, err := c.vec7()
				return vec7ToString(v), err
			})
		}
		return decodeArray(c, func(c *cursor) (string, error) {
			v, err := c.vec8()
			return vec8ToString(v), err
		})

	case typeInt32:
		v, err := c.i32()
		if err != nil {
			return nil, err
		}
		return []string{strconv.FormatInt(int64(v), 10)}, nil

	case typeArrayOfInt32:
		return decodeArray(c, func(c *cursor) (string, error) {
			v, err := c.i32()
			return strconv.FormatInt(int64(v), 10), err
		})

	case typeUInt32, typeUInt32Type2:
		v, err := c.u32()
		if err != nil {
			return nil, err
		}
		return []string{u32Text(v)}, nil

	case typeArrayOfUInt32:
		return decodeArray(c, func(c *cursor) (string, error) {
			v, err := c.u32()
			return u32Text(v), err
		})

	case typeInt16:
		v, err := c.i16()
		if err != nil {
			return nil, err
		}
		return []string{i16Text(v)}, nil

	case typeArrayOfInt16:
		return decodeArray(c, func(c *cursor) (string, error) {
			v, err := c.i16()
			return strconv.FormatInt(int64(v), 10), err
		})

	case typeUInt16:
		v, err := c.u16()
		if err != nil {
			return nil, err
		}
		return []string{u16Text(v)}, nil

	case typeArrayOfUInt16:
		return decodeArray(c, func(c *cursor) (string, error) {
			v, err := c.u16()
			return strconv.FormatUint(uint64(v), 10), err
		})

	case typeInt64:
		v, err := c.i64()
		if err != nil {
			return nil, err
		}
		return []string{strconv.FormatInt(v, 10)}, nil

	case typeArrayOfInt64:
		return decodeArray(c, func(c *cursor) (string, error) {
			v, err := c.i64()
			return strconv.FormatInt(v, 10), err
		})

	case typeUInt64:
		v, err := c.u64()
		if err != nil {
			return nil, err
		}
		return []string{u64Text(v)}, nil

	case typeArrayOfUInt64:
		return decodeArray(c, func(c *cursor) (string, error) {
			v, err := c.u64()
			return strconv.FormatUint(v, 10), err
		})

	case typeByteBool:
		v, err := c.boolean()
		if err != nil {
			return nil, err
		}
		return []string{strconv.FormatBool(v)}, nil

	case typeArrayOfByteBool:
		return decodeArray(c, func(c *cursor) (string, error) {
			v, err := c.boolean()
			return strconv.FormatBool(v), err
		})

	case typeID, typeIDType2, typeIDType3:
		v, err := decodeID(c)
		if err != nil {
			return nil, err
		}
		return []string{v.Value}, nil

	case typeArrayOfIDA, typeArrayOfIDC, typeArrayOfIDE:
		return decodeArray(c, func(c *cursor) (string, error) {
			v, err := decodeID(c)
			return v.Value, err
		})

	case typeArrayOfVectorOf2Int32:
		// Despite the name, the reference decoder treats this as a
		// single value, not an array — see the dispatcher's doc
		// comment and README.md.
		v, err := c.int32Vec2()
		if err != nil {
			return nil, err
		}
		return []string{int32Vec2ToString(v)}, nil

	default:
		return nil, fmt.Errorf("unknown BSII data type %d", t)
	}
}

// decodeArray reads a u32 element count then calls decodeOne that many
// times, matching every ArrayOf* case's shared "count prefix, then N
// elements" layout.
func decodeArray(c *cursor, decodeOne func(*cursor) (string, error)) ([]string, error) {
	n, err := c.u32()
	if err != nil {
		return nil, err
	}
	result := make([]string, n)
	for i := uint32(0); i < n; i++ {
		s, err := decodeOne(c)
		if err != nil {
			return nil, err
		}
		result[i] = s
	}
	return result, nil
}

// decodeOrdinalString reads the u32 index into an OrdinalString field
// and resolves it against that structure's string table, with the
// same two field-name special cases (lashing_method/lashing_gear_type
// always resolve to "none") the reference decoder hard-codes for
// fields whose ordinal table doesn't otherwise resolve correctly.
func decodeOrdinalString(c *cursor, ordinals map[uint32]string, fieldName string) (string, error) {
	index, err := c.u32()
	if err != nil {
		return "", err
	}
	if fieldName == "lashing_method" || fieldName == "lashing_gear_type" {
		return "none", nil
	}
	if s, ok := ordinals[index]; ok {
		return s, nil
	}
	return "", nil
}

// isArrayType reports whether t renders as "name: count\n" + N
// "name[i]: value\n" lines (matching bsii_serializer.rs's serialize_vec
// arm) rather than a single "name: value\n" line. This is a property
// of the type, not of how many elements happen to decode — a
// single-element array must still render with a "[0]" index, or
// a plain-text SII parser would file it under Attributes instead
// of Arrays and normalize's player.trucks[]-style resolution would
// silently miss it.
//
// typeArrayOfVectorOf2Int32 is deliberately excluded: despite its
// name, the reference decoder always renders it as a scalar (see
// decodeField's comment on that case).
func isArrayType(t dataType) bool {
	switch t {
	case typeArrayOfByteBool, typeArrayOfEncodedString,
		typeArrayOfIDA, typeArrayOfIDC, typeArrayOfIDE,
		typeArrayOfInt32, typeArrayOfInt64, typeArrayOfInt16,
		typeArrayOfSingle, typeArrayOfUInt16, typeArrayOfUInt32, typeArrayOfUInt64,
		typeArrayOfUTF8String,
		typeArrayOfVectorOf2Single, typeArrayOfVectorOf3Single, typeArrayOfVectorOf4Single, typeArrayOfVectorOf8Single,
		typeArrayOfVectorOf3Int32:
		return true
	default:
		return false
	}
}

// serializeDocument renders decoded blocks as plain SiiNunit text,
// matching bsii_serializer's output format exactly: a top-level
// "SiiNunit\n{\n...}\n" wrapper, one "<type> : <id> {\n...}\n\n" block
// per unit in decode order, and any block with an empty type name or
// empty ID silently dropped (both indicate a structure/data lookup
// that didn't fully resolve, not a unit worth keeping).
func serializeDocument(blocks []decodedBlock) []byte {
	var b strings.Builder
	b.WriteString("SiiNunit\n{\n")

	for _, block := range blocks {
		if block.Name == "" || block.ID == "" {
			continue
		}
		b.WriteString(block.Name)
		b.WriteString(" : ")
		b.WriteString(block.ID)
		b.WriteString(" {\n")
		for _, f := range block.Fields {
			if f.Type == 0 {
				continue
			}
			if !isArrayType(f.Type) {
				b.WriteString(" ")
				b.WriteString(f.Name)
				b.WriteString(": ")
				b.WriteString(f.Text[0])
				b.WriteString("\n")
				continue
			}
			b.WriteString(" ")
			b.WriteString(f.Name)
			b.WriteString(": ")
			b.WriteString(strconv.Itoa(len(f.Text)))
			b.WriteString("\n")
			for i, v := range f.Text {
				b.WriteString(" ")
				b.WriteString(f.Name)
				b.WriteString("[")
				b.WriteString(strconv.Itoa(i))
				b.WriteString("]: ")
				b.WriteString(v)
				b.WriteString("\n")
			}
		}
		b.WriteString("}\n\n")
	}

	b.WriteString("}")
	return []byte(b.String())
}
