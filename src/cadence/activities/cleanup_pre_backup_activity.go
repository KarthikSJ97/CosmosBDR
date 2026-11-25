package activities

import (
	"context"
	"fmt"
	"strings"

	"cosmosbdr/src/sourceread"
	"cosmosbdr/src/types"
)

func (ja *JobActivities) CleanupPreBackupActivity(ctx context.Context, input types.CleanupPreBackupRequest) (types.CleanupPreBackupResponse, error) {
	if !input.CleanupAfter {
		return types.CleanupPreBackupResponse{}, nil
	}
	if ja.Registry == nil {
		return types.CleanupPreBackupResponse{}, fmt.Errorf("source registry is not configured")
	}
	sourceID := strings.TrimSpace(input.SourceID)
	if sourceID == "" || strings.TrimSpace(input.OutputRelativePath) == "" {
		return types.CleanupPreBackupResponse{}, fmt.Errorf("source_id and output_relative_path are required")
	}
	src, ok := ja.Registry.GetSource(sourceID)
	if !ok {
		return types.CleanupPreBackupResponse{}, fmt.Errorf("unknown source_id %q", sourceID)
	}

	remotePath := src.RemoteOutputPath(input.OutputRelativePath)
	if err := sourceread.RemoveSFTPFile(ctx, src, remotePath); err != nil {
		return types.CleanupPreBackupResponse{}, fmt.Errorf("cleanup pre_backup remote file: %w", err)
	}
	return types.CleanupPreBackupResponse{RemovedPath: remotePath}, nil
}