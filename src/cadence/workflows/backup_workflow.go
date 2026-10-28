package workflows

import (
	"cosmosbdr/src/cadence/activities"
	"fmt"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"go.uber.org/cadence/workflow"
)

type BackupWorkflowRequest struct {
	FlowType   string                      `json:"flow_type"`
	SourcePath string                      `json:"source_path"`
	TargetPath string                      `json:"target_path"`
	Payload    activities.BackupJobPayload `json:"payload"`
}

type BackupWorkflowResponse struct {
	JobID string
}

func BackupWorkflow(ctx workflow.Context, request BackupWorkflowRequest) (*BackupWorkflowResponse, error) {
	// 1. Configure activity options (timeouts, retries)
	ao := workflow.ActivityOptions{
		StartToCloseTimeout:    120 * time.Second, // Increased to account for chunk file I/O & hashing
		ScheduleToStartTimeout: 5 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// 2. Initialize Job record via activity
	actReq := activities.InitJobRequest{
		FlowType:   request.FlowType,
		SourcePath: request.SourcePath,
		TargetPath: request.TargetPath,
		Payload:    request.Payload,
	}

	var actResp activities.InitJobResponse
	var jobActivities *activities.JobActivities

	err := workflow.ExecuteActivity(ctx, jobActivities.InitJobActivity, actReq).Get(ctx, &actResp)
	if err != nil {
		return nil, fmt.Errorf("failed to execute InitJobActivity: %w", err)
	}

	// 3. Update the job status to IN_PROGRESS
	statusInput := activities.UpdateJobStatusRequest{
		JobID:  actResp.Job.ID,
		Status: "IN_PROGRESS",
	}
	err = workflow.ExecuteActivity(ctx, jobActivities.UpdateJobStatusActivity, statusInput).Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update job status to IN_PROGRESS: %w", err)
	}

	// 4. Inspect file for duplicate/change checking rules
	inspectRequest := activities.InspectFileRequest{
		JobID:    actResp.Job.ID,
		ItemPath: filepath.Join(actResp.Job.SourcePath, request.Payload.FileName),
	}
	var inspectResponse activities.InspectFileResponse
	err = workflow.ExecuteActivity(ctx, jobActivities.InspectFileActivity, inspectRequest).Get(ctx, &inspectResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to execute InspectFileActivity: %w", err)
	}

	// CASE 1: File is skipped because it matches history baseline perfectly
	if inspectResponse.Action == "SKIP" {
		// Log message via replay-safe cadence logger
		workflow.GetLogger(ctx).Info("File processing skipped due to deduplication baseline match", zap.String("path", inspectRequest.ItemPath))
		return &BackupWorkflowResponse{JobID: actResp.Job.ID}, nil
	}

	// CASE 2: File is new or changed -> Begin Parallel Chunking Pipeline
	fileSize := inspectResponse.JobItem.OriginalSize
	if fileSize == 0 {
		// Handle empty file edge case safely: No chunks needed
		workflow.GetLogger(ctx).Info("File is empty, skipping chunk processing segments", zap.String("path", inspectRequest.ItemPath))
		return &BackupWorkflowResponse{JobID: actResp.Job.ID}, nil
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

		chunkReq := activities.ProcessFileChunkRequest{
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
		var chunkResp activities.ProcessFileChunkResponse
		err := future.Get(ctx, &chunkResp)
		if err != nil {
			return nil, fmt.Errorf("parallel chunk processing failed at index %d: %w", i, err)
		}
	}

	workflow.GetLogger(ctx).Info("All chunks processed successfully", zap.String("job_id", actResp.Job.ID), zap.Int("total_chunks", len(futures)))

	return &BackupWorkflowResponse{JobID: actResp.Job.ID}, nil
}
