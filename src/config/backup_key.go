package config

import (
	"fmt"
	"path"
	"strings"
)

const backupKeySep = ":"

// FormatBackupKey builds the stable logical identity source_id:relative_path.
func FormatBackupKey(sourceID, relativePath string) string {
	return sourceID + backupKeySep + NormalizeRelativePath(relativePath)
}

// ParseBackupKey splits source_id and relative_path from a backup_key.
func ParseBackupKey(backupKey string) (sourceID, relativePath string, err error) {
	backupKey = strings.TrimSpace(backupKey)
	if backupKey == "" {
		return "", "", fmt.Errorf("backup_key is empty")
	}
	i := strings.Index(backupKey, backupKeySep)
	if i <= 0 || i == len(backupKey)-1 {
		return "", "", fmt.Errorf("invalid backup_key %q: expected source_id:relative_path", backupKey)
	}
	return backupKey[:i], backupKey[i+1:], nil
}

// NormalizeRelativePath cleans client-supplied paths for stable keys and joins.
func NormalizeRelativePath(relativePath string) string {
	relativePath = strings.TrimSpace(relativePath)
	relativePath = strings.TrimPrefix(relativePath, "/")
	return path.Clean(relativePath)
}