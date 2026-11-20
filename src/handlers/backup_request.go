package handlers

import (
	"fmt"
	"path/filepath"
	"strings"

	"cosmosbdr/src/config"
	"cosmosbdr/src/types"
)

// resolveBackupWorkflowRequest fills source_path, file_name, and payload backup identity fields.
// Supports source_id + relative_path (preferred) or legacy source_path + file_name.
func resolveBackupWorkflowRequest(reg *config.SourceRegistry, req *types.BackupWorkflowRequest) error {
	payload := &req.Payload

	if strings.TrimSpace(payload.SourceID) != "" {
		rel := payload.RelativePath
		if rel == "" {
			rel = payload.FileName
		}
		mount, backupKey, err := reg.ResolveReadPath(payload.SourceID, rel)
		if err != nil {
			return err
		}
		req.SourcePath = mountDir(mount)
		payload.RelativePath = config.NormalizeRelativePath(rel)
		payload.FileName = payload.RelativePath
		payload.BackupKey = backupKey
		return nil
	}

	if req.SourcePath == "" || payload.FileName == "" {
		return fmt.Errorf("provide source_id and relative_path (or file_name), or legacy source_path and file_name")
	}

	if payload.BackupKey == "" && strings.TrimSpace(payload.SourceID) == "" {
		// Legacy path-only job; backup_key may remain empty until backfilled.
		payload.RelativePath = config.NormalizeRelativePath(payload.FileName)
	}
	return nil
}

func mountDir(absoluteFilePath string) string {
	return filepath.Dir(absoluteFilePath)
}