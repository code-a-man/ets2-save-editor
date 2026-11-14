// Package bsii decodes SCS Software's binary SII format ("BSII") into
// plain SiiNunit text — the same human-readable syntax a save written
// with g_save_format "0" already uses.
//
// Ported from github.com/CoffeSiberian/DecryptTruck (Rust, MIT license,
// copyright (c) 2024 Fernando Garrido, github.com/SiberianCoffe), v1.3.7
// — chosen over writing an original implementation because that
// project's own decoded output was independently verified against a
// real ~270,000-line ETS2 save (18,843 units, 0 parser warnings,
// correct HQ city/fleet/garage/driver counts once fed through a plain-
// text SII parser), and its raw byte output matched this port's,
// byte-for-byte, on the same real input. This package is a line-by-
// line port of its decode/format logic, not a reimplementation from
// the format's general description — the byte-level algorithms (in
// particular decodeEncodedString's base-38 alphabet and decodeID's
// "nameless" hex-address reconstruction) are non-obvious enough that
// re-deriving them independently would risk silently getting them
// wrong in a way nothing would catch. See README.md for the full
// provenance/validation writeup.
package bsii

// dataType mirrors DecryptTruck's DataTypeIdFormat enum exactly —
// see https://github.com/CoffeSiberian/DecryptTruck/blob/main/src/strucs/sii_types.rs
type dataType uint32

const (
	typeUTF8String             dataType = 1
	typeArrayOfUTF8String      dataType = 2
	typeEncodedString          dataType = 3
	typeArrayOfEncodedString   dataType = 4
	typeSingle                 dataType = 5
	typeArrayOfSingle          dataType = 6
	typeVectorOf2Single        dataType = 7
	typeArrayOfVectorOf2Single dataType = 8
	typeVectorOf3Single        dataType = 9
	typeArrayOfVectorOf3Single dataType = 10
	typeVectorOf3Int32         dataType = 17
	typeArrayOfVectorOf3Int32  dataType = 18
	typeVectorOf4Single        dataType = 23
	typeArrayOfVectorOf4Single dataType = 24
	typeVectorOf8Single        dataType = 25
	typeArrayOfVectorOf8Single dataType = 26
	typeInt32                  dataType = 37
	typeArrayOfInt32           dataType = 38
	typeUInt32                 dataType = 39
	typeArrayOfUInt32          dataType = 40
	typeInt16                  dataType = 41
	typeArrayOfInt16           dataType = 42
	typeUInt16                 dataType = 43
	typeArrayOfUInt16          dataType = 44
	typeUInt32Type2            dataType = 47
	typeInt64                  dataType = 49
	typeArrayOfInt64           dataType = 50
	typeUInt64                 dataType = 51
	typeArrayOfUInt64          dataType = 52
	typeByteBool               dataType = 53
	typeArrayOfByteBool        dataType = 54
	typeOrdinalString          dataType = 55
	typeID                     dataType = 57
	typeArrayOfIDA             dataType = 58
	typeIDType2                dataType = 59
	typeArrayOfIDC             dataType = 60
	typeIDType3                dataType = 61
	typeArrayOfIDE             dataType = 62
	typeArrayOfVectorOf2Int32  dataType = 65
)

const (
	bsiiVersion1 uint32 = 1
	bsiiVersion2 uint32 = 2
	bsiiVersion3 uint32 = 3
)

// id is a decoded 0x39/0x3B/0x3D identifier: either a dotted path built
// from encoded-string parts ("company.volatile.x.y"), "null" (zero
// parts), or a "_nameless.xx.xxxx.xxxx"-style reconstructed hex address.
type id struct {
	Value string
}

type vec2 struct{ A, B float32 }
type vec3 struct{ A, B, C float32 }
type vec4 struct{ A, B, C, D float32 }

// vec7/vec8 are ETS2's "placement" encoding: a position (A,B,C) plus a
// quaternion. vec8's D component is quantization bias applied to A/C
// during decode (see decodeVec8) and is never itself serialized.
type vec7 struct{ A, B, C, D, E, F, G float32 }
type vec8 struct{ A, B, C, D, E, F, G, H float32 }

type int32Vec2 struct{ A, B int32 }
type int32Vec3 struct{ A, B, C int32 }

// fieldDef is one field declaration inside a structure block: a name
// and a data type, in declaration order. Data blocks referencing this
// structure supply one value per fieldDef, in this same order.
type fieldDef struct {
	Name string
	Type dataType
}

// structureBlock is a schema declaration (BSII "block_type == 0"):
// what fields a given structureID's data blocks contain, and (for any
// OrdinalString field) the string table those values index into.
type structureBlock struct {
	StructureID    uint32
	Name           string
	Fields         []fieldDef
	OrdinalStrings map[uint32]string // only set for the first OrdinalString field found
}

// decodedField is one resolved field value on a decoded data block,
// pre-formatted into the exact text DecryptTruck's serializer would
// have emitted (see format.go) — this keeps decode and dispatch
// together the same way the reference implementation does, rather
// than introducing an intermediate typed-value representation this
// port doesn't need.
type decodedField struct {
	Name string
	Type dataType
	Text []string // one element for a scalar, N for an array
}

// decodedBlock is one fully-decoded unit instance.
type decodedBlock struct {
	OrderPos    uint32
	StructureID uint32
	Name        string
	ID          string
	Fields      []decodedField
}
