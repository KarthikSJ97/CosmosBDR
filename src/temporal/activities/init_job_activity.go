package activities

import (
	"context"
	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/repositories"
	"fmt"
)

type JobActivities struct {
	JobRepo *repositories.JobRepository
}

type InitJobRequest struct {
	FlowType   string
	SourcePath string
	TargetPath string
	Payload    []byte
}

type InitJobResponse struct {
	JobID string
}

func (ja *JobActivities) InitJobActivity(ctx context.Context, request InitJobRequest) (*InitJobResponse, error) {
	if len(request.Payload) == 0 {
		request.Payload = []byte("{}")
	}

	job := &schema.Job{
		FlowType:   request.FlowType,
		Status:     "IN_PROGRESS",
		SourcePath: request.SourcePath,
		TargetPath: &request.TargetPath,
		Payload:    request.Payload,
	}

	if err := ja.JobRepo.CreateJob(job); err != nil {
		return nil, fmt.Errorf("failed to initialize job via repository: %w", err)
	}

	return &InitJobResponse{JobID: job.ID}, nil
}
