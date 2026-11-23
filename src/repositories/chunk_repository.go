package repositories

import (
	"context"
	"errors"
	"fmt"
	"cosmosbdr/src/db/schema"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChunkRepository struct {
	DB *gorm.DB
}

func NewChunkRepository(db *gorm.DB) *ChunkRepository {
	return &ChunkRepository{DB: db}
}

// Ensure the name matches EXACTLY: FindOrCreateChunk
func (r *ChunkRepository) FindOrCreateChunk(ctx context.Context, newChunk *schema.Chunk) (*schema.Chunk, bool, error) {
	var existingChunk schema.Chunk
	var createdNew bool

	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("chunk_hash = ?", newChunk.ChunkHash).
			First(&existingChunk).Error

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				newChunk.ReferenceCount = 1
				if createErr := tx.Create(newChunk).Error; createErr != nil {
					return fmt.Errorf("failed to create new chunk record: %w", createErr)
				}
				existingChunk = *newChunk
				createdNew = true
				return nil
			}
			return fmt.Errorf("failed to check existing chunk: %w", err)
		}

		existingChunk.ReferenceCount++
		if updateErr := tx.Save(&existingChunk).Error; updateErr != nil {
			return fmt.Errorf("failed to increment chunk reference count: %w", updateErr)
		}
		createdNew = false
		return nil
	})

	if err != nil {
		return nil, false, err
	}

	return &existingChunk, createdNew, nil
}

// Ensure the name matches EXACTLY: CreateItemChunkMapping
func (r *ChunkRepository) CreateItemChunkMapping(ctx context.Context, mapping *schema.ItemChunk) error {
	err := r.DB.WithContext(ctx).Create(mapping).Error
	if err != nil {
		return fmt.Errorf("failed to map item to chunk: %w", err)
	}
	return nil
}

// ItemChunkWithStorage is the manifest row used when reassembling a file from SeaweedFS.
type ItemChunkWithStorage struct {
	ChunkIndex  int
	OffsetBytes int64
	LengthBytes int
	ChunkHash   string
	SeaweedKey  string
}

// CountItemChunkReferences returns how many job items still reference a global chunk row.
func (r *ChunkRepository) CountItemChunkReferences(ctx context.Context, chunkID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).Model(&schema.ItemChunk{}).Where("chunk_id = ?", chunkID).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count item_chunks for chunk %s: %w", chunkID, err)
	}
	return count, nil
}

// GetChunkByID loads a chunk row by primary key.
func (r *ChunkRepository) GetChunkByID(ctx context.Context, chunkID string) (*schema.Chunk, error) {
	var chunk schema.Chunk
	err := r.DB.WithContext(ctx).First(&chunk, "id = ?", chunkID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load chunk %s: %w", chunkID, err)
	}
	return &chunk, nil
}

// DeleteChunkRow hard-deletes a chunks row (caller must ensure no item_chunks reference it).
func (r *ChunkRepository) DeleteChunkRow(ctx context.Context, chunkID string) error {
	res := r.DB.WithContext(ctx).Delete(&schema.Chunk{}, "id = ?", chunkID)
	if res.Error != nil {
		return fmt.Errorf("delete chunk row %s: %w", chunkID, res.Error)
	}
	return nil
}

// ListUnreferencedChunks returns global chunk rows with zero item_chunk links (batch).
func (r *ChunkRepository) ListUnreferencedChunks(ctx context.Context, limit int) ([]schema.Chunk, error) {
	if limit <= 0 {
		limit = 500
	}
	var chunks []schema.Chunk
	err := r.DB.WithContext(ctx).
		Table("chunks").
		Select("chunks.*").
		Joins(`LEFT JOIN item_chunks ON item_chunks.chunk_id = chunks.id`).
		Where("item_chunks.chunk_id IS NULL").
		Limit(limit).
		Find(&chunks).Error
	if err != nil {
		return nil, fmt.Errorf("list unreferenced chunks: %w", err)
	}
	return chunks, nil
}

func (r *ChunkRepository) ListItemChunksWithStorage(ctx context.Context, itemID string) ([]ItemChunkWithStorage, error) {
	var rows []ItemChunkWithStorage
	err := r.DB.WithContext(ctx).
		Table("item_chunks").
		Select("item_chunks.chunk_index, item_chunks.offset_bytes, item_chunks.length_bytes, chunks.chunk_hash, chunks.seaweed_key").
		Joins("JOIN chunks ON chunks.id = item_chunks.chunk_id").
		Where("item_chunks.item_id = ?", itemID).
		Order("item_chunks.chunk_index ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list item chunks: %w", err)
	}
	return rows, nil
}
