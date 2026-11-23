package chunkcodec

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestEncodeDecode_roundtrip(t *testing.T) {
	for _, alg := range []string{AlgGzip, AlgZstd} {
		t.Run(alg, func(t *testing.T) {
			c, err := New(testKey(t), alg)
			if err != nil {
				t.Fatal(err)
			}
			plain := bytes.Repeat([]byte("hello-backup-world"), 1024)
			blob, err := c.Encode(plain)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(blob, plain) {
				t.Fatal("expected stored blob to differ from plaintext")
			}
			out, err := c.Decode(blob, alg, true)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out, plain) {
				t.Fatalf("roundtrip mismatch (%d vs %d bytes)", len(out), len(plain))
			}
		})
	}
}

func TestDecode_legacyPlaintext(t *testing.T) {
	c, err := New(testKey(t), AlgGzip)
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("legacy chunk bytes")
	out, err := c.Decode(plain, AlgNone, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatalf("expected legacy pass-through, got %q", out)
	}
}

func TestParseAES256Key(t *testing.T) {
	raw := "0123456789abcdef0123456789abcdef"
	key, err := ParseAES256Key(raw)
	if err != nil || len(key) != 32 {
		t.Fatalf("raw key: %v len=%d", err, len(key))
	}
	hexKey := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	key2, err := ParseAES256Key(hexKey)
	if err != nil || len(key2) != 32 {
		t.Fatalf("hex key: %v", err)
	}
}