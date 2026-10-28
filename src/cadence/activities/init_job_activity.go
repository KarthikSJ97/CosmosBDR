package activities

import (
	"context"
	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/repositories"
	"encoding/json"
	"fmt"

	"gorm.io/datatypes"
)

type JobActivities struct {
	JobRepo     *repositories.JobRepository
	JobItemRepo *repositories.JobItemRepository
}

type InitJobRequest struct {
	FlowType   string
	SourcePath string
	TargetPath string
	Payload    BackupJobPayload
}

// BackupJobPayload defines the specific file details inside the payload
type BackupJobPayload struct {
	FileName     string `json:"file_name"`
	RelativePath string `json:"relative_path,omitempty"`
}

type InitJobResponse struct {
	Job *schema.Job
}

func (ja *JobActivities) InitJobActivity(ctx context.Context, request InitJobRequest) (*InitJobResponse, error) {
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

	return &InitJobResponse{Job: job}, nil
}
