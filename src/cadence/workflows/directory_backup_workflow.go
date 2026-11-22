package workflows

import (
	"cosmosbdr/src/cadence/activities"
	"cosmosbdr/src/config"
	"cosmosbdr/src/types"
	"fmt"
	"strings"
	"time"

	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"
)

const directoryListTimeout = 30 * time.Minute

func DirectoryBackupWorkflow(ctx workflow.Context, request types.BackupWorkflowRequest) (*types.DirectoryBackupWorkflowResponse, error) {
	if request.Payload.Directory == nil {
		return nil, fmt.Errorf("directory scope is required")
	}
	listStartToClose := 5 * time.Minute
	if request.Payload.RemoteIO {
		listStartToClose = directoryListTimeout
	}

	ctx = workflow.WithActivityOptions(ctx, workflowActivityOptions(listStartToClose))

	actReq := types.InitJobRequest{
		FlowType:   request.FlowType,
		SourcePath: request.SourcePath,
		TargetPath: request.TargetPath,
		Payload:    request.Payload,
	}

	var actResp types.InitJobResponse
	var jobActivities *activities.JobActivities

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

	listReq := types.ListDirectoryRequest{
		SourceID:     request.Payload.SourceID,
		DirRelative:  dirRel,
		Scope:        *request.Payload.Directory,
		ListRootPath: request.ListRootPath,
		SourcePrefix: request.SourcePath,
		RemoteIO:     request.Payload.RemoteIO,
	}

	var listResp types.ListDirectoryResponse
	if err := workflow.ExecuteActivity(ctx, jobActivities.ListDirectoryActivity, listReq).Get(ctx, &listResp); err != nil {
		recordWorkflowFailure(ctx, jobActivities, parentJobID, "", err)
		return nil, fmt.Errorf("list directory: %w", err)
	}

	workflow.GetLogger(ctx).Info("Directory listing complete",
		zap.Int("file_count", len(listResp.Files)),
		zap.String("parent_job_id", parentJobID),
	)

	runtimeLim, err := loadCadenceRuntimeLimits(ctx, jobActivities)
	if err != nil {
		recordWorkflowFailure(ctx, jobActivities, parentJobID, "", err)
		return nil, fmt.Errorf("load runtime limits: %w", err)
	}
	concurrency := runtimeLim.MaxConcurrentWorkflows

	childCtxBase := workflow.WithActivityOptions(ctx, workflowActivityOptions(2*time.Hour))

	remoteIO := request.Payload.RemoteIO
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
			childReq, err := childBackupRequest(request, parentJobID, f.RelativeToSource, remoteIO)
			if err != nil {
				failed++
				continue
			}
			cwo := workflow.ChildWorkflowOptions{
				WorkflowID:               fmt.Sprintf("backup_dirchild_%s_%d_%s", parentJobID, i, sanitizeWorkflowIDPart(f.RelativeToSource)),
				TaskList:                 workflow.GetInfo(ctx).TaskListName,
				ExecutionStartToCloseTimeout: 2 * time.Hour,
			}
			childCtx := workflow.WithChildOptions(childCtxBase, cwo)
			future := workflow.ExecuteChildWorkflow(childCtx, BackupWorkflow, childReq)
			futures = append(futures, future)
		}
		for _, fut := range futures {
			var childResp types.BackupWorkflowResponse
			if err := fut.Get(childCtxBase, &childResp); err != nil {
				failed++
				workflow.GetLogger(ctx).Warn("Child backup failed", zap.Error(err))
				continue
			}
			succeeded++
		}
	}

	finalStatus := "SUCCESS"
	var errMsg *string
	if failed > 0 {
		finalStatus = "FAILED"
		msg := fmt.Sprintf("%d of %d file backups failed", failed, total)
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
		return &types.DirectoryBackupWorkflowResponse{
			JobID:          parentJobID,
			FilesTotal:     total,
			FilesSucceeded: succeeded,
			FilesFailed:    failed,
		}, fmt.Errorf("%d of %d file backups failed", failed, total)
	}

	return &types.DirectoryBackupWorkflowResponse{
		JobID:          parentJobID,
		FilesTotal:     total,
		FilesSucceeded: succeeded,
		FilesFailed:    0,
	}, nil
}

func childBackupRequest(parent types.BackupWorkflowRequest, parentJobID, fileRel string, remoteIO bool) (types.BackupWorkflowRequest, error) {
	fileRel = config.NormalizeRelativePath(fileRel)
	if fileRel == "" {
		return types.BackupWorkflowRequest{}, fmt.Errorf("empty file relative path")
	}
	backupKey := config.FormatBackupKey(parent.Payload.SourceID, fileRel)
	return types.BackupWorkflowRequest{
		FlowType:   parent.FlowType,
		SourcePath: parent.SourcePath,
		Payload: types.BackupJobPayload{
			FileName:     fileRel,
			RelativePath: fileRel,
			SourceID:     parent.Payload.SourceID,
			BackupKey:    backupKey,
			RemoteIO:     remoteIO,
			ParentJobID:  parentJobID,
		},
	}, nil
}

func sanitizeWorkflowIDPart(rel string) string {
	rel = strings.ReplaceAll(rel, "/", "_")
	rel = strings.ReplaceAll(rel, " ", "_")
	if len(rel) > 120 {
		rel = rel[:120]
	}
	return rel
}