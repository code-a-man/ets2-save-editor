package bsii

import (
	"fmt"
	"math"
)

// cursor is a bounds-checked reader over a byte slice, mirroring the
// `&mut usize` offset parameter DecryptTruck threads through every
// decode_* function.
type cursor struct {
	data []byte
	pos  int
}

func (c *cursor) remaining() int { return len(c.data) - c.pos }

func (c *cursor) take(n int) ([]byte, error) {
	if n < 0 || c.remaining() < n {
		return nil, fmt.Errorf("unexpected end of BSII data at offset %d (need %d bytes, have %d)", c.pos, n, c.remaining())
	}
	b := c.data[c.pos : c.pos+n]
	c.pos += n
	return b, nil
}

func (c *cursor) u8() (byte, error) {
	b, err := c.take(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (c *cursor) u16() (uint16, error) {
	b, err := c.take(2)
	if err != nil {
		return 0, err
	}
	return uint16(b[0]) | uint16(b[1])<<8, nil
}

func (c *cursor) u32() (uint32, error) {
	b, err := c.take(4)
	if err != nil {
		return 0, err
	}
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24, nil
}

func (c *cursor) u64() (uint64, error) {
	b, err := c.take(8)
	if err != nil {
		return 0, err
	}
	var v uint64
	for i := 7; i >= 0; i-- {
		v = v<<8 | uint64(b[i])
	}
	return v, nil
}

func (c *cursor) i16() (int16, error) {
	v, err := c.u16()
	return int16(v), err
}

func (c *cursor) i32() (int32, error) {
	v, err := c.u32()
	return int32(v), err
}

func (c *cursor) i64() (int64, error) {
	v, err := c.u64()
	return int64(v), err
}

func (c *cursor) f32() (float32, error) {
	v, err := c.u32()
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(v), nil
}

func (c *cursor) boolean() (bool, error) {
	b, err := c.u8()
	if err != nil {
		return false, err
	}
	return b != 0, nil
}

// utf8String reads a u32 length prefix followed by that many raw bytes
// (0x01 UTF8String — not null-terminated).
func (c *cursor) utf8String() (string, error) {
	n, err := c.u32()
	if err != nil {
		return "", err
	}
	b, err := c.take(int(n))
	if err != nil {
		return "", err
	}
	return string(b), nil
}
