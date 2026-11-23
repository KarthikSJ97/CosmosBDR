package activities

import (
	"cosmosbdr/src/chunkcodec"
	"cosmosbdr/src/config"
	"cosmosbdr/src/repositories"
	"cosmosbdr/src/storage"
)

type JobActivities struct {
	JobRepo     *repositories.JobRepository
	JobItemRepo *repositories.JobItemRepository
	ChunkRepo   *repositories.ChunkRepository
	Seaweed     *storage.SeaweedClient
	ChunkCodec  *chunkcodec.Codec
	Registry    *config.SourceRegistry

	CadenceMaxConcurrentWorkflows        int
	CadenceWorkerMaxConcurrentActivities int

	RetentionGCItemBatchSize   int
	RetentionGCOrphanBatchSize int
}