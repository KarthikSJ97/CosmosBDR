package workflows

import (
	"cosmosbdr/src/cadence/activities"
	"cosmosbdr/src/manifest"
	"cosmosbdr/src/types"
	"fmt"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"go.uber.org/cadence/workflow"
)

func BackupWorkflow(ctx workflow.Context, request types.BackupWorkflowRequest) (*types.BackupWorkflowResponse, error) {
	startToClose := 120 * time.Second
	if request.Payload.RemoteIO {
		startToClose = 20 * time.Minute
	}
	ctx = workflow.WithActivityOptions(ctx, workflowActivityOptions(startToClose))

	var jobActivities *activities.JobActivities

	actReq := types.InitJobRequest{
		FlowType:   request.FlowType,
		SourcePath: request.SourcePath,
		TargetPath: request.TargetPath,
		Payload:    request.Payload,
	}
	var actResp types.InitJobResponse
	if err := workflow.ExecuteActivity(ctx, jobActivities.InitJobActivity, actReq).Get(ctx, &actResp); err != nil {
		return nil, fmt.Errorf("failed to execute InitJobActivity: %w", err)
	}
	jobID := actResp.Job.ID

	resp, jobItemID, err := runBackupWorkflow(ctx, jobActivities, request, actResp)
	if err != nil {
		recordWorkflowFailure(ctx, jobActivities, jobID, jobItemID, err)
		return nil, err
	}
	return resp, nil
}

func runBackupWorkflow(
	ctx workflow.Context,
	jobActivities *activities.JobActivities,
	request types.BackupWorkflowRequest,
	actResp types.InitJobResponse,
) (*types.BackupWorkflowResponse, string, error) {
	statusInput := types.UpdateJobStatusRequest{
		JobID:  actResp.Job.ID,
		Status: "IN_PROGRESS",
	}
	if err := workflow.ExecuteActivity(ctx, jobActivities.UpdateJobStatusActivity, statusInput).Get(ctx, nil); err != nil {
		return nil, "", fmt.Errorf("failed to update job status to IN_PROGRESS: %w", err)
	}

	itemPath := filepath.Join(actResp.Job.SourcePath, request.Payload.FileName)
	inspectRequest := types.InspectFileRequest{
		JobID:     actResp.Job.ID,
		ItemPath:  itemPath,
		SourceID:  request.Payload.SourceID,
		BackupKey: request.Payload.BackupKey,
	}
	var inspectResponse types.InspectFileResponse
	if err := workflow.ExecuteActivity(ctx, jobActivities.InspectFileActivity, inspectRequest).Get(ctx, &inspectResponse); err != nil {
		return nil, "", fmt.Errorf("failed to execute InspectFileActivity: %w", err)
	}
	jobItemID := inspectResponse.JobItem.ID

	if inspectResponse.Action == "SKIP" {
		workflow.GetLogger(ctx).Info("File processing skipped due to deduplication baseline match", zap.String("path", inspectRequest.ItemPath))

		var manifestResp types.PublishSnapshotManifestResponse
		publishReq := types.PublishSnapshotManifestRequest{
			ItemID:   inspectResponse.JobItem.ID,
			FileName: request.Payload.FileName,
		}
		if err := workflow.ExecuteActivity(ctx, jobActivities.PublishSnapshotManifestActivity, publishReq).Get(ctx, &manifestResp); err != nil {
			return nil, jobItemID, fmt.Errorf("failed to publish snapshot manifest for skipped file: %w", err)
		}

		skipFinalizeReq := types.FinalizeSkippedBackupItemRequest{
			ItemID:      inspectResponse.JobItem.ID,
			SkipReason:  inspectResponse.Reason,
			ManifestKey: manifestResp.ManifestKey,
		}
		if err := workflow.ExecuteActivity(ctx, jobActivities.FinalizeSkippedBackupItemActivity, skipFinalizeReq).Get(ctx, nil); err != nil {
			return nil, jobItemID, fmt.Errorf("failed to record skipped backup in database: %w", err)
		}

		finalizeJobReq := types.FinalizeJobRequest{
			JobID:  actResp.Job.ID,
			Status: "SUCCESS",
		}
		if err := workflow.ExecuteActivity(ctx, jobActivities.FinalizeJobActivity, finalizeJobReq).Get(ctx, nil); err != nil {
			return nil, jobItemID, fmt.Errorf("failed to finalize skipped job status: %w", err)
		}

		return &types.BackupWorkflowResponse{JobID: actResp.Job.ID}, jobItemID, nil
	}

	fileSize := inspectResponse.JobItem.OriginalSize
	if fileSize == 0 {
		workflow.GetLogger(ctx).Info("File is empty, skipping chunk processing segments", zap.String("path", inspectRequest.ItemPath))

		emptyPublishReq := types.PublishSnapshotManifestRequest{
			ItemID:   inspectResponse.JobItem.ID,
			FileName: request.Payload.FileName,
		}
		if err := workflow.ExecuteActivity(ctx, jobActivities.PublishSnapshotManifestActivity, emptyPublishReq).Get(ctx, nil); err != nil {
			return nil, jobItemID, fmt.Errorf("failed to publish snapshot manifest for empty file: %w", err)
		}

		emptyCompleteReq := types.CompleteJobItemRequest{
			JobID:  actResp.Job.ID,
			ItemID: inspectResponse.JobItem.ID,
		}
		if err := workflow.ExecuteActivity(ctx, jobActivities.CompleteJobItemActivity, emptyCompleteReq).Get(ctx, nil); err != nil {
			return nil, jobItemID, fmt.Errorf("failed to complete empty file job item: %w", err)
		}

		finalizeJobReq := types.FinalizeJobRequest{
			JobID:  actResp.Job.ID,
			Status: "SUCCESS",
		}
		if err := workflow.ExecuteActivity(ctx, jobActivities.FinalizeJobActivity, finalizeJobReq).Get(ctx, nil); err != nil {
			return nil, jobItemID, fmt.Errorf("failed to finalize empty file job: %w", err)
		}

		return &types.BackupWorkflowResponse{JobID: actResp.Job.ID}, jobItemID, nil
	}

	chunkSize := int64(manifest.ChunkSizeBytes)
	var offset int64 = 0
	chunkIndex := 0

	runtimeLim, err := loadCadenceRuntimeLimits(ctx, jobActivities)
	if err != nil {
		return nil, jobItemID, fmt.Errorf("load runtime limits: %w", err)
	}
	waveCap := runtimeLim.WorkerMaxConcurrentActivities

	for offset < fileSize {
		var batch []workflow.Future
		var batchIndexes []int

		for len(batch) < waveCap && offset < fileSize {
			length := chunkSize
			if offset+length > fileSize {
				length = fileSize - offset
			}

			chunkReq := types.ProcessFileChunkRequest{
				JobID:       actResp.Job.ID,
				ItemID:      inspectResponse.JobItem.ID,
				ItemPath:    inspectRequest.ItemPath,
				SourceID:    request.Payload.SourceID,
				FileName:    request.Payload.FileName,
				ChunkIndex:  chunkIndex,
				OffsetBytes: offset,
				LengthBytes: int(length),
			}
			batch = append(batch, workflow.ExecuteActivity(ctx, jobActivities.ProcessFileChunkActivity, chunkReq))
			batchIndexes = append(batchIndexes, chunkIndex)

			offset += chunkSize
			chunkIndex++
		}

		for i, future := range batch {
			var chunkResp types.ProcessFileChunkResponse
			if err := future.Get(ctx, &chunkResp); err != nil {
				return nil, jobItemID, fmt.Errorf("chunk processing failed at index %d: %w", batchIndexes[i], err)
			}
		}
	}

	publishReq := types.PublishSnapshotManifestRequest{
		ItemID:   inspectResponse.JobItem.ID,
		FileName: request.Payload.FileName,
	}
	if err := workflow.ExecuteActivity(ctx, jobActivities.PublishSnapshotManifestActivity, publishReq).Get(ctx, nil); err != nil {
		return nil, jobItemID, fmt.Errorf("failed to publish snapshot manifest: %w", err)
	}

	finalizeReq := types.CompleteJobItemRequest{
		JobID:  actResp.Job.ID,
		ItemID: inspectResponse.JobItem.ID,
	}
	var finalizeResp types.CompleteJobItemResponse
	if err := workflow.ExecuteActivity(ctx, jobActivities.CompleteJobItemActivity, finalizeReq).Get(ctx, &finalizeResp); err != nil {
		return nil, jobItemID, fmt.Errorf("failed to execute CompleteJobItemActivity: %w", err)
	}

	workflow.GetLogger(ctx).Info("All chunks processed and job item marked complete",
		zap.String("job_id", actResp.Job.ID),
		zap.String("item_id", finalizeResp.ItemID),
	)

	finalizeJobReq := types.FinalizeJobRequest{
		JobID:  actResp.Job.ID,
		Status: "SUCCESS",
	}
	if err := workflow.ExecuteActivity(ctx, jobActivities.FinalizeJobActivity, finalizeJobReq).Get(ctx, nil); err != nil {
		return nil, jobItemID, fmt.Errorf("failed to finalize job status: %w", err)
	}

	return &types.BackupWorkflowResponse{JobID: actResp.Job.ID}, jobItemID, nil
}