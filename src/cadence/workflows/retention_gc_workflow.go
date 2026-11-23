package workflows

import (
	"cosmosbdr/src/cadence/activities"
	"cosmosbdr/src/types"
	"fmt"
	"time"

	"go.uber.org/cadence/workflow"
)

// RetentionGCWorkflow runs TTL expiry and unreferenced chunk cleanup (Phase F).
func RetentionGCWorkflow(ctx workflow.Context, request types.RetentionGCRequest) (*types.RetentionGCResponse, error) {
	ctx = workflow.WithActivityOptions(ctx, workflowActivityOptions(2*time.Hour))

	var jobActivities *activities.JobActivities
	var resp types.RetentionGCResponse
	if err := workflow.ExecuteActivity(ctx, jobActivities.RetentionGCActivity, request).Get(ctx, &resp); err != nil {
		return nil, fmt.Errorf("RetentionGCActivity: %w", err)
	}
	return &resp, nil
}