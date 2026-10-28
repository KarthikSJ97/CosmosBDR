package activities

import (
	"context"
	"cosmosbdr/src/types"
	"fmt"
)

func (ja *JobActivities) FinalizeJobActivity(ctx context.Context, input types.FinalizeJobRequest) error {
	if input.JobID == "" {
		return fmt.Errorf("jobID cannot be empty")
	}
	return ja.JobRepo.FinalizeJobExecution(ctx, input)
}
