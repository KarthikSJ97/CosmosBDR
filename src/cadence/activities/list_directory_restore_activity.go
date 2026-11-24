package activities

import (
	"context"
	"fmt"
	"strings"

	"cosmosbdr/src/config"
	"cosmosbdr/src/dirsel"
	"cosmosbdr/src/types"
)

func (ja *JobActivities) ListDirectoryRestoreActivity(ctx context.Context, input types.ListDirectoryRestoreRequest) (types.ListDirectoryRestoreResponse, error) {
	if ja.JobItemRepo == nil {
		return types.ListDirectoryRestoreResponse{}, fmt.Errorf("job item repository is not available")
	}
	if strings.TrimSpace(input.SourceID) == "" {
		return types.ListDirectoryRestoreResponse{}, fmt.Errorf("source_id is required")
	}
	dirRel := strings.TrimSpace(input.DirRelative)
	if dirRel == "" {
		dirRel = "."
	}
	dirRel = config.NormalizeRelativePath(dirRel)

	var snapshots []types.DirectoryRestoreTarget

	if strings.TrimSpace(input.SourceDirectoryJobID) != "" {
		items, err := ja.JobItemRepo.ListBackupSnapshotsByParentJobID(ctx, input.SourceDirectoryJobID)
		if err != nil {
			return types.ListDirectoryRestoreResponse{}, err
		}
		for _, it := range items {
			_, relPath, perr := config.ParseBackupKey(it.BackupKey)
			if perr != nil {
				continue
			}
			snapshots = append(snapshots, types.DirectoryRestoreTarget{
				RelativePath:    relPath,
				BackupKey:       it.BackupKey,
				SourceJobItemID: it.ID,
				SizeBytes:       it.OriginalSize,
			})
		}
	} else {
		prefix := directoryBackupKeyPrefix(input.SourceID, dirRel)
		items, err := ja.JobItemRepo.ListLatestEligibleBackupsUnderBackupKeyPrefix(ctx, prefix)
		if err != nil {
			return types.ListDirectoryRestoreResponse{}, err
		}
		for _, it := range items {
			_, relPath, perr := config.ParseBackupKey(it.BackupKey)
			if perr != nil {
				continue
			}
			if !pathUnderDirectoryRoot(dirRel, relPath) {
				continue
			}
			snapshots = append(snapshots, types.DirectoryRestoreTarget{
				RelativePath:    relPath,
				BackupKey:       it.BackupKey,
				SourceJobItemID: it.ID,
				SizeBytes:       it.OriginalSize,
			})
		}
	}

	if input.Scope != nil {
		if err := dirsel.ValidateScope(input.Scope); err != nil {
			return types.ListDirectoryRestoreResponse{}, err
		}
		m, err := dirsel.NewMatcher(*input.Scope)
		if err != nil {
			return types.ListDirectoryRestoreResponse{}, err
		}
		filtered := snapshots[:0]
		for _, s := range snapshots {
			relDir, ok := relativeToDirectoryRoot(dirRel, s.RelativePath)
			if !ok {
				continue
			}
			if !m.Accept(relDir, s.SizeBytes) {
				continue
			}
			filtered = append(filtered, s)
		}
		snapshots = filtered
	}

	if snapshots == nil {
		snapshots = []types.DirectoryRestoreTarget{}
	}
	return types.ListDirectoryRestoreResponse{Files: snapshots}, nil
}

func directoryBackupKeyPrefix(sourceID, dirRel string) string {
	dirRel = config.NormalizeRelativePath(dirRel)
	if dirRel == "." || dirRel == "" {
		return sourceID + ":"
	}
	return config.FormatBackupKey(sourceID, dirRel) + "/"
}

func pathUnderDirectoryRoot(dirRel, fileRel string) bool {
	dirRel = config.NormalizeRelativePath(dirRel)
	fileRel = config.NormalizeRelativePath(fileRel)
	if dirRel == "." || dirRel == "" {
		return fileRel != "" && fileRel != "."
	}
	if fileRel == dirRel {
		return true
	}
	return strings.HasPrefix(fileRel, dirRel+"/")
}

func relativeToDirectoryRoot(dirRel, fileRel string) (string, bool) {
	dirRel = config.NormalizeRelativePath(dirRel)
	fileRel = config.NormalizeRelativePath(fileRel)
	if !pathUnderDirectoryRoot(dirRel, fileRel) {
		return "", false
	}
	if dirRel == "." || dirRel == "" {
		return fileRel, true
	}
	if fileRel == dirRel {
		return "", false
	}
	return strings.TrimPrefix(fileRel, dirRel+"/"), true
}