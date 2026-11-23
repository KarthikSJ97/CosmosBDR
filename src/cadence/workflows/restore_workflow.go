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

func RestoreWorkflow(ctx workflow.Context, request types.BackupWorkflowRequest) (*types.BackupWorkflowResponse, error) {
	ctx = workflow.WithActivityOptions(ctx, workflowActivityOptions(120*time.Second))

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

	resp, jobItemID, err := runRestoreWorkflow(ctx, jobActivities, request, actResp)
	if err != nil {
		recordWorkflowFailure(ctx, jobActivities, jobID, jobItemID, err)
		return nil, err
	}
	return resp, nil
}

func runRestoreWorkflow(
	ctx workflow.Context,
	jobActivities *activities.JobActivities,
	request types.BackupWorkflowRequest,
	actResp types.InitJobResponse,
) (*types.BackupWorkflowResponse, string, error) {
	itemPath := filepath.Join(request.SourcePath, request.Payload.FileName)
	statusInput := types.UpdateJobStatusRequest{
		JobID:  actResp.Job.ID,
		Status: "IN_PROGRESS",
	}
	if err := workflow.ExecuteActivity(ctx, jobActivities.UpdateJobStatusActivity, statusInput).Get(ctx, nil); err != nil {
		return nil, "", fmt.Errorf("failed to update job status to IN_PROGRESS: %w", err)
	}

	resolveReq := types.ResolveRestoreSourceRequest{
		JobID:           actResp.Job.ID,
		ItemPath:        itemPath,
		BackupKey:       request.Payload.BackupKey,
		SourceJobItemID: request.Payload.SourceJobItemID,
		TargetPath:      request.TargetPath,
	}
	var resolveResp types.ResolveRestoreSourceResponse
	if err := workflow.ExecuteActivity(ctx, jobActivities.ResolveRestoreSourceActivity, resolveReq).Get(ctx, &resolveResp); err != nil {
		return nil, "", fmt.Errorf("failed to execute ResolveRestoreSourceActivity: %w", err)
	}
	jobItemID := resolveResp.RestoreJobItem.ID

	prepareReq := types.PrepareRestoreTargetRequest{
		DestinationPath: resolveResp.DestinationPath,
		SourceID:        resolveResp.SourceID,
		RemoteIO:        resolveResp.RemoteIO,
		OriginalSize:    resolveResp.OriginalSize,
	}
	if err := workflow.ExecuteActivity(ctx, jobActivities.PrepareRestoreTargetActivity, prepareReq).Get(ctx, nil); err != nil {
		return nil, jobItemID, fmt.Errorf("failed to execute PrepareRestoreTargetActivity: %w", err)
	}

	if resolveResp.OriginalSize > 0 && len(resolveResp.Chunks) > 0 {
		runtimeLim, err := loadCadenceRuntimeLimits(ctx, jobActivities)
		if err != nil {
			return nil, jobItemID, fmt.Errorf("load runtime limits: %w", err)
		}
		waveCap := runtimeLim.WorkerMaxConcurrentActivities
		chunks := resolveResp.Chunks

		for i := 0; i < len(chunks); i += waveCap {
			end := i + waveCap
			if end > len(chunks) {
				end = len(chunks)
			}
			batch := chunks[i:end]
			var futures []workflow.Future
			for _, chunk := range batch {
				chunkReq := types.RestoreFileChunkRequest{
					DestinationPath: resolveResp.DestinationPath,
					SourceID:        resolveResp.SourceID,
					RemoteIO:        resolveResp.RemoteIO,
					ChunkIndex:      chunk.ChunkIndex,
					OffsetBytes:     chunk.OffsetBytes,
					LengthBytes:     chunk.LengthBytes,
					ChunkHash:       chunk.ChunkHash,
					SeaweedKey:      chunk.SeaweedKey,
					CompressionAlg:  chunk.CompressionAlg,
					Encrypted:       chunk.Encrypted,
				}
				futures = append(futures, workflow.ExecuteActivity(ctx, jobActivities.RestoreFileChunkActivity, chunkReq))
			}
			for j, future := range futures {
				var chunkResp types.RestoreFileChunkResponse
				if err := future.Get(ctx, &chunkResp); err != nil {
					return nil, jobItemID, fmt.Errorf("restore chunk failed at index %d: %w", batch[j].ChunkIndex, err)
				}
			}
		}
	} else {
		workflow.GetLogger(ctx).Info("Restore target has no chunks; empty file prepared",
			zap.String("path", resolveResp.DestinationPath),
		)
	}

	completeReq := types.CompleteJobItemRequest{
		JobID:  actResp.Job.ID,
		ItemID: resolveResp.RestoreJobItem.ID,
	}
	var completeResp types.CompleteJobItemResponse
	if err := workflow.ExecuteActivity(ctx, jobActivities.CompleteJobItemActivity, completeReq).Get(ctx, &completeResp); err != nil {
		return nil, jobItemID, fmt.Errorf("failed to execute CompleteJobItemActivity: %w", err)
	}

	workflow.GetLogger(ctx).Info("File restored and job item marked complete",
		zap.String("job_id", actResp.Job.ID),
		zap.String("item_id", completeResp.ItemID),
		zap.String("source_job_item_id", resolveResp.SourceJobItemID),
		zap.String("destination_path", resolveResp.DestinationPath),
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