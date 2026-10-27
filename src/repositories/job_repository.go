package repositories

import (
	"cosmosbdr/src/db/schema"

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
