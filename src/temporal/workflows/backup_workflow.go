package workflows

import (
	"time"
	"cosmosbdr/src/temporal/activities"
	"go.temporal.io/sdk/workflow"
)

type BackupWorkflowRequest struct {
	FlowType   string
	SourcePath string
	TargetPath string
	Payload    []byte
}

type BackupWorkflowResponse struct {
	JobID string
}

func BackupWorkflow(ctx workflow.Context, request BackupWorkflowRequest) (*BackupWorkflowResponse, error) {
	// Configure activity options (timeouts, retries)
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
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

	return &BackupWorkflowResponse{JobID: actResp.JobID}, nil
}