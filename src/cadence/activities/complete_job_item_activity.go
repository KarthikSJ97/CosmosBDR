package activities

import (
	"context"
	"fmt"
)

type CompleteJobItemRequest struct {
	JobID  string `json:"job_id"`
	ItemID string `json:"item_id"`
}

type CompleteJobItemResponse struct {
	ItemID string `json:"item_id"`
	Status string `json:"status"`
}

func (ja *JobActivities) CompleteJobItemActivity(ctx context.Context, input CompleteJobItemRequest) (CompleteJobItemResponse, error) {
	if input.ItemID == "" {
		return CompleteJobItemResponse{}, fmt.Errorf("itemID cannot be empty")
	}

	err := ja.JobItemRepo.CompleteJobItem(ctx, input.ItemID)
	if err != nil {
		return CompleteJobItemResponse{}, fmt.Errorf("failed to finalize job item processing: %w", err)
	}

	return CompleteJobItemResponse{
		ItemID: input.ItemID,
		Status: "COMPLETED",
	}, nil
}
