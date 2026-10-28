package workflows

import (
	"cosmosbdr/src/cadence/activities"
	"path/filepath"
	"time"

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
	// Configure activity options (timeouts, retries)
	ao := workflow.ActivityOptions{
		StartToCloseTimeout:    30 * time.Second,
		ScheduleToStartTimeout: 5 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Map workflow request to activity request
	actReq := activities.InitJobRequest{
		FlowType:   request.FlowType,
		SourcePath: request.SourcePath,
		TargetPath: request.TargetPath,
		Payload:    request.Payload,
	}

	var actResp activities.InitJobResponse

	// Invoke the activity using interface pointer syntax
	var jobActivities *activities.JobActivities
	err := workflow.ExecuteActivity(ctx, jobActivities.InitJobActivity, actReq).Get(ctx, &actResp)
	if err != nil {
		return nil, err
	}

	// Update the job status to IN_PROGRESS
	statusInput := activities.UpdateJobStatusRequest{
		JobID:  actResp.Job.ID,
		Status: "IN_PROGRESS",
	}

	err = workflow.ExecuteActivity(ctx, jobActivities.UpdateJobStatusActivity, statusInput).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	inspectRequest := activities.InspectFileRequest{
		JobID:    actResp.Job.ID,
		ItemPath: filepath.Join(actResp.Job.SourcePath, request.Payload.FileName),
	}
	var inspectResponse activities.InspectFileResponse
	err = workflow.ExecuteActivity(ctx, jobActivities.InspectFileActivity, inspectRequest).Get(ctx, &inspectResponse)
	if err != nil {
		return nil, err
	}

	// TODO: Next step -> Split file into chunks, hash, and upload to SeaweedFS in parallel

	return &BackupWorkflowResponse{JobID: actResp.Job.ID}, nil
}
