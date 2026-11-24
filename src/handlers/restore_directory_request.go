package handlers

import (
	"fmt"
	"strings"

	"cosmosbdr/src/config"
	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/dirsel"
	"cosmosbdr/src/types"
)

// IsDirectoryRestore reports a multi-file restore under a directory root (not live disk listing).
func IsDirectoryRestore(req *types.BackupWorkflowRequest) bool {
	if req == nil || req.FlowType != string(schema.FlowRestore) {
		return false
	}
	if req.Payload.Directory != nil {
		return true
	}
	return strings.TrimSpace(req.Payload.SourceDirectoryJobID) != ""
}

func ResolveDirectoryRestoreRequest(reg *config.SourceRegistry, req *types.BackupWorkflowRequest) error {
	payload := &req.Payload
	if strings.TrimSpace(payload.SourceID) == "" {
		return fmt.Errorf("source_id is required for directory restore")
	}
	dirRel := strings.TrimSpace(payload.RelativePath)
	if dirRel == "" {
		dirRel = "."
	}
	dirRel = config.NormalizeRelativePath(dirRel)
	payload.RelativePath = dirRel
	payload.FileName = ""

	if payload.Directory != nil {
		if err := dirsel.ValidateScope(payload.Directory); err != nil {
			return err
		}
	}

	layout, err := reg.ResolveDirectoryLayout(payload.SourceID, dirRel)
	if err != nil {
		return err
	}
	req.SourcePath = layout.SourcePathPrefix
	payload.RemoteIO = layout.RemoteIO
	return nil
}