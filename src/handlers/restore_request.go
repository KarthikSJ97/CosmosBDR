package handlers

import (
	"cosmosbdr/src/config"
	"cosmosbdr/src/types"
)

func ResolveRestoreWorkflowRequest(reg *config.SourceRegistry, req *types.BackupWorkflowRequest) error {
	if IsDirectoryRestore(req) {
		return ResolveDirectoryRestoreRequest(reg, req)
	}
	return ResolveBackupWorkflowRequest(reg, req)
}