package bsii

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// bsii's decoder is already validated against a real ~270,000-line ETS2
// save byte-for-byte against DecryptTruck's own independent decode of
// the exact same input (see README.md's Provenance section) — that
// validation lives in the project this was ported from, not here,
// since shipping any real save-derived data in this public repo would
// mean shipping whatever personal information happens to be in it.
//
// What this test file proves instead: a minimal BSII stream, built by
// hand from the format's own rules (see decode.go's doc comments), a
// signature and version, one structure ("schema") block declaring a
// unit type with two fields, and one data block instantiating it —
// decodes to exactly the SiiNunit text the format's own rules predict.
// This exercises the real structure-block/data-block state machine and
// two representative field types (UInt32, UTF8String) end to end,
// entirely with synthetic data.

// bsiiBuilder assembles a minimal, valid BSII byte stream by hand,
// mirroring exactly what cursor.go's readers expect on the way back
// out — see each write's matching read in decode.go/cursor.go.
type bsiiBuilder struct {
	buf bytes.Buffer
}

func (b *bsiiBuilder) u8(v byte) *bsiiBuilder {
	b.buf.WriteByte(v)
	return b
}

func (b *bsiiBuilder) u32(v uint32) *bsiiBuilder {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], v)
	b.buf.Write(tmp[:])
	return b
}

func (b *bsiiBuilder) bool_(v bool) *bsiiBuilder {
	if v {
		return b.u8(1)
	}
	return b.u8(0)
}

// utf8String writes cursor.go's utf8String encoding: a u32 length
// prefix followed by the raw bytes.
func (b *bsiiBuilder) utf8String(s string) *bsiiBuilder {
	b.u32(uint32(len(s)))
	b.buf.WriteString(s)
	return b
}

func TestDecode_SyntheticStructureAndDataBlock(t *testing.T) {
	b := &bsiiBuilder{}
	b.buf.WriteString("BSII") // signature — Decode reads and discards it as a plain u32
	b.u32(bsiiVersion1)       // version

	// Structure (schema) block: block_type == 0, one unit type
	// ("test_unit", structure id 1) with two fields — "value"
	// (typeUInt32) and "label" (typeUTF8String) — terminated by a
	// field-type 0.
	b.u32(0)                      // block_type == 0 -> structure block follows
	b.bool_(true)                 // valid
	b.u32(1)                      // structureID
	b.utf8String("test_unit")     // unit type name
	b.u32(uint32(typeUInt32))     // field 1 type
	b.utf8String("value")         // field 1 name
	b.u32(uint32(typeUTF8String)) // field 2 type
	b.utf8String("label")         // field 2 name
	b.u32(0)                      // field-list terminator

	// Data block: references structure id 1, ID with partCount == 0
	// (renders as the literal "null"), value=42, label="hello world"
	// (contains a space, so format.go's quoting rule must quote it).
	b.u32(1)                    // block_type == structureID 1 -> ends phase 1, starts this data block
	b.u8(0)                     // decodeID: partCount == 0 -> "null"
	b.u32(42)                   // field "value" (UInt32)
	b.utf8String("hello world") // field "label" (UTF8String)

	got, err := Decode(b.buf.Bytes())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	want := "SiiNunit\n{\ntest_unit : null {\n value: 42\n label: \"hello world\"\n}\n\n}"
	if string(got) != want {
		t.Errorf("Decode output =\n%s\nwant:\n%s", got, want)
	}
}

func TestDecode_UnsupportedVersion(t *testing.T) {
	b := &bsiiBuilder{}
	b.buf.WriteString("BSII")
	b.u32(99) // not a supported version

	if _, err := Decode(b.buf.Bytes()); err == nil {
		t.Error("expected an error for an unsupported BSII version")
	}
}

func TestDecode_TruncatedInput(t *testing.T) {
	b := &bsiiBuilder{}
	b.buf.WriteString("BSII")
	b.u32(bsiiVersion1)
	b.u32(0) // claims a structure block follows, but nothing does

	if _, err := Decode(b.buf.Bytes()); err == nil {
		t.Error("expected an error for a truncated structure block")
	}
}
