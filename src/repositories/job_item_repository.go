package repositories

import (
	"context"
	"cosmosbdr/src/db/schema"
	"errors"
	"fmt"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type JobItemRepository struct {
	DB *gorm.DB
}

// backupSnapshotStatuses are job_item rows that represent a successful file backup snapshot.
var backupSnapshotStatuses = []string{"COMPLETED", "SKIPPED"}

func NewJobItemRepository(db *gorm.DB) *JobItemRepository {
	return &JobItemRepository{DB: db}
}

// GetLatestCompletedItemByPath looks up the most recent completed job item for a given path across past jobs.
func (r *JobItemRepository) GetLatestCompletedItemByPath(ctx context.Context, itemPath string) (*schema.JobItem, error) {
	var item schema.JobItem
	err := r.DB.WithContext(ctx).
		Joins("JOIN jobs ON jobs.id = job_items.job_id").
		Where("job_items.item_path = ? AND jobs.status = ? AND jobs.flow_type = ? AND job_items.status IN ?",
			itemPath, "SUCCESS", string(schema.FlowBackup), backupSnapshotStatuses).
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

// GetEligibleBackupItemByID loads a restorable snapshot item from a successful BACKUP job.
func (r *JobItemRepository) GetEligibleBackupItemByID(ctx context.Context, itemID string) (*schema.JobItem, error) {
	var item schema.JobItem
	err := r.DB.WithContext(ctx).
		Joins("JOIN jobs ON jobs.id = job_items.job_id").
		Where("job_items.id = ? AND jobs.status = ? AND jobs.flow_type = ? AND job_items.status IN ?",
			itemID, "SUCCESS", string(schema.FlowBackup), backupSnapshotStatuses).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query backup job item: %w", err)
	}
	return &item, nil
}

// GetLatestEligibleBackupItemByPath is like GetLatestCompletedItemByPath (BACKUP-only).
func (r *JobItemRepository) GetLatestEligibleBackupItemByPath(ctx context.Context, itemPath string) (*schema.JobItem, error) {
	return r.GetLatestCompletedItemByPath(ctx, itemPath)
}

// ListEligibleBackupItemsByPath returns backup snapshots for a file (including unchanged dedup skips), newest first.
func (r *JobItemRepository) ListEligibleBackupItemsByPath(ctx context.Context, itemPath string) ([]schema.JobItem, error) {
	var items []schema.JobItem
	err := r.DB.WithContext(ctx).
		Joins("JOIN jobs ON jobs.id = job_items.job_id").
		Where("job_items.item_path = ? AND jobs.status = ? AND jobs.flow_type = ? AND job_items.status IN ?",
			itemPath, "SUCCESS", string(schema.FlowBackup), backupSnapshotStatuses).
		Preload("Job").
		Order("job_items.created_at DESC").
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list backup job items: %w", err)
	}
	return items, nil
}

// CreateRestoreJobItem opens a restore-scoped JobItem cloned from an eligible backup snapshot.
func (r *JobItemRepository) CreateRestoreJobItem(ctx context.Context, restoreJobID string, source *schema.JobItem) (*schema.JobItem, error) {
	restoreItem := &schema.JobItem{
		JobID:        restoreJobID,
		ItemPath:     source.ItemPath,
		OriginalSize: source.OriginalSize,
		Status:       "PENDING",
		ItemMetadata: source.ItemMetadata,
	}
	if err := r.DB.WithContext(ctx).Create(restoreItem).Error; err != nil {
		return nil, fmt.Errorf("failed to create restore job item: %w", err)
	}
	return restoreItem, nil
}

func (r *JobItemRepository) GetJobItemByID(ctx context.Context, itemID string) (*schema.JobItem, error) {
	var item schema.JobItem
	err := r.DB.WithContext(ctx).First(&item, "id = ?", itemID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load job item: %w", err)
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

// UpdateSkippedBackupItemProof persists DR manifest location and skip proof on an unchanged backup run.
func (r *JobItemRepository) UpdateSkippedBackupItemProof(ctx context.Context, itemID string, metadata datatypes.JSON) error {
	err := r.DB.WithContext(ctx).Model(&schema.JobItem{}).
		Where("id = ? AND status = ?", itemID, "SKIPPED").
		Updates(map[string]interface{}{
			"item_metadata": metadata,
		}).Error
	if err != nil {
		return fmt.Errorf("failed to record skipped backup proof: %w", err)
	}
	return nil
}

func (r *JobItemRepository) CompleteJobItem(ctx context.Context, itemID string, contentChecksum string) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var chunks []struct {
			SizeBytes int64
		}

		err := tx.Table("item_chunks").
			Select("chunks.size_bytes").
			Joins("JOIN chunks ON chunks.id = item_chunks.chunk_id").
			Where("item_chunks.item_id = ?", itemID).
			Order("item_chunks.chunk_index ASC").
			Scan(&chunks).Error
		if err != nil {
			return fmt.Errorf("failed to fetch content chunk hashes: %w", err)
		}

		var totalProcessedSize int64 = 0
		for _, c := range chunks {
			totalProcessedSize += c.SizeBytes
		}

		err = tx.Model(&schema.JobItem{}).Where("id = ?", itemID).Updates(map[string]interface{}{
			"status":         "COMPLETED",
			"processed_size": totalProcessedSize,
			"checksum":       contentChecksum,
		}).Error
		if err != nil {
			return fmt.Errorf("failed to finalize job item record: %w", err)
		}

		return nil
	})
}
