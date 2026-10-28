package workflows

import (
	"cosmosbdr/src/cadence/activities"
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
		JobID:    actResp.Job.ID,
		ItemPath: filepath.Join(actResp.Job.SourcePath, request.Payload.FileName),
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
		return &types.BackupWorkflowResponse{JobID: actResp.Job.ID}, nil
	}

	// CASE 2: File is new or changed -> Begin Parallel Chunking Pipeline
	fileSize := inspectResponse.JobItem.OriginalSize
	if fileSize == 0 {
		// Handle empty file edge case safely: No chunks needed
		workflow.GetLogger(ctx).Info("File is empty, skipping chunk processing segments", zap.String("path", inspectRequest.ItemPath))
		return &types.BackupWorkflowResponse{JobID: actResp.Job.ID}, nil
	}

	// 5. Calculate chunk segmentation array configurations
	const chunkSize int64 = 4 * 1024 * 1024 // Fixed 4MB window allocations
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

	// NEW: 7. Finalize and mark the file transaction row as COMPLETED
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
