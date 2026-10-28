package repositories

import (
	"context"
	"cosmosbdr/src/db/schema"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type JobItemRepository struct {
	DB *gorm.DB
}

func NewJobItemRepository(db *gorm.DB) *JobItemRepository {
	return &JobItemRepository{DB: db}
}

// GetLatestCompletedItemByPath looks up the most recent completed job item for a given path across past jobs.
func (r *JobItemRepository) GetLatestCompletedItemByPath(ctx context.Context, itemPath string) (*schema.JobItem, error) {
	var item schema.JobItem
	err := r.DB.WithContext(ctx).
		Table("job_items").
		Joins("JOIN jobs ON jobs.id = job_items.job_id").
		Where("job_items.item_path = ? AND jobs.status = ?", itemPath, "COMPLETED").
		Order("job_items.created_at DESC").
		First(&item).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No history found, brand new file
		}
		return nil, fmt.Errorf("failed to query historical job item: %w", err)
	}

	return &item, nil
}

func (r *JobItemRepository) CreateJobItem(jobItem *schema.JobItem) error {
	return r.DB.Create(jobItem).Error
}

func (r *JobItemRepository) CloneChunksFromItem(ctx context.Context, historicalItemID string, newItemID string) error {
	var historicalChunks []schema.ItemChunk

	// 1. Fetch chunk configurations mapping records linked to the old file execution version
	err := r.DB.WithContext(ctx).Where("item_id = ?", historicalItemID).Find(&historicalChunks).Error
	if err != nil {
		return fmt.Errorf("failed to fetch historical chunks: %w", err)
	}

	// Escape gracefully if dealing with an empty file containing no chunks
	if len(historicalChunks) == 0 {
		return nil
	}

	// 2. Transpile old data definitions onto the new structural identifiers
	newChunks := make([]schema.ItemChunk, len(historicalChunks))
	for i, oldChunk := range historicalChunks {
		newChunks[i] = schema.ItemChunk{
			ItemID:      newItemID,            // Target our new JobItem record
			ChunkID:     oldChunk.ChunkID,     // Reuse the exact same global deduplicated chunk pointer
			ChunkIndex:  oldChunk.ChunkIndex,  // Maintain block sequence structure
			OffsetBytes: oldChunk.OffsetBytes, // Retain exact data positioning metrics
			LengthBytes: oldChunk.LengthBytes, // Retain block size values
		}
	}

	// 3. Batch write all mapping elements into the index in a single network round-trip
	err = r.DB.WithContext(ctx).Create(&newChunks).Error
	if err != nil {
		return fmt.Errorf("failed to bulk insert cloned chunks: %w", err)
	}

	return nil
}
