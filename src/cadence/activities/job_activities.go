package activities

import (
	"cosmosbdr/src/repositories"
	"cosmosbdr/src/storage"
)

type JobActivities struct {
	JobRepo     *repositories.JobRepository
	JobItemRepo *repositories.JobItemRepository
	ChunkRepo   *repositories.ChunkRepository
	Seaweed     *storage.SeaweedClient
}