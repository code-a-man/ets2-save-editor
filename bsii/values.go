package bsii

import (
	"fmt"
	"strings"
)

// charTable mirrors DecryptTruck's CHAR_TABLE: 10 digits + 26 lowercase
// letters + underscore, 37 symbols, used by the base-38 encoded-string
// format (type 0x03/0x04 and the non-nameless branch of decodeID).
const charTable = "0123456789abcdefghijklmnopqrstuvwxyz_"

// decodeEncodedString decodes SII's "string packed into a 64-bit
// integer" format (0x03 EncodedString). It's a base-38 positional
// encoding: each base-38 "digit" (value%38) maps to charTable[digit-1],
// with digit 0 meaning "no character here" (padding). Characters come
// out least-significant-digit first and are appended in that order —
// this only produces the correct forward-order string because the
// encoder that originally packed it wrote characters in the matching
// reverse convention; this is a faithful port, not a re-derivation.
func decodeEncodedString(v uint64) string {
	var buf []byte
	for v != 0 {
		idx := int64(v%38) - 1
		v /= 38
		if idx >= 0 && idx < int64(len(charTable)) {
			buf = append(buf, charTable[idx])
		}
	}
	return string(buf)
}

func (c *cursor) encodedString() (string, error) {
	v, err := c.u64()
	if err != nil {
		return "", err
	}
	return decodeEncodedString(v), nil
}

// decodeID decodes a 0x39/0x3B/0x3D identifier. The first byte is a
// "part count": 0xFF means what follows is a u64 "nameless" address to
// reconstruct into "_nameless.xx.xxxx.xxxx..." hex groups (see the
// worked-through trace in README.md —
// this is the least obvious part of the whole format); any other
// value is how many encoded-string parts follow, joined with '.'
// (0 parts -> the literal string "null", matching how e.g.
// "assigned_truck: null" is represented when no truck is assigned).
func decodeID(c *cursor) (id, error) {
	partCount, err := c.u8()
	if err != nil {
		return id{}, err
	}

	if partCount == 0xFF {
		address, err := c.u64()
		if err != nil {
			return id{}, err
		}
		return id{Value: decodeNamelessAddress(address)}, nil
	}

	var b strings.Builder
	for i := 0; i < int(partCount); i++ {
		v, err := c.u64()
		if err != nil {
			return id{}, err
		}
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(decodeEncodedString(v))
	}
	if partCount == 0 {
		return id{Value: "null"}, nil
	}
	return id{Value: b.String()}, nil
}

// decodeNamelessAddress reconstructs a "_nameless.<hex>.<hex>.<hex>"
// identifier from a u64 address, matching decode_id's 0xFF branch
// exactly. The address's 8 little-endian bytes are grouped into four
// 16-bit big-endian hex chunks (least-significant chunk first in
// memory order, most-significant last); the two most-significant
// chunks have leading zeros stripped, and any chunk that strips to
// nothing (or the two low chunks, which are never stripped) is joined
// with '.' in most-significant-first order for the final string —
// which is why real IDs are usually 2-3 dotted groups, not 4: the
// top 16 bits of a process-relative address are typically all zero
// and vanish entirely rather than appearing as an empty group.
func decodeNamelessAddress(address uint64) string {
	data := [8]byte{
		byte(address), byte(address >> 8), byte(address >> 16), byte(address >> 24),
		byte(address >> 32), byte(address >> 40), byte(address >> 48), byte(address >> 56),
	}

	value := ""
	currentPart := ""
	for i, b := range data {
		if i%2 == 0 && i > 0 {
			if i >= len(data)-2 {
				currentPart = strings.TrimLeft(currentPart, "0")
			}
			if currentPart != "" {
				value = currentPart + "." + value
			}
			currentPart = ""
		}
		currentPart = fmt.Sprintf("%02x", b) + currentPart
		if i == len(data)-1 {
			currentPart = strings.TrimLeft(currentPart, "0")
			if currentPart != "" {
				value = currentPart + "." + value
			}
			currentPart = ""
		}
	}
	return "_nameless." + strings.TrimSuffix(value, ".")
}

func (c *cursor) vec2() (vec2, error) {
	a, err := c.f32()
	if err != nil {
		return vec2{}, err
	}
	b, err := c.f32()
	if err != nil {
		return vec2{}, err
	}
	return vec2{A: a, B: b}, nil
}

func (c *cursor) vec3() (vec3, error) {
	a, err := c.f32()
	if err != nil {
		return vec3{}, err
	}
	b, err := c.f32()
	if err != nil {
		return vec3{}, err
	}
	cc, err := c.f32()
	if err != nil {
		return vec3{}, err
	}
	return vec3{A: a, B: b, C: cc}, nil
}

func (c *cursor) vec4() (vec4, error) {
	a, err := c.f32()
	if err != nil {
		return vec4{}, err
	}
	b, err := c.f32()
	if err != nil {
		return vec4{}, err
	}
	cc, err := c.f32()
	if err != nil {
		return vec4{}, err
	}
	d, err := c.f32()
	if err != nil {
		return vec4{}, err
	}
	return vec4{A: a, B: b, C: cc, D: d}, nil
}

func (c *cursor) vec7() (vec7, error) {
	vals := make([]float32, 7)
	for i := range vals {
		v, err := c.f32()
		if err != nil {
			return vec7{}, err
		}
		vals[i] = v
	}
	return vec7{A: vals[0], B: vals[1], C: vals[2], D: vals[3], E: vals[4], F: vals[5], G: vals[6]}, nil
}

// vec8 is ETS2's "placement" encoding, version 2/3: 8 floats where D
// packs a quantization bias applied to A and C (see the bit
// arithmetic below), and D itself is never part of the serialized
// text — see format.go's placement formatting.
func (c *cursor) vec8() (vec8, error) {
	vals := make([]float32, 8)
	for i := range vals {
		v, err := c.f32()
		if err != nil {
			return vec8{}, err
		}
		vals[i] = v
	}
	result := vec8{A: vals[0], B: vals[1], C: vals[2], D: vals[3], E: vals[4], F: vals[5], G: vals[6], H: vals[7]}

	bias := int64(result.D)

	bits := bias & 0xFFF
	bits -= 2048
	bits <<= 9
	result.A += float32(bits)

	bits2 := bias >> 12
	bits2 &= 0xFFF
	bits2 -= 2048
	bits2 <<= 9
	result.C += float32(bits2)

	return result, nil
}

func (c *cursor) int32Vec2() (int32Vec2, error) {
	a, err := c.i32()
	if err != nil {
		return int32Vec2{}, err
	}
	b, err := c.i32()
	if err != nil {
		return int32Vec2{}, err
	}
	return int32Vec2{A: a, B: b}, nil
}

func (c *cursor) int32Vec3() (int32Vec3, error) {
	a, err := c.i32()
	if err != nil {
		return int32Vec3{}, err
	}
	b, err := c.i32()
	if err != nil {
		return int32Vec3{}, err
	}
	cc, err := c.i32()
	if err != nil {
		return int32Vec3{}, err
	}
	return int32Vec3{A: a, B: b, C: cc}, nil
}
