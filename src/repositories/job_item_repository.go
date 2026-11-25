package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/types"

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

func (r *JobItemRepository) backupSnapshotQuery(ctx context.Context) *gorm.DB {
	return r.DB.WithContext(ctx).
		Joins("JOIN jobs ON jobs.id = job_items.job_id").
		Where("jobs.status = ? AND jobs.flow_type = ? AND job_items.status IN ?",
			"SUCCESS", string(schema.FlowBackup), backupSnapshotStatuses)
}

// GetLatestCompletedItemByBackupKey looks up the most recent completed job item for a logical backup_key.
func (r *JobItemRepository) GetLatestCompletedItemByBackupKey(ctx context.Context, backupKey string) (*schema.JobItem, error) {
	var item schema.JobItem
	err := r.backupSnapshotQuery(ctx).
		Where("job_items.backup_key = ?", backupKey).
		Order("job_items.created_at DESC").
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query historical job item by backup_key: %w", err)
	}
	return &item, nil
}

// ListEligibleBackupItemsByBackupKey returns backup snapshots for a backup_key, newest first.
func (r *JobItemRepository) ListEligibleBackupItemsByBackupKey(ctx context.Context, backupKey string) ([]schema.JobItem, error) {
	var items []schema.JobItem
	err := r.backupSnapshotQuery(ctx).
		Where("job_items.backup_key = ?", backupKey).
		Preload("Job").
		Order("job_items.created_at DESC").
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list backup job items by backup_key: %w", err)
	}
	return items, nil
}

// GetLatestEligibleBackupItemByBackupKey is an alias for restore/list use.
func (r *JobItemRepository) GetLatestEligibleBackupItemByBackupKey(ctx context.Context, backupKey string) (*schema.JobItem, error) {
	return r.GetLatestCompletedItemByBackupKey(ctx, backupKey)
}

// GetLatestCompletedItemByPath looks up the most recent completed job item for a given path across past jobs.
func (r *JobItemRepository) GetLatestCompletedItemByPath(ctx context.Context, itemPath string) (*schema.JobItem, error) {
	var item schema.JobItem
	err := r.backupSnapshotQuery(ctx).
		Where("job_items.item_path = ?", itemPath).
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
	err := r.backupSnapshotQuery(ctx).
		Where("job_items.id = ?", itemID).
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

// ListBackupSnapshotsByParentJobID returns backup job_items from child file jobs of a directory backup parent.
func (r *JobItemRepository) ListBackupSnapshotsByParentJobID(ctx context.Context, parentJobID string) ([]schema.JobItem, error) {
	if parentJobID == "" {
		return nil, fmt.Errorf("parent_job_id is required")
	}
	var items []schema.JobItem
	err := r.backupSnapshotQuery(ctx).
		Where("jobs.payload->>'parent_job_id' = ?", parentJobID).
		Order("job_items.backup_key ASC").
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list snapshots for directory backup %q: %w", parentJobID, err)
	}
	return items, nil
}

// ListLatestEligibleBackupsUnderBackupKeyPrefix returns the newest eligible snapshot per backup_key under prefix.
func (r *JobItemRepository) ListLatestEligibleBackupsUnderBackupKeyPrefix(ctx context.Context, backupKeyPrefix string) ([]schema.JobItem, error) {
	backupKeyPrefix = strings.TrimSpace(backupKeyPrefix)
	if backupKeyPrefix == "" {
		return nil, fmt.Errorf("backup_key prefix is required")
	}
	like := backupKeyPrefix
	if !strings.HasSuffix(like, "%") {
		like = like + "%"
	}
	var items []schema.JobItem
	err := r.DB.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (job_items.backup_key) job_items.*
		FROM job_items
		INNER JOIN jobs ON jobs.id = job_items.job_id
		WHERE jobs.status = ?
		  AND jobs.flow_type = ?
		  AND job_items.status IN ?
		  AND job_items.backup_key LIKE ?
		ORDER BY job_items.backup_key, job_items.created_at DESC
	`, "SUCCESS", string(schema.FlowBackup), backupSnapshotStatuses, like).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list latest backups under prefix %q: %w", backupKeyPrefix, err)
	}
	return items, nil
}

// ListEligibleBackupItemsByPath returns backup snapshots for a file (including unchanged dedup skips), newest first.
func (r *JobItemRepository) ListEligibleBackupItemsByPath(ctx context.Context, itemPath string) ([]schema.JobItem, error) {
	var items []schema.JobItem
	err := r.backupSnapshotQuery(ctx).
		Where("job_items.item_path = ?", itemPath).
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
		BackupKey:    source.BackupKey,
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

// MergeJobItemManifestMetadata adds manifest DR location (and WORM ids) onto a job item's metadata.
func (r *JobItemRepository) MergeJobItemManifestMetadata(ctx context.Context, itemID string, manifestKey, manifestVersionID string, manifestRetainUntil time.Time) error {
	item, err := r.GetJobItemByID(ctx, itemID)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("job item not found: %s", itemID)
	}
	var meta types.ItemMetadata
	if len(item.ItemMetadata) > 0 {
		if err := json.Unmarshal(item.ItemMetadata, &meta); err != nil {
			return fmt.Errorf("unmarshal item metadata: %w", err)
		}
	}
	meta.ManifestKey = manifestKey
	if manifestVersionID != "" {
		meta.ManifestVersionID = manifestVersionID
	}
	if !manifestRetainUntil.IsZero() {
		meta.ManifestRetainUntil = manifestRetainUntil.UTC()
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal item metadata: %w", err)
	}
	res := r.DB.WithContext(ctx).Model(&schema.JobItem{}).Where("id = ?", itemID).
		Update("item_metadata", datatypes.JSON(metaBytes))
	if res.Error != nil {
		return fmt.Errorf("update manifest metadata: %w", res.Error)
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

// FailIncompleteJobItems marks PENDING items as FAILED (partial uploads; chunk GC is Phase F).
func (r *JobItemRepository) FailIncompleteJobItems(ctx context.Context, jobID, itemID, reason string) error {
	if jobID == "" {
		return fmt.Errorf("job_id is required")
	}
	q := r.DB.WithContext(ctx).Model(&schema.JobItem{}).
		Where("job_id = ? AND status = ?", jobID, "PENDING")
	if itemID != "" {
		q = q.Where("id = ?", itemID)
	}
	meta := datatypes.JSON([]byte(`{}`))
	if reason != "" {
		metaObj := types.ItemMetadata{
			SkipReason: reason,
		}
		if b, err := json.Marshal(metaObj); err == nil {
			meta = datatypes.JSON(b)
		}
	}
	result := q.Updates(map[string]interface{}{
		"status":         "FAILED",
		"item_metadata": meta,
	})
	if result.Error != nil {
		return result.Error
	}
	return nil
}

// ListExpiredSnapshotItems returns COMPLETED/SKIPPED snapshots older than expireBefore for one source (backup_key prefix).
func (r *JobItemRepository) ListExpiredSnapshotItems(ctx context.Context, sourceID string, expireBefore time.Time, limit int) ([]schema.JobItem, error) {
	if limit <= 0 {
		limit = 200
	}
	prefix := sourceID + ":%"
	var items []schema.JobItem
	err := r.backupSnapshotQuery(ctx).
		Where("job_items.backup_key LIKE ?", prefix).
		Where("job_items.created_at < ?", expireBefore).
		Order("job_items.created_at ASC").
		Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list expired snapshots for source %q: %w", sourceID, err)
	}
	return items, nil
}

// ChunkIDsForItem returns distinct global chunk IDs linked to a job item (before hard delete).
func (r *JobItemRepository) ChunkIDsForItem(ctx context.Context, itemID string) ([]string, error) {
	var ids []string
	err := r.DB.WithContext(ctx).Model(&schema.ItemChunk{}).
		Distinct("chunk_id").
		Where("item_id = ?", itemID).
		Pluck("chunk_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("list chunk ids for item %s: %w", itemID, err)
	}
	return ids, nil
}

// HardDeleteJobItem permanently removes a job_items row (item_chunks cascade).
func (r *JobItemRepository) HardDeleteJobItem(ctx context.Context, itemID string) error {
	res := r.DB.WithContext(ctx).Delete(&schema.JobItem{}, "id = ?", itemID)
	if res.Error != nil {
		return fmt.Errorf("hard delete job item %s: %w", itemID, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("job item not found: %s", itemID)
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
