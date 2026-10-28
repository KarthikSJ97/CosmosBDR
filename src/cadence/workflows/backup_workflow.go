package workflows

import (
	"cosmosbdr/src/cadence/activities"
	"encoding/json"
	"time"

	"go.uber.org/cadence/workflow"
)

type BackupWorkflowRequest struct {
	FlowType   string          `json:"flow_type"`
	SourcePath string          `json:"source_path"`
	TargetPath string          `json:"target_path"`
	Payload    json.RawMessage `json:"payload"`
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
		JobID:  actResp.JobID,
		Status: "IN_PROGRESS",
	}

	err = workflow.ExecuteActivity(ctx, jobActivities.UpdateJobStatusActivity, statusInput).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	// TODO: Next steps (e.g. pending job picker, file checking, chunking, and parallel SeaweedFS uploads)

	return &BackupWorkflowResponse{JobID: actResp.JobID}, nil
}
