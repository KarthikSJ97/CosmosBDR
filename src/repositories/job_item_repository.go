package repositories

import (
	"context"
	"cosmosbdr/src/db/schema"
	"crypto/sha256"
	"encoding/hex"
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
		Joins("JOIN jobs ON jobs.id = job_items.job_id").
		Where("job_items.item_path = ? AND jobs.status = ? AND job_items.status = ?", itemPath, "SUCCESS", "COMPLETED").
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

func (r *JobItemRepository) CompleteJobItem(ctx context.Context, itemID string) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Join with the global chunk table to pull actual content hashes and their sizes
		var chunks []struct {
			ChunkHash string
			SizeBytes int64
		}

		// Note: We pull size_bytes from the chunks table to track actual written size
		err := tx.Table("item_chunks").
			Select("chunks.chunk_hash, chunks.size_bytes").
			Joins("JOIN chunks ON chunks.id = item_chunks.chunk_id").
			Where("item_chunks.item_id = ?", itemID).
			Order("item_chunks.chunk_index ASC").
			Scan(&chunks).Error
		if err != nil {
			return fmt.Errorf("failed to fetch content chunk hashes: %w", err)
		}

		// 2. Compute a composite manifest signature and accumulate total processed size
		h := sha256.New()
		var totalProcessedSize int64 = 0

		for _, c := range chunks {
			h.Write([]byte(c.ChunkHash))
			totalProcessedSize += c.SizeBytes // Accumulate byte counts smoothly here
		}
		compositeChecksum := hex.EncodeToString(h.Sum(nil))

		// 3. Update the JobItem state to terminal COMPLETED
		err = tx.Model(&schema.JobItem{}).Where("id = ?", itemID).Updates(map[string]interface{}{
			"status":         "COMPLETED",
			"processed_size": totalProcessedSize, // Variable is now correctly declared and populated
			"checksum":       compositeChecksum,
		}).Error
		if err != nil {
			return fmt.Errorf("failed to finalize job item record: %w", err)
		}

		return nil
	})
}
