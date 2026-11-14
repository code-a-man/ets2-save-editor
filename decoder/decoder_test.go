package decoder

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"testing"
)

// encryptScsC is the test-only inverse of decryptScsC: zlib-compress,
// PKCS7-pad, then AES-256-CBC encrypt, and wrap in the same "ScsC"
// header layout decryptScsC expects. This never ships in the tool
// itself — the tool only ever needs to decrypt a real save the game
// already wrote — but building a synthetic encrypted file this way
// (rather than depending on a real save) lets the round trip prove the
// crypto layer without any real save data anywhere in this repo.
func encryptScsC(t *testing.T, plain []byte) []byte {
	t.Helper()

	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(plain); err != nil {
		t.Fatalf("zlib compress: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing zlib writer: %v", err)
	}

	padded := pkcs7Pad(compressed.Bytes())

	iv := make([]byte, ivLen)
	if _, err := rand.Read(iv); err != nil {
		t.Fatalf("generating IV: %v", err)
	}

	block, err := aes.NewCipher(siiKey)
	if err != nil {
		t.Fatalf("building AES cipher: %v", err)
	}
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)

	var out bytes.Buffer
	out.WriteString(sigEncrypted)
	out.Write(make([]byte, hmacLen)) // HMAC tag: never verified on decrypt, zero is fine
	out.Write(iv)
	sizeBuf := make([]byte, sizeLen)
	binary.LittleEndian.PutUint32(sizeBuf, uint32(compressed.Len()))
	out.Write(sizeBuf)
	out.Write(ciphertext)
	return out.Bytes()
}

func pkcs7Pad(data []byte) []byte {
	padLen := aes.BlockSize - len(data)%aes.BlockSize
	padding := bytes.Repeat([]byte{byte(padLen)}, padLen)
	return append(data, padding...)
}

func TestDetectSignature(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want Signature
	}{
		{"plain", []byte("SiiNunit\n{\n}"), SigPlain},
		{"binary", append([]byte(sigBinary), 0, 0, 0, 0), SigBinary},
		{"encrypted", append([]byte(sigEncrypted), make([]byte, 60)...), SigEncrypted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := DetectSignature(c.in)
			if err != nil {
				t.Fatalf("DetectSignature: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestDetectSignature_UnrecognizedSignature(t *testing.T) {
	_, err := DetectSignature([]byte("XXXXsomething"))
	var target *ErrUnrecognizedSignature
	if !errors.As(err, &target) {
		t.Errorf("error = %v, want *ErrUnrecognizedSignature", err)
	}
}

func TestDecode_PlainTextPassesThrough(t *testing.T) {
	in := []byte("SiiNunit\n{\ntest_unit : test {\n}\n\n}")
	data, kind, err := Decode(in)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if kind != KindPlain {
		t.Errorf("kind = %q, want %q", kind, KindPlain)
	}
	if !bytes.Equal(data, in) {
		t.Errorf("plain-text input was mutated: got %q, want %q", data, in)
	}
}

func TestDecode_BinaryPassesThroughUndecoded(t *testing.T) {
	// Decode only unwraps encryption — turning BSII binary into text is
	// the sibling bsii package's job (see cmd/ets2-decrypt for how the
	// two compose).
	in := append([]byte(sigBinary), 1, 2, 3, 4)
	data, kind, err := Decode(in)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if kind != KindBinary {
		t.Errorf("kind = %q, want %q", kind, KindBinary)
	}
	if !bytes.Equal(data, in) {
		t.Errorf("binary input was mutated: got %v, want %v", data, in)
	}
}

// TestDecode_EncryptedPlainRoundTrip is this package's core validation:
// build a synthetic ScsC-encrypted container the same way the game
// does (zlib-compress, PKCS7-pad, AES-256-CBC encrypt), then confirm
// Decode recovers the exact original plaintext. No real save data is
// used anywhere — the crypto layer is fully exercised by a
// self-contained round trip.
func TestDecode_EncryptedPlainRoundTrip(t *testing.T) {
	original := []byte("SiiNunit\n{\ncompany : company.volatile.test {\n money: 250000\n}\n\n}")
	encrypted := encryptScsC(t, original)

	data, kind, err := Decode(encrypted)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if kind != KindPlain {
		t.Errorf("kind = %q, want %q", kind, KindPlain)
	}
	if !bytes.Equal(data, original) {
		t.Errorf("round trip = %q, want %q", data, original)
	}
}

// TestDecode_EncryptedBinaryRoundTrip confirms Decode correctly detects
// a BSII payload underneath the ScsC encryption layer too — the same
// two-stage detection real saves under most g_save_format settings go
// through.
func TestDecode_EncryptedBinaryRoundTrip(t *testing.T) {
	original := append([]byte(sigBinary), 1, 2, 3, 4, 5, 6, 7, 8)
	encrypted := encryptScsC(t, original)

	data, kind, err := Decode(encrypted)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if kind != KindBinary {
		t.Errorf("kind = %q, want %q", kind, KindBinary)
	}
	if !bytes.Equal(data, original) {
		t.Errorf("round trip = %v, want %v", data, original)
	}
}

func TestDecryptScsC_RejectsNonEncryptedInput(t *testing.T) {
	if _, err := DecryptScsC([]byte("SiiNunit\n{\n}")); err == nil {
		t.Error("expected an error decrypting plain-text input")
	}
}

func TestDecode_TooShort(t *testing.T) {
	if _, _, err := Decode([]byte("Si")); err == nil {
		t.Error("expected an error for input shorter than the signature")
	}
}
