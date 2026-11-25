package workflows

import (
	"fmt"
	"time"

	"cosmosbdr/src/cadence/activities"
	"cosmosbdr/src/types"

	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"
)

// Pre-backup helpers shared by BackupWorkflow and DirectoryBackupWorkflow (not a registered Cadence workflow).

// runPreBackupIfNeeded runs the source-level remote hook once per backup job (not on directory child file workflows).
func runPreBackupIfNeeded(ctx workflow.Context, jobActivities *activities.JobActivities, request types.BackupWorkflowRequest) (types.RunPreBackupResponse, error) {
	var empty types.RunPreBackupResponse
	if !request.Payload.PreBackup || request.Payload.ParentJobID != "" {
		return empty, nil
	}
	preCtx := workflow.WithActivityOptions(ctx, workflowActivityOptions(2*time.Hour))
	preReq := types.RunPreBackupRequest{
		SourceID:     request.Payload.SourceID,
		RelativePath: request.Payload.RelativePath,
	}
	var resp types.RunPreBackupResponse
	if err := workflow.ExecuteActivity(preCtx, jobActivities.RunPreBackupActivity, preReq).Get(ctx, &resp); err != nil {
		return empty, fmt.Errorf("pre_backup failed: %w", err)
	}
	return resp, nil
}

func maybeCleanupPreBackup(ctx workflow.Context, jobActivities *activities.JobActivities, sourceID string, pre types.RunPreBackupResponse) {
	if !pre.CleanupAfter || pre.OutputRelativePath == "" {
		return
	}
	cleanupReq := types.CleanupPreBackupRequest{
		SourceID:           sourceID,
		OutputRelativePath: pre.OutputRelativePath,
		CleanupAfter:       true,
	}
	if err := workflow.ExecuteActivity(ctx, jobActivities.CleanupPreBackupActivity, cleanupReq).Get(ctx, nil); err != nil {
		workflow.GetLogger(ctx).Warn("pre_backup remote cleanup failed", zap.Error(err))
	}
}