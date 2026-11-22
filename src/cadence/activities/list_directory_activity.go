package activities

import (
	"context"
	"fmt"

	"cosmosbdr/src/config"
	"cosmosbdr/src/sourceread"
	"cosmosbdr/src/types"
)

func (ja *JobActivities) ListDirectoryActivity(ctx context.Context, input types.ListDirectoryRequest) (types.ListDirectoryResponse, error) {
	if ja.Registry == nil {
		return types.ListDirectoryResponse{}, fmt.Errorf("source registry is not available in worker")
	}
	if input.SourceID == "" {
		return types.ListDirectoryResponse{}, fmt.Errorf("source_id is required")
	}

	layout := config.DirectoryLayout{
		SourcePathPrefix: input.SourcePrefix,
		ListRootPath:     input.ListRootPath,
		DirRelative:      input.DirRelative,
		RemoteIO:         input.RemoteIO,
	}

	files, err := sourceread.ListDirectoryFiles(ctx, ja.Registry, input.SourceID, layout, input.Scope)
	if err != nil {
		return types.ListDirectoryResponse{}, err
	}
	return types.ListDirectoryResponse{Files: files}, nil
}