package workflows

import (
	"cosmosbdr/src/cadence/activities"
	"cosmosbdr/src/config"
	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/types"
	"fmt"
	"strings"
	"time"

	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"
)

func DirectoryRestoreWorkflow(ctx workflow.Context, request types.BackupWorkflowRequest) (*types.DirectoryRestoreWorkflowResponse, error) {
	ctx = workflow.WithActivityOptions(ctx, workflowActivityOptions(15*time.Minute))

	var jobActivities *activities.JobActivities

	actReq := types.InitJobRequest{
		FlowType:   request.FlowType,
		SourcePath: request.SourcePath,
		TargetPath: request.TargetPath,
		Payload:    request.Payload,
	}
	var actResp types.InitJobResponse
	if err := workflow.ExecuteActivity(ctx, jobActivities.InitJobActivity, actReq).Get(ctx, &actResp); err != nil {
		return nil, fmt.Errorf("init job: %w", err)
	}
	parentJobID := actResp.Job.ID

	statusInput := types.UpdateJobStatusRequest{
		JobID:  parentJobID,
		Status: "IN_PROGRESS",
	}
	if err := workflow.ExecuteActivity(ctx, jobActivities.UpdateJobStatusActivity, statusInput).Get(ctx, nil); err != nil {
		recordWorkflowFailure(ctx, jobActivities, parentJobID, "", err)
		return nil, fmt.Errorf("update parent job status: %w", err)
	}

	dirRel := request.Payload.RelativePath
	if strings.TrimSpace(dirRel) == "" {
		dirRel = "."
	}
	dirRel = config.NormalizeRelativePath(dirRel)

	listReq := types.ListDirectoryRestoreRequest{
		SourceID:             request.Payload.SourceID,
		DirRelative:          dirRel,
		SourceDirectoryJobID: request.Payload.SourceDirectoryJobID,
		Scope:                request.Payload.Directory,
	}
	var listResp types.ListDirectoryRestoreResponse
	if err := workflow.ExecuteActivity(ctx, jobActivities.ListDirectoryRestoreActivity, listReq).Get(ctx, &listResp); err != nil {
		recordWorkflowFailure(ctx, jobActivities, parentJobID, "", err)
		return nil, fmt.Errorf("list directory restore targets: %w", err)
	}

	workflow.GetLogger(ctx).Info("Directory restore target list complete",
		zap.Int("file_count", len(listResp.Files)),
		zap.String("parent_job_id", parentJobID),
	)

	if len(listResp.Files) == 0 {
		finalizeJobReq := types.FinalizeJobRequest{
			JobID:  parentJobID,
			Status: "FAILED",
			ErrorMessage: strPtr("no eligible backup snapshots found for directory restore"),
		}
		_ = workflow.ExecuteActivity(ctx, jobActivities.FinalizeJobActivity, finalizeJobReq).Get(ctx, nil)
		return nil, fmt.Errorf("no eligible backup snapshots found for directory restore")
	}

	runtimeLim, err := loadCadenceRuntimeLimits(ctx, jobActivities)
	if err != nil {
		recordWorkflowFailure(ctx, jobActivities, parentJobID, "", err)
		return nil, fmt.Errorf("load runtime limits: %w", err)
	}
	concurrency := runtimeLim.MaxConcurrentWorkflows

	childCtxBase := workflow.WithActivityOptions(ctx, workflowActivityOptions(2*time.Hour))

	var succeeded, failed int
	total := len(listResp.Files)

	for i := 0; i < total; i += concurrency {
		end := i + concurrency
		if end > total {
			end = total
		}
		batch := listResp.Files[i:end]
		futures := make([]workflow.ChildWorkflowFuture, 0, len(batch))
		for _, f := range batch {
			childReq := childRestoreRequest(request, parentJobID, f)
			cwo := workflow.ChildWorkflowOptions{
				WorkflowID:                   fmt.Sprintf("restore_dirchild_%s_%d_%s", parentJobID, i, sanitizeWorkflowIDPart(f.RelativePath)),
				TaskList:                     workflow.GetInfo(ctx).TaskListName,
				ExecutionStartToCloseTimeout: 2 * time.Hour,
			}
			childCtx := workflow.WithChildOptions(childCtxBase, cwo)
			future := workflow.ExecuteChildWorkflow(childCtx, RestoreWorkflow, childReq)
			futures = append(futures, future)
		}
		for _, fut := range futures {
			var childResp types.BackupWorkflowResponse
			if err := fut.Get(childCtxBase, &childResp); err != nil {
				failed++
				workflow.GetLogger(ctx).Warn("Child restore failed", zap.Error(err))
				continue
			}
			succeeded++
		}
	}

	finalStatus := "SUCCESS"
	var errMsg *string
	if failed > 0 {
		finalStatus = "FAILED"
		msg := fmt.Sprintf("%d of %d file restores failed", failed, total)
		errMsg = &msg
	}

	finalizeJobReq := types.FinalizeJobRequest{
		JobID:        parentJobID,
		Status:       finalStatus,
		ErrorMessage: errMsg,
	}
	if err := workflow.ExecuteActivity(ctx, jobActivities.FinalizeJobActivity, finalizeJobReq).Get(ctx, nil); err != nil {
		return nil, fmt.Errorf("finalize parent job: %w", err)
	}

	if failed > 0 {
		return &types.DirectoryRestoreWorkflowResponse{
			JobID:          parentJobID,
			FilesTotal:     total,
			FilesSucceeded: succeeded,
			FilesFailed:    failed,
		}, fmt.Errorf("%d of %d file restores failed", failed, total)
	}

	return &types.DirectoryRestoreWorkflowResponse{
		JobID:          parentJobID,
		FilesTotal:     total,
		FilesSucceeded: succeeded,
		FilesFailed:    0,
	}, nil
}

func childRestoreRequest(parent types.BackupWorkflowRequest, parentJobID string, target types.DirectoryRestoreTarget) types.BackupWorkflowRequest {
	fileRel := config.NormalizeRelativePath(target.RelativePath)
	return types.BackupWorkflowRequest{
		FlowType:   string(schema.FlowRestore),
		SourcePath: parent.SourcePath,
		TargetPath: parent.TargetPath,
		Payload: types.BackupJobPayload{
			FileName:        fileRel,
			RelativePath:    fileRel,
			SourceID:        parent.Payload.SourceID,
			BackupKey:       target.BackupKey,
			SourceJobItemID: target.SourceJobItemID,
			ParentJobID:     parentJobID,
			RemoteIO:        parent.Payload.RemoteIO,
		},
	}
}

func strPtr(s string) *string {
	return &s
}