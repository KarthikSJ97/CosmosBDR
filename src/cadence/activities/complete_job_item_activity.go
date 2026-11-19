package activities

import (
	"context"
	"fmt"
	"cosmosbdr/src/filehash"
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

	contentChecksum, err := filehash.ContentSHA256(item.ItemPath)
	if err != nil {
		return types.CompleteJobItemResponse{}, fmt.Errorf("failed to hash file at %s: %w", item.ItemPath, err)
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