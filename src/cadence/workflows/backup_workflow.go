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
	// 1. Configure activity options (timeouts, retries)
	ao := workflow.ActivityOptions{
		StartToCloseTimeout:    120 * time.Second, // Increased to account for chunk file I/O & hashing
		ScheduleToStartTimeout: 5 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// 2. Initialize Job record via activity
	actReq := types.InitJobRequest{
		FlowType:   request.FlowType,
		SourcePath: request.SourcePath,
		TargetPath: request.TargetPath,
		Payload:    request.Payload,
	}

	var actResp types.InitJobResponse
	var jobActivities *activities.JobActivities

	err := workflow.ExecuteActivity(ctx, jobActivities.InitJobActivity, actReq).Get(ctx, &actResp)
	if err != nil {
		return nil, fmt.Errorf("failed to execute InitJobActivity: %w", err)
	}

	// 3. Update the job status to IN_PROGRESS
	statusInput := types.UpdateJobStatusRequest{
		JobID:  actResp.Job.ID,
		Status: "IN_PROGRESS",
	}
	err = workflow.ExecuteActivity(ctx, jobActivities.UpdateJobStatusActivity, statusInput).Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update job status to IN_PROGRESS: %w", err)
	}

	// 4. Inspect file for duplicate/change checking rules
	inspectRequest := types.InspectFileRequest{
		JobID:      actResp.Job.ID,
		ItemPath:   filepath.Join(actResp.Job.SourcePath, request.Payload.FileName),
		BackupKey:  request.Payload.BackupKey,
	}
	var inspectResponse types.InspectFileResponse
	err = workflow.ExecuteActivity(ctx, jobActivities.InspectFileActivity, inspectRequest).Get(ctx, &inspectResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to execute InspectFileActivity: %w", err)
	}

	// CASE 1: File is skipped because it matches history baseline perfectly
	if inspectResponse.Action == "SKIP" {
		// Log message via replay-safe cadence logger
		workflow.GetLogger(ctx).Info("File processing skipped due to deduplication baseline match", zap.String("path", inspectRequest.ItemPath))

		var manifestResp types.PublishSnapshotManifestResponse
		publishReq := types.PublishSnapshotManifestRequest{
			ItemID:   inspectResponse.JobItem.ID,
			FileName: request.Payload.FileName,
		}
		if err := workflow.ExecuteActivity(ctx, jobActivities.PublishSnapshotManifestActivity, publishReq).Get(ctx, &manifestResp); err != nil {
			return nil, fmt.Errorf("failed to publish snapshot manifest for skipped file: %w", err)
		}

		skipFinalizeReq := types.FinalizeSkippedBackupItemRequest{
			ItemID:      inspectResponse.JobItem.ID,
			SkipReason:  inspectResponse.Reason,
			ManifestKey: manifestResp.ManifestKey,
		}
		if err := workflow.ExecuteActivity(ctx, jobActivities.FinalizeSkippedBackupItemActivity, skipFinalizeReq).Get(ctx, nil); err != nil {
			return nil, fmt.Errorf("failed to record skipped backup in database: %w", err)
		}

		// Trigger the job finalization activity to mark the parent Job as SUCCESS before returning
		finalizeJobReq := types.FinalizeJobRequest{
			JobID:  actResp.Job.ID,
			Status: "SUCCESS",
		}
		err = workflow.ExecuteActivity(ctx, jobActivities.FinalizeJobActivity, finalizeJobReq).Get(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to finalize skipped job status: %w", err)
		}

		return &types.BackupWorkflowResponse{JobID: actResp.Job.ID}, nil
	}

	// CASE 2: File is new or changed -> Begin Parallel Chunking Pipeline
	fileSize := inspectResponse.JobItem.OriginalSize
	if fileSize == 0 {
		workflow.GetLogger(ctx).Info("File is empty, skipping chunk processing segments", zap.String("path", inspectRequest.ItemPath))

		emptyPublishReq := types.PublishSnapshotManifestRequest{
			ItemID:   inspectResponse.JobItem.ID,
			FileName: request.Payload.FileName,
		}
		if err := workflow.ExecuteActivity(ctx, jobActivities.PublishSnapshotManifestActivity, emptyPublishReq).Get(ctx, nil); err != nil {
			return nil, fmt.Errorf("failed to publish snapshot manifest for empty file: %w", err)
		}

		emptyCompleteReq := types.CompleteJobItemRequest{
			JobID:  actResp.Job.ID,
			ItemID: inspectResponse.JobItem.ID,
		}
		if err := workflow.ExecuteActivity(ctx, jobActivities.CompleteJobItemActivity, emptyCompleteReq).Get(ctx, nil); err != nil {
			return nil, fmt.Errorf("failed to complete empty file job item: %w", err)
		}

		finalizeJobReq := types.FinalizeJobRequest{
			JobID:  actResp.Job.ID,
			Status: "SUCCESS",
		}
		if err := workflow.ExecuteActivity(ctx, jobActivities.FinalizeJobActivity, finalizeJobReq).Get(ctx, nil); err != nil {
			return nil, fmt.Errorf("failed to finalize empty file job: %w", err)
		}

		return &types.BackupWorkflowResponse{JobID: actResp.Job.ID}, nil
	}

	// 5. Calculate chunk segmentation array configurations
	chunkSize := int64(manifest.ChunkSizeBytes)
	var offset int64 = 0
	chunkIndex := 0

	// Create an array list slice to capture asynchronous future handlers
	var futures []workflow.Future

	for offset < fileSize {
		length := chunkSize
		if offset+length > fileSize {
			length = fileSize - offset // Truncate segment bounds for remaining tail bytes
		}

		chunkReq := types.ProcessFileChunkRequest{
			JobID:       actResp.Job.ID,
			ItemID:      inspectResponse.JobItem.ID,
			ItemPath:    inspectRequest.ItemPath,
			FileName:    request.Payload.FileName,
			ChunkIndex:  chunkIndex,
			OffsetBytes: offset,
			LengthBytes: int(length),
		}

		// Execute activity asynchronously (returns a Future immediately without blocking)
		future := workflow.ExecuteActivity(ctx, jobActivities.ProcessFileChunkActivity, chunkReq)
		futures = append(futures, future)

		offset += chunkSize
		chunkIndex++
	}

	// 6. Gather and synchronize all concurrent thread executions (Fan-in)
	for i, future := range futures {
		var chunkResp types.ProcessFileChunkResponse
		err := future.Get(ctx, &chunkResp)
		if err != nil {
			return nil, fmt.Errorf("parallel chunk processing failed at index %d: %w", i, err)
		}
	}

	// 7. DR manifest (strict) then mark the file row COMPLETED
	publishReq := types.PublishSnapshotManifestRequest{
		ItemID:   inspectResponse.JobItem.ID,
		FileName: request.Payload.FileName,
	}
	if err := workflow.ExecuteActivity(ctx, jobActivities.PublishSnapshotManifestActivity, publishReq).Get(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to publish snapshot manifest: %w", err)
	}

	finalizeReq := types.CompleteJobItemRequest{
		JobID:  actResp.Job.ID,
		ItemID: inspectResponse.JobItem.ID,
	}
	var finalizeResp types.CompleteJobItemResponse
	err = workflow.ExecuteActivity(ctx, jobActivities.CompleteJobItemActivity, finalizeReq).Get(ctx, &finalizeResp)
	if err != nil {
		return nil, fmt.Errorf("failed to execute CompleteJobItemActivity: %w", err)
	}

	workflow.GetLogger(ctx).Info("All chunks processed and job item marked complete",
		zap.String("job_id", actResp.Job.ID),
		zap.String("item_id", finalizeResp.ItemID),
	)

	// Final Step: Finalize job as SUCCESS
	finalizeJobReq := types.FinalizeJobRequest{
		JobID:  actResp.Job.ID,
		Status: "SUCCESS",
	}
	err = workflow.ExecuteActivity(ctx, jobActivities.FinalizeJobActivity, finalizeJobReq).Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to finalize job status: %w", err)
	}

	return &types.BackupWorkflowResponse{JobID: actResp.Job.ID}, nil
}
