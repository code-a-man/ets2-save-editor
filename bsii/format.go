package bsii

import (
	"math"
	"strconv"
)

// singleToString renders a float32 the same way plain-text SII does:
// a whole number prints as a plain decimal, anything with a fractional
// part (or too large to be a "clean" integer, matching DecryptTruck's
// >= 1e7 threshold) prints as plain-text SII's raw-hex-float
// literal ("&" + 8 hex digits of the IEEE-754 bit pattern).
func singleToString(f float32) string {
	if float32(int32(f)) != f || f >= 1e7 {
		return "&" + hex8(math.Float32bits(f))
	}
	return strconv.FormatInt(int64(int32(f)), 10)
}

const hexDigits = "0123456789abcdef"

func hex8(v uint32) string {
	b := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		b[i] = hexDigits[v&0xF]
		v >>= 4
	}
	return string(b)
}

func vec2ToString(v vec2) string {
	return "(" + singleToString(v.A) + ", " + singleToString(v.B) + ")"
}

func vec3ToString(v vec3) string {
	return "(" + singleToString(v.A) + ", " + singleToString(v.B) + ", " + singleToString(v.C) + ")"
}

// vec4ToString renders a bare quaternion (no accompanying position) as
// a single "(w; x, y, z)" group.
func vec4ToString(v vec4) string {
	return "(" + singleToString(v.A) + "; " + singleToString(v.B) + ", " + singleToString(v.C) + ", " + singleToString(v.D) + ")"
}

// vec7ToString / vec8ToString render ETS2's "placement" (position +
// quaternion) as two groups: "(x, y, z) (w; x, y, z)". vec8's D
// component is quantization bias consumed during decode (see
// cursor.vec8) and is deliberately not one of the 7 printed floats.
func vec7ToString(v vec7) string {
	return "(" + singleToString(v.A) + ", " + singleToString(v.B) + ", " + singleToString(v.C) + ") (" +
		singleToString(v.D) + "; " + singleToString(v.E) + ", " + singleToString(v.F) + ", " + singleToString(v.G) + ")"
}

func vec8ToString(v vec8) string {
	return "(" + singleToString(v.A) + ", " + singleToString(v.B) + ", " + singleToString(v.C) + ") (" +
		singleToString(v.E) + "; " + singleToString(v.F) + ", " + singleToString(v.G) + ", " + singleToString(v.H) + ")"
}

func int32Vec2ToString(v int32Vec2) string {
	return "(" + strconv.FormatInt(int64(v.A), 10) + ", " + strconv.FormatInt(int64(v.B), 10) + ")"
}

func int32Vec3ToString(v int32Vec3) string {
	return "(" + strconv.FormatInt(int64(v.A), 10) + ", " + strconv.FormatInt(int64(v.B), 10) + ", " + strconv.FormatInt(int64(v.C), 10) + ")"
}

// u32Text renders a uint32, mapping the all-ones sentinel to the
// literal token "nil" the same way DecryptTruck's serializer does
// (used for UInt32/UInt32Type2/UInt64/UInt16 "unset" values).
func u32Text(v uint32) string {
	if v == 0xFFFFFFFF {
		return "nil"
	}
	return strconv.FormatUint(uint64(v), 10)
}

func u64Text(v uint64) string {
	if v == 0xFFFFFFFFFFFFFFFF {
		return "nil"
	}
	return strconv.FormatUint(v, 10)
}

func u16Text(v uint16) string {
	if v == 0xFFFF {
		return "nil"
	}
	return strconv.FormatUint(uint64(v), 10)
}

func i16Text(v int16) string {
	if v == 0x7FFF {
		return "nil"
	}
	return strconv.FormatInt(int64(v), 10)
}

// limitedAlphabet mirrors DecryptTruck's LIMITED_ALPHABET: an unquoted
// UTF8String value is only ever printed bare if every character is
// alphanumeric or '_' — anything else gets wrapped in quotes.
func isLimitedAlphabet(s string) bool {
	for _, r := range s {
		ok := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
		if !ok {
			return false
		}
	}
	return true
}

// formatUTF8Scalar applies the same scalar-string formatting rules as
// DecryptTruck's serialize_utf8_string: a value that parses as a plain
// integer, or consists only of the limited alphabet, prints unquoted
// (a plain-text SII parser treats bare tokens and quoted strings
// identically, so this only affects readability); anything else,
// including the empty string, is quoted.
func formatUTF8Scalar(s string) string {
	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		return s
	}
	if s == "" {
		return `""`
	}
	if isLimitedAlphabet(s) {
		return s
	}
	return `"` + s + `"`
}

// formatUTF8ArrayElem is formatUTF8Scalar's array-element counterpart.
// The reference decoder checks unsigned-integer parseability here
// (serialize_utf8_string_vec) versus signed for the scalar case
// (serialize_utf8_string) — a real, if minor, asymmetry preserved
// deliberately rather than unified.
func formatUTF8ArrayElem(s string) string {
	if _, err := strconv.ParseUint(s, 10, 64); err == nil {
		return s
	}
	if s == "" {
		return `""`
	}
	if isLimitedAlphabet(s) {
		return s
	}
	return `"` + s + `"`
}

// ordinalStringOverrides are field-name-specific corrections
// DecryptTruck applies for a handful of known SCS ordinal/encoded
// string quirks it couldn't otherwise resolve generically — ported
// as-is rather than guessed at independently.
var ordinalStringOverrides = map[[2]string]string{
	{"part_type", "vehicle"}: "unknown",
	{"type", "parking"}:      "spot",
	{"setup", "low_beam"}:    "candela_hue_saturation",
	{"setup", "parking"}:     "lumen_hue_saturation",
	{"dir_type", "parking"}:  "wide",
	{"dir_type", "low_beam"}: "narrow",
}

// formatOrdinalOrEncodedScalar applies the override table above, plus
// the same empty-string-becomes-cut_direction-default and
// empty-string-becomes-quoted-empty-string rules DecryptTruck's
// serialize_single_value_string uses.
func formatOrdinalOrEncodedScalar(fieldName, value string) string {
	if override, ok := ordinalStringOverrides[[2]string{fieldName, value}]; ok {
		return override
	}
	if fieldName == "cut_direction" && value == "" {
		return "forward"
	}
	if value == "" {
		return `""`
	}
	return value
}
