package activities

import (
	"cosmosbdr/src/repositories"
)

type JobActivities struct {
	JobRepo     *repositories.JobRepository
	JobItemRepo *repositories.JobItemRepository
	ChunkRepo   *repositories.ChunkRepository
}