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

	dirRel := input.DirRelative
	if dirRel == "" {
		dirRel = "."
	}
	sourcePrefix := input.SourcePrefix
	listRoot := input.ListRootPath
	if sourcePrefix == "" || listRoot == "" {
		resolved, err := ja.Registry.ResolveDirectoryLayout(input.SourceID, dirRel)
		if err != nil {
			return types.ListDirectoryResponse{}, err
		}
		if sourcePrefix == "" {
			sourcePrefix = resolved.SourcePathPrefix
		}
		if listRoot == "" {
			listRoot = resolved.ListRootPath
		}
	}
	if listRoot == "" {
		return types.ListDirectoryResponse{}, fmt.Errorf("list_root_path is required (re-resolve failed)")
	}

	layout := config.DirectoryLayout{
		SourcePathPrefix: sourcePrefix,
		ListRootPath:     listRoot,
		DirRelative:      dirRel,
		RemoteIO:         input.RemoteIO,
	}

	files, err := sourceread.ListDirectoryFiles(ctx, ja.Registry, input.SourceID, layout, input.Scope)
	if err != nil {
		return types.ListDirectoryResponse{}, err
	}
	if files == nil {
		files = []types.ListedBackupFile{}
	}
	return types.ListDirectoryResponse{Files: files}, nil
}