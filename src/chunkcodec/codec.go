package chunkcodec

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/klauspost/compress/zstd"
)

const (
	AlgGzip = "gzip"
	AlgZstd = "zstd"
	AlgNone = "none"

	gcmNonceSize = 12
)

// Codec compresses then encrypts chunk payloads for SeaweedFS (restore reverses the pipeline).
type Codec struct {
	key                []byte
	compressionAlgorithm string
}

// New builds a codec from a 32-byte AES key and compression algorithm name (gzip or zstd).
func New(encryptionKey []byte, compressionAlgorithm string) (*Codec, error) {
	if len(encryptionKey) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes for AES-256")
	}
	alg := strings.ToLower(strings.TrimSpace(compressionAlgorithm))
	switch alg {
	case AlgGzip, AlgZstd:
	default:
		return nil, fmt.Errorf("unsupported compression algorithm %q (use gzip or zstd)", compressionAlgorithm)
	}
	keyCopy := make([]byte, 32)
	copy(keyCopy, encryptionKey)
	return &Codec{
		key:                  keyCopy,
		compressionAlgorithm: alg,
	}, nil
}

// ParseAES256Key accepts a 32-byte raw string or 64-character hex encoding.
func ParseAES256Key(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("encryption key is empty")
	}
	if len(s) == 64 {
		b, err := hex.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("decode ENCRYPTION_KEY hex: %w", err)
		}
		if len(b) == 32 {
			return b, nil
		}
	}
	if len(s) == 32 {
		return []byte(s), nil
	}
	return nil, fmt.Errorf("ENCRYPTION_KEY must be 32 raw bytes or 64 hex characters (got %d chars)", len(s))
}

// CompressionAlgorithm returns the configured algorithm for new chunks.
func (c *Codec) CompressionAlgorithm() string {
	if c == nil {
		return AlgNone
	}
	return c.compressionAlgorithm
}

// Encode runs compress → AES-256-GCM with a random per-object nonce prepended to the ciphertext.
func (c *Codec) Encode(plaintext []byte) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("chunk codec is not configured")
	}
	compressed, err := c.compress(plaintext)
	if err != nil {
		return nil, err
	}
	return c.encrypt(compressed)
}

// Decode decrypts and decompresses a stored blob. Legacy plaintext objects pass through when encrypted is false.
func (c *Codec) Decode(blob []byte, compressionAlg string, encrypted bool) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("chunk codec is not configured")
	}
	var payload []byte
	var err error
	if encrypted {
		payload, err = c.decrypt(blob)
		if err != nil {
			return nil, err
		}
	} else {
		payload = blob
	}
	alg := strings.ToLower(strings.TrimSpace(compressionAlg))
	if alg == "" || alg == AlgNone {
		return payload, nil
	}
	return c.decompress(payload, alg)
}

func (c *Codec) compress(plaintext []byte) ([]byte, error) {
	var buf bytes.Buffer
	switch c.compressionAlgorithm {
	case AlgGzip:
		w := gzip.NewWriter(&buf)
		if _, err := w.Write(plaintext); err != nil {
			return nil, fmt.Errorf("gzip compress: %w", err)
		}
		if err := w.Close(); err != nil {
			return nil, fmt.Errorf("gzip close: %w", err)
		}
	case AlgZstd:
		enc, err := zstd.NewWriter(&buf)
		if err != nil {
			return nil, fmt.Errorf("zstd writer: %w", err)
		}
		if _, err := enc.Write(plaintext); err != nil {
			return nil, fmt.Errorf("zstd compress: %w", err)
		}
		if err := enc.Close(); err != nil {
			return nil, fmt.Errorf("zstd close: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported compression %q", c.compressionAlgorithm)
	}
	return buf.Bytes(), nil
}

func (c *Codec) decompress(data []byte, alg string) ([]byte, error) {
	switch alg {
	case AlgGzip:
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer r.Close()
		out, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("gzip decompress: %w", err)
		}
		return out, nil
	case AlgZstd:
		dec, err := zstd.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("zstd reader: %w", err)
		}
		defer dec.Close()
		out, err := io.ReadAll(dec)
		if err != nil {
			return nil, fmt.Errorf("zstd decompress: %w", err)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported compression algorithm %q", alg)
	}
}

func (c *Codec) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, len(nonce)+len(ciphertext))
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

func (c *Codec) decrypt(blob []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	if len(blob) < gcmNonceSize+gcm.Overhead() {
		return nil, fmt.Errorf("encrypted blob too short (%d bytes)", len(blob))
	}
	nonce := blob[:gcmNonceSize]
	ciphertext := blob[gcmNonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}