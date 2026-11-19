package filehash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// ContentSHA256 computes the standard SHA-256 digest of entire file bytes (same as shasum -a 256).
func ContentSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("read file for hashing: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}