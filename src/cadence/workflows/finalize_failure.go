package workflows

import (
	"cosmosbdr/src/cadence/activities"
	"cosmosbdr/src/types"

	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"
)

// recordWorkflowFailure marks incomplete job items and the job row FAILED (orphan chunk blobs rely on Phase F GC).
func recordWorkflowFailure(ctx workflow.Context, jobActivities *activities.JobActivities, jobID, jobItemID string, cause error) {
	if jobID == "" || cause == nil {
		return
	}
	msg := cause.Error()

	failReq := types.FailJobExecutionRequest{
		JobID:     jobID,
		JobItemID: jobItemID,
		Reason:    msg,
	}
	if err := workflow.ExecuteActivity(ctx, jobActivities.FailJobExecutionActivity, failReq).Get(ctx, nil); err != nil {
		workflow.GetLogger(ctx).Warn("failed to record job failure in database",
			zap.String("job_id", jobID),
			zap.Error(err),
		)
		return
	}

	finalize := types.FinalizeJobRequest{
		JobID:        jobID,
		Status:       "FAILED",
		ErrorMessage: &msg,
	}
	if err := workflow.ExecuteActivity(ctx, jobActivities.FinalizeJobActivity, finalize).Get(ctx, nil); err != nil {
		workflow.GetLogger(ctx).Warn("failed to finalize job as FAILED",
			zap.String("job_id", jobID),
			zap.Error(err),
		)
	}
}