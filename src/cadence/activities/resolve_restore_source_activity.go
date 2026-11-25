package activities

import (
	"context"
	"fmt"

	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/types"
)

func (ja *JobActivities) ResolveRestoreSourceActivity(ctx context.Context, input types.ResolveRestoreSourceRequest) (types.ResolveRestoreSourceResponse, error) {
	if input.JobID == "" {
		return types.ResolveRestoreSourceResponse{}, fmt.Errorf("job_id is required")
	}
	if input.ItemPath == "" {
		return types.ResolveRestoreSourceResponse{}, fmt.Errorf("item_path is required")
	}

	var sourceItem *schema.JobItem
	var err error

	if input.SourceJobItemID != "" {
		sourceItem, err = ja.JobItemRepo.GetEligibleBackupItemByID(ctx, input.SourceJobItemID)
		if err != nil {
			return types.ResolveRestoreSourceResponse{}, err
		}
		if sourceItem == nil {
			return types.ResolveRestoreSourceResponse{}, fmt.Errorf("no eligible backup found for job_item_id %s", input.SourceJobItemID)
		}
		if input.BackupKey != "" && sourceItem.BackupKey != input.BackupKey {
			return types.ResolveRestoreSourceResponse{}, fmt.Errorf(
				"source job item backup_key %q does not match requested backup_key %q",
				sourceItem.BackupKey, input.BackupKey,
			)
		}
		if input.BackupKey == "" && sourceItem.ItemPath != input.ItemPath {
			return types.ResolveRestoreSourceResponse{}, fmt.Errorf(
				"source job item path %q does not match requested item_path %q",
				sourceItem.ItemPath, input.ItemPath,
			)
		}
	} else if input.BackupKey != "" {
		sourceItem, err = ja.JobItemRepo.GetLatestEligibleBackupItemByBackupKey(ctx, input.BackupKey)
		if err != nil {
			return types.ResolveRestoreSourceResponse{}, err
		}
		if sourceItem == nil {
			return types.ResolveRestoreSourceResponse{}, fmt.Errorf("no completed backup found for backup_key %s", input.BackupKey)
		}
	} else {
		sourceItem, err = ja.JobItemRepo.GetLatestEligibleBackupItemByPath(ctx, input.ItemPath)
		if err != nil {
			return types.ResolveRestoreSourceResponse{}, err
		}
		if sourceItem == nil {
			return types.ResolveRestoreSourceResponse{}, fmt.Errorf("no completed backup found for path %s", input.ItemPath)
		}
	}

	manifestRows, err := ja.ChunkRepo.ListItemChunksWithStorage(ctx, sourceItem.ID)
	if err != nil {
		return types.ResolveRestoreSourceResponse{}, err
	}

	if sourceItem.OriginalSize > 0 && len(manifestRows) == 0 {
		return types.ResolveRestoreSourceResponse{}, fmt.Errorf(
			"backup job_item %s has size %d but no chunk mappings",
			sourceItem.ID, sourceItem.OriginalSize,
		)
	}

	restoreItem, err := ja.JobItemRepo.CreateRestoreJobItem(ctx, input.JobID, sourceItem)
	if err != nil {
		return types.ResolveRestoreSourceResponse{}, err
	}

	if len(manifestRows) > 0 {
		if err := ja.JobItemRepo.CloneChunksFromItem(ctx, sourceItem.ID, restoreItem.ID); err != nil {
			return types.ResolveRestoreSourceResponse{}, fmt.Errorf("failed to clone chunk manifest for restore: %w", err)
		}
	}

	chunks := make([]types.RestoreChunkPlan, len(manifestRows))
	for i, row := range manifestRows {
		chunks[i] = types.RestoreChunkPlan{
			ChunkIndex:     row.ChunkIndex,
			OffsetBytes:    row.OffsetBytes,
			LengthBytes:    row.LengthBytes,
			ChunkHash:      row.ChunkHash,
			SeaweedKey:     row.SeaweedKey,
			CompressionAlg: row.CompressionAlg,
			Encrypted:      row.Encrypted,
		}
	}

	destinationPath := sourceItem.ItemPath
	sourceID := ""
	remoteIO := false

	if input.TargetPath != "" {
		destinationPath = input.TargetPath
	} else if sourceItem.BackupKey != "" && ja.Registry != nil {
		dest, err := ja.Registry.RestoreDestinationForBackupKey(sourceItem.BackupKey)
		if err != nil {
			return types.ResolveRestoreSourceResponse{}, fmt.Errorf("resolve restore destination: %w", err)
		}
		destinationPath = dest.DestinationPath
		sourceID = dest.SourceID
		remoteIO = dest.RemoteIO
	}

	if destinationPath != "" && destinationPath != restoreItem.ItemPath {
		if err := ja.JobItemRepo.UpdateJobItemPath(ctx, restoreItem.ID, destinationPath); err != nil {
			return types.ResolveRestoreSourceResponse{}, fmt.Errorf("update restore job item path: %w", err)
		}
		restoreItem.ItemPath = destinationPath
	}

	return types.ResolveRestoreSourceResponse{
		DestinationPath: destinationPath,
		SourceID:        sourceID,
		RemoteIO:        remoteIO,
		SourceJobItemID: sourceItem.ID,
		RestoreJobItem:  restoreItem,
		OriginalSize:    sourceItem.OriginalSize,
		Chunks:          chunks,
	}, nil
}