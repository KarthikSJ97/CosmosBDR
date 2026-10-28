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

	err := ja.JobItemRepo.CompleteJobItem(ctx, input.ItemID)
	if err != nil {
		return types.CompleteJobItemResponse{}, fmt.Errorf("failed to finalize job item processing: %w", err)
	}

	return types.CompleteJobItemResponse{
		ItemID: input.ItemID,
		Status: "COMPLETED",
	}, nil
}
