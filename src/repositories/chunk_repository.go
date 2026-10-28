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
