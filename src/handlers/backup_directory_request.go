package handlers

import (
	"fmt"
	"strings"

	"cosmosbdr/src/config"
	"cosmosbdr/src/dirsel"
	"cosmosbdr/src/types"
)

func IsDirectoryBackup(req *types.BackupWorkflowRequest) bool {
	return req.Payload.Directory != nil
}

func ResolveDirectoryBackupRequest(reg *config.SourceRegistry, req *types.BackupWorkflowRequest) error {
	payload := &req.Payload
	if strings.TrimSpace(payload.SourceID) == "" {
		return fmt.Errorf("source_id is required for directory backup")
	}
	if err := dirsel.ValidateScope(payload.Directory); err != nil {
		return err
	}

	dirRel := strings.TrimSpace(payload.RelativePath)
	if dirRel == "" {
		dirRel = "."
	}
	dirRel = config.NormalizeRelativePath(dirRel)
	payload.RelativePath = dirRel
	payload.FileName = ""

	layout, err := reg.ResolveDirectoryLayout(payload.SourceID, dirRel)
	if err != nil {
		return err
	}
	req.SourcePath = layout.SourcePathPrefix
	req.ListRootPath = layout.ListRootPath
	payload.RemoteIO = layout.RemoteIO
	return nil
}