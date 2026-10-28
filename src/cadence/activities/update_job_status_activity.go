package activities

import (
	"context"
	"fmt"
	"cosmosbdr/src/types"
)

func (ja *JobActivities) UpdateJobStatusActivity(ctx context.Context, input types.UpdateJobStatusRequest) error {
	if input.JobID == "" || input.Status == "" {
		return fmt.Errorf("jobID and status cannot be empty")
	}

	// 1. Fetch current status from DB
	currentStatus, err := ja.JobRepo.GetJobStatus(ctx, input.JobID)
	if err != nil {
		return fmt.Errorf("failed to fetch current status: %w", err)
	}

	// 2. Validate state transition in activity
	if !isValidTransition(currentStatus, input.Status) {
		return fmt.Errorf("invalid state transition from %s to %s", currentStatus, input.Status)
	}

	// 3. Perform update via repository
	err = ja.JobRepo.UpdateJobStatus(ctx, input.JobID, input.Status)
	if err != nil {
		return fmt.Errorf("failed to update status in repo: %w", err)
	}

	return nil
}

func isValidTransition(current, target string) bool {
	switch current {
	case "CREATED":
		return target == "IN_PROGRESS" || target == "FAILED"
	case "IN_PROGRESS":
		return target == "COMPLETED" || target == "FAILED"
	case "FAILED":
		return target == "IN_PROGRESS" // Allow reprocessing/retries
	case "COMPLETED":
		return false // Terminal state
	default:
		return false
	}
}
