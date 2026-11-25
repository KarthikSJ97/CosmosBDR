package handlers

import (
	"fmt"
	"strings"

	"cosmosbdr/src/config"
	"cosmosbdr/src/types"
)

// ResolveBackupWorkflowRequest fills source_path, file_name, and payload backup identity fields.
// Supports source_id + relative_path (preferred) or legacy source_path + file_name.
func ResolveBackupWorkflowRequest(reg *config.SourceRegistry, req *types.BackupWorkflowRequest) error {
	if IsDirectoryBackup(req) {
		return ResolveDirectoryBackupRequest(reg, req)
	}

	payload := &req.Payload

	if strings.TrimSpace(payload.SourceID) != "" {
		rel := payload.RelativePath
		if rel == "" {
			rel = payload.FileName
		}
		src, ok := reg.GetSource(payload.SourceID)
		if !ok {
			return fmt.Errorf("unknown source_id %q", payload.SourceID)
		}
		pathForJob := rel
		if src.HasPreBackup() {
			payload.PreBackup = true
			pathForJob = src.PreBackupOutputRelative(rel)
		}
		layout, err := reg.ResolveJobPaths(payload.SourceID, pathForJob)
		if err != nil {
			return err
		}
		req.SourcePath = layout.SourcePathPrefix
		payload.RelativePath = config.NormalizeRelativePath(pathForJob)
		payload.FileName = payload.RelativePath
		payload.BackupKey = layout.BackupKey
		payload.RemoteIO = layout.RemoteIO
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