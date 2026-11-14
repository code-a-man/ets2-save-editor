// Package decoder unwraps ETS2/ATS save files (game.sii, info.sii,
// profile.sii, ...) down to either plain SiiNunit text or BSII binary.
//
// SCS Software's save files come in three formats, identified by a 4-byte
// signature at the start of the file:
//
//	"SiiN"  plain text (SiiNunit) — nothing to do
//	"BSII"  binary-encoded, but NOT encrypted — see the sibling bsii package
//	"ScsC"  AES-256-CBC encrypted, zlib-compressed payload (the usual default)
//
// The AES-256 key below is fixed inside the game binary itself and has been
// public knowledge in the SCS modding community since it was first pulled
// out of the executable for the original (Pascal) SII_Decrypt tool. It is
// not a secret and every open-source SII decryptor (C#, TS, Rust, Zig...)
// ships the same bytes.
package decoder

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io"
)

// Kind identifies what format a save file's content ended up in after
// signature detection and (if needed) decryption.
type Kind string

const (
	KindPlain  Kind = "plain"  // SiiNunit plain text, ready to parse.
	KindBinary Kind = "binary" // BSII binary — see the sibling bsii package.
)

const (
	sigPlain     = "SiiN"
	sigBinary    = "BSII"
	sigEncrypted = "ScsC"

	// Encrypted header layout, all fields fixed-size and little-endian:
	//   4  bytes  signature "ScsC"
	//   32 bytes  HMAC-SHA256 tag (SCS uses this for integrity checking;
	//             not required to decrypt, so we just skip over it)
	//   16 bytes  AES IV
	//   4  bytes  uint32 decompressed size (sanity check only)
	//   N  bytes  AES-256-CBC ciphertext, PKCS7 padded
	sigLen    = 4
	hmacLen   = 32
	ivLen     = 16
	sizeLen   = 4
	headerLen = sigLen + hmacLen + ivLen + sizeLen
)

// siiKey is the fixed AES-256 key SCS Software uses to encrypt SII save data.
var siiKey = []byte{
	0x2a, 0x5f, 0xcb, 0x17, 0x91, 0xd2, 0x2f, 0xb6, 0x02, 0x45, 0xb3, 0xd8, 0x36, 0x9e, 0xd0, 0xb2,
	0xc2, 0x73, 0x71, 0x56, 0x3f, 0xbf, 0x1f, 0x3c, 0x9e, 0xdf, 0x6b, 0x11, 0x82, 0x5a, 0x5d, 0x0a,
}

// ErrUnrecognizedSignature is wrapped into the error returned by Decode
// when the input's leading 4 bytes don't match any known SII signature.
type ErrUnrecognizedSignature struct {
	Got []byte
}

func (e *ErrUnrecognizedSignature) Error() string {
	return fmt.Sprintf("unrecognized signature %q — not a known SII/save file", e.Got)
}

// Signature is a raw save file's outer 4-byte format marker, exposed for
// callers that need to detect and dispatch on format before deciding how
// to decode.
type Signature string

const (
	SigPlain     Signature = sigPlain
	SigBinary    Signature = sigBinary
	SigEncrypted Signature = sigEncrypted
)

// DetectSignature reads just the leading 4 bytes of raw and classifies
// them, without decoding or decrypting anything.
func DetectSignature(raw []byte) (Signature, error) {
	if len(raw) < sigLen {
		return "", fmt.Errorf("file too short to have a valid SII header")
	}
	switch string(raw[:sigLen]) {
	case sigPlain:
		return SigPlain, nil
	case sigBinary:
		return SigBinary, nil
	case sigEncrypted:
		return SigEncrypted, nil
	default:
		return "", &ErrUnrecognizedSignature{Got: append([]byte(nil), raw[:sigLen]...)}
	}
}

// DecryptScsC decrypts and decompresses an AES-256-CBC + zlib "ScsC"
// container and returns its inner payload as-is, without inspecting or
// re-detecting what format that payload is in — callers that need to know
// should run DetectSignature on the result.
func DecryptScsC(raw []byte) ([]byte, error) {
	if len(raw) < sigLen || string(raw[:sigLen]) != sigEncrypted {
		return nil, fmt.Errorf("input is not ScsC-encrypted")
	}
	return decryptScsC(raw)
}

// Decode reads raw save-file bytes and returns the content unwrapped down
// to either plain SiiNunit text or BSII binary, plus which of those two it
// ended up as. It never mutates or writes to the input.
func Decode(raw []byte) (data []byte, kind Kind, err error) {
	if len(raw) < sigLen {
		return nil, "", fmt.Errorf("file too short to have a valid SII header")
	}

	switch string(raw[:sigLen]) {
	case sigPlain:
		return raw, KindPlain, nil
	case sigBinary:
		return raw, KindBinary, nil
	case sigEncrypted:
		plain, err := decryptScsC(raw)
		if err != nil {
			return nil, "", err
		}
		if len(plain) >= sigLen && string(plain[:sigLen]) == sigBinary {
			return plain, KindBinary, nil
		}
		return plain, KindPlain, nil
	default:
		return nil, "", &ErrUnrecognizedSignature{Got: append([]byte(nil), raw[:sigLen]...)}
	}
}

// decryptScsC undoes the AES-256-CBC + zlib wrapping SCS puts around
// encrypted save data and returns the raw payload (plain SiiN text or BSII
// binary, depending on the game's save-format setting).
func decryptScsC(raw []byte) ([]byte, error) {
	if len(raw) < headerLen {
		return nil, fmt.Errorf("file too short for an encrypted SII header (need %d bytes, got %d)", headerLen, len(raw))
	}

	ivStart := sigLen + hmacLen
	sizeStart := ivStart + ivLen
	iv := raw[ivStart:sizeStart]
	// raw[sizeStart:headerLen] is SCS's own uint32 LE decompressed-size
	// sanity check; zlib already validates the stream itself, so we skip
	// past it without needing its value.
	ciphertext := raw[headerLen:]

	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length %d is not a multiple of the AES block size — file may be truncated or corrupt", len(ciphertext))
	}

	block, err := aes.NewCipher(siiKey)
	if err != nil {
		return nil, fmt.Errorf("building AES cipher: %w", err)
	}
	decrypted := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(decrypted, ciphertext)

	decrypted, err = pkcs7Unpad(decrypted)
	if err != nil {
		return nil, fmt.Errorf("removing PKCS7 padding (wrong key/IV or corrupt file): %w", err)
	}

	zr, err := zlib.NewReader(bytes.NewReader(decrypted))
	if err != nil {
		return nil, fmt.Errorf("opening zlib stream: %w", err)
	}
	defer zr.Close()

	plain, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("decompressing zlib stream: %w", err)
	}
	return plain, nil
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty plaintext")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > aes.BlockSize || padLen > len(data) {
		return nil, fmt.Errorf("invalid padding length %d", padLen)
	}
	for _, b := range data[len(data)-padLen:] {
		if int(b) != padLen {
			return nil, fmt.Errorf("invalid padding bytes")
		}
	}
	return data[:len(data)-padLen], nil
}
