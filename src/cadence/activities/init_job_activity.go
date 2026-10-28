package activities

import (
	"context"
	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/types"
	"encoding/json"
	"fmt"

	"gorm.io/datatypes"
)

func (ja *JobActivities) InitJobActivity(ctx context.Context, request types.InitJobRequest) (*types.InitJobResponse, error) {
	payloadBytes, err := json.Marshal("")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload struct: %w", err)
	}

	job := &schema.Job{
		FlowType:   request.FlowType,
		Status:     "CREATED",
		SourcePath: request.SourcePath,
		TargetPath: &request.TargetPath,
		Payload:    datatypes.JSON(payloadBytes),
	}

	if err := ja.JobRepo.CreateJob(job); err != nil {
		return nil, fmt.Errorf("failed to initialize job via repository: %w", err)
	}

	return &types.InitJobResponse{Job: job}, nil
}
