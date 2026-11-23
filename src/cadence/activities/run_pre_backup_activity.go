package activities

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cosmosbdr/src/sourceread"
	"cosmosbdr/src/types"
)

const preBackupSSHTimeout = 2 * time.Hour

func (ja *JobActivities) RunPreBackupActivity(ctx context.Context, input types.RunPreBackupRequest) (types.RunPreBackupResponse, error) {
	if ja.Registry == nil {
		return types.RunPreBackupResponse{}, fmt.Errorf("source registry is not configured")
	}
	sourceID := strings.TrimSpace(input.SourceID)
	if sourceID == "" {
		return types.RunPreBackupResponse{}, fmt.Errorf("source_id is required")
	}
	src, ok := ja.Registry.GetSource(sourceID)
	if !ok {
		return types.RunPreBackupResponse{}, fmt.Errorf("unknown source_id %q", sourceID)
	}
	if !src.HasPreBackup() {
		return types.RunPreBackupResponse{}, fmt.Errorf("source %q has no pre_backup configured", sourceID)
	}

	dumpRel := src.DumpRelativePath(input.RelativePath)
	if dumpRel == "" {
		return types.RunPreBackupResponse{}, fmt.Errorf("relative_path is required")
	}

	argv, env, err := src.BuildPreBackupExec(dumpRel)
	if err != nil {
		return types.RunPreBackupResponse{}, err
	}

	execCtx, cancel := context.WithTimeout(ctx, preBackupSSHTimeout)
	defer cancel()

	if err := sourceread.RunSSHCommand(execCtx, src, argv, env); err != nil {
		return types.RunPreBackupResponse{}, fmt.Errorf("pre_backup exec: %w", err)
	}

	return types.RunPreBackupResponse{
		DumpRelativePath: dumpRel,
		RemotePath:       src.RemoteDumpPath(dumpRel),
		CleanupAfter:     src.ResolvedPreBackup.CleanupAfter,
	}, nil
}