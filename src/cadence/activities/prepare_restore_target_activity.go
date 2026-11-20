package activities

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"cosmosbdr/src/sourceread"
	"cosmosbdr/src/types"
)

func (ja *JobActivities) PrepareRestoreTargetActivity(ctx context.Context, input types.PrepareRestoreTargetRequest) (types.PrepareRestoreTargetResponse, error) {
	if input.DestinationPath == "" {
		return types.PrepareRestoreTargetResponse{}, fmt.Errorf("destination_path is required")
	}
	if input.OriginalSize < 0 {
		return types.PrepareRestoreTargetResponse{}, fmt.Errorf("original_size cannot be negative")
	}

	if input.RemoteIO {
		if ja.Registry == nil {
			return types.PrepareRestoreTargetResponse{}, fmt.Errorf("source registry is not configured")
		}
		if input.SourceID == "" {
			return types.PrepareRestoreTargetResponse{}, fmt.Errorf("source_id is required for remote restore")
		}
		src, ok := ja.Registry.GetSource(input.SourceID)
		if !ok {
			return types.PrepareRestoreTargetResponse{}, fmt.Errorf("unknown source_id %q", input.SourceID)
		}
		if err := sourceread.PrepareSFTPRestoreFile(ctx, src, input.DestinationPath, input.OriginalSize); err != nil {
			return types.PrepareRestoreTargetResponse{}, fmt.Errorf("prepare remote restore target: %w", err)
		}
		return types.PrepareRestoreTargetResponse{
			DestinationPath: input.DestinationPath,
			OriginalSize:    input.OriginalSize,
		}, nil
	}

	parentDir := filepath.Dir(input.DestinationPath)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return types.PrepareRestoreTargetResponse{}, fmt.Errorf("failed to create parent directories: %w", err)
	}

	file, err := os.OpenFile(input.DestinationPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return types.PrepareRestoreTargetResponse{}, fmt.Errorf("failed to create restore target file: %w", err)
	}
	defer file.Close()

	if err := file.Truncate(input.OriginalSize); err != nil {
		return types.PrepareRestoreTargetResponse{}, fmt.Errorf("failed to size restore target file: %w", err)
	}

	return types.PrepareRestoreTargetResponse{
		DestinationPath: input.DestinationPath,
		OriginalSize:    input.OriginalSize,
	}, nil
}