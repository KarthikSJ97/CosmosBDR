package activities

import (
	"context"
	"fmt"

	"cosmosbdr/src/types"
)

func (ja *JobActivities) FailJobExecutionActivity(ctx context.Context, input types.FailJobExecutionRequest) error {
	if input.JobID == "" {
		return fmt.Errorf("job_id is required")
	}
	return ja.JobItemRepo.FailIncompleteJobItems(ctx, input.JobID, input.JobItemID, input.Reason)
}