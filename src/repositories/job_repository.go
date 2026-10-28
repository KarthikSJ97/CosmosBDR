package repositories

import (
	"context"
	"cosmosbdr/src/db/schema"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type JobRepository struct {
	DB *gorm.DB
}

func NewJobRepository(db *gorm.DB) *JobRepository {
	return &JobRepository{DB: db}
}

func (r *JobRepository) CreateJob(job *schema.Job) error {
	return r.DB.Create(job).Error
}

// UpdateJobStatus updates the job status and updated_at timestamp by ID.
func (r *JobRepository) UpdateJobStatus(ctx context.Context, jobID string, targetStatus string) error {
	result := r.DB.WithContext(ctx).Table("jobs").
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":     targetStatus,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update job status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("job not found: %s", jobID)
	}

	return nil
}

// GetJobStatus fetches the current status of a job (useful for validation in activities)
func (r *JobRepository) GetJobStatus(ctx context.Context, jobID string) (string, error) {
	var status string
	err := r.DB.WithContext(ctx).Table("jobs").
		Select("status").
		Where("id = ?", jobID).
		Scan(&status).Error

	if err != nil {
		return "", fmt.Errorf("failed to fetch job status: %w", err)
	}

	if status == "" {
		return "", fmt.Errorf("job not found: %s", jobID)
	}

	return status, nil
}
