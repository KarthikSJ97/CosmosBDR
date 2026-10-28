package repositories

import (
	"context"
	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/types"
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

func (r *JobRepository) FinalizeJobExecution(ctx context.Context, input types.FinalizeJobRequest) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var totalItems int64
		var totalBytes int64

		// 1. Gather all file item aggregates belonging to this job run execution
		err := tx.Model(&schema.JobItem{}).Where("job_id = ?", input.JobID).Count(&totalItems).Error
		if err != nil {
			return err
		}

		err = tx.Model(&schema.JobItem{}).Where("job_id = ?", input.JobID).Select("COALESCE(SUM(original_size), 0)").Scan(&totalBytes).Error
		if err != nil {
			return err
		}

		// 2. Finalize parent job parameters
		updates := map[string]interface{}{
			"status":        input.Status,
			"total_items":   totalItems,
			"total_bytes":   totalBytes,
			"error_message": input.ErrorMessage,
		}

		return tx.Model(&schema.Job{}).Where("id = ?", input.JobID).Updates(updates).Error
	})
}
