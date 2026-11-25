package activities

import (
	"context"
	"fmt"
	"cosmosbdr/src/types"
)

func (ja *JobActivities) CompleteJobItemActivity(ctx context.Context, input types.CompleteJobItemRequest) (types.CompleteJobItemResponse, error) {
	if input.ItemID == "" {
		return types.CompleteJobItemResponse{}, fmt.Errorf("itemID cannot be empty")
	}

	item, err := ja.JobItemRepo.GetJobItemByID(ctx, input.ItemID)
	if err != nil {
		return types.CompleteJobItemResponse{}, err
	}
	if item == nil {
		return types.CompleteJobItemResponse{}, fmt.Errorf("job item not found: %s", input.ItemID)
	}

	hashPath := item.ItemPath
	hashBackupKey := item.BackupKey
	hashSourceID := ""
	if input.VerifyPath != "" {
		hashPath = input.VerifyPath
		hashBackupKey = "" // do not infer SFTP reader from backup_key when hashing restore destination
		if input.VerifyRemoteIO {
			hashSourceID = input.VerifySourceID
		}
	}
	contentChecksum, err := ja.hashJobItem(ctx, hashPath, hashBackupKey, hashSourceID)
	if err != nil {
		return types.CompleteJobItemResponse{}, fmt.Errorf("failed to hash file at %s: %w", hashPath, err)
	}

	err = ja.JobItemRepo.CompleteJobItem(ctx, input.ItemID, contentChecksum)
	if err != nil {
		return types.CompleteJobItemResponse{}, fmt.Errorf("failed to finalize job item processing: %w", err)
	}

	return types.CompleteJobItemResponse{
		ItemID: input.ItemID,
		Status: "COMPLETED",
	}, nil
}