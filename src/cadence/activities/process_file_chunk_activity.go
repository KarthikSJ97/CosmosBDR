package activities

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	"cosmosbdr/src/config"
	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/sourceread"
	"cosmosbdr/src/types"
)

func (ja *JobActivities) ProcessFileChunkActivity(ctx context.Context, input types.ProcessFileChunkRequest) (types.ProcessFileChunkResponse, error) {
	reader, err := ja.openSourceReader(ctx, input.SourceID, input.ItemPath)
	if err != nil {
		return types.ProcessFileChunkResponse{}, fmt.Errorf("open source: %w", err)
	}
	defer reader.Close()

	buffer, err := sourceread.ReadFullSegment(ctx, reader, input.OffsetBytes, input.LengthBytes)
	if err != nil {
		return types.ProcessFileChunkResponse{}, fmt.Errorf("failed to read block segment: %w", err)
	}
	bytesRead := len(buffer)

	// 4. Content-Addressable Validation: Compute Block Signature Hash (SHA-256)
	hasher := sha256.New()
	hasher.Write(buffer)
	chunkHash := hex.EncodeToString(hasher.Sum(nil))

	// 5. Object key: {source_id}/{relative_path}/chunks/{chunk_hash} (bucket is configured on the client)
	filePrefix := config.FormatSeaweedObjectPrefix(input.SourceID, input.FileName)
	if filePrefix == "" {
		filePrefix = filepath.Base(input.ItemPath)
	}
	seaweedKey := fmt.Sprintf("%s/chunks/%s", filePrefix, chunkHash)

	if ja.ChunkCodec == nil {
		return types.ProcessFileChunkResponse{}, fmt.Errorf("chunk codec is not configured")
	}

	storedBlob, err := ja.ChunkCodec.Encode(buffer)
	if err != nil {
		return types.ProcessFileChunkResponse{}, fmt.Errorf("compress/encrypt chunk: %w", err)
	}

	// Initialize the potential new global Chunk model structure
	potentialChunk := &schema.Chunk{
		ChunkHash:           chunkHash,
		SeaweedKey:          seaweedKey,
		SizeBytes:           int64(bytesRead),
		CompressedSizeBytes: int64(len(storedBlob)),
		CompressionAlg:      ja.ChunkCodec.CompressionAlgorithm(),
		Encrypted:           true,
	}

	// 6. DB Deduplication Check: Check or insert the master global row atomically
	resolvedChunk, createdNew, err := ja.ChunkRepo.FindOrCreateChunk(ctx, potentialChunk)
	if err != nil {
		return types.ProcessFileChunkResponse{}, fmt.Errorf("deduplication routine failure: %w", err)
	}

	// 7. Physical Storage Engine Upload (If the block is completely unique)
	snapshotAt := time.Now().UTC()
	retainUntil := ja.retainUntilForSource(input.SourceID, snapshotAt)
	if createdNew {
		if ja.Seaweed == nil || ja.Seaweed.Client == nil {
			return types.ProcessFileChunkResponse{}, fmt.Errorf("seaweed client is not initialized")
		}
		if ja.sourceObjectLockEnabled(input.SourceID) {
			putRes, err := ja.Seaweed.PutObjectLocked(ctx, seaweedKey, storedBlob, "", retainUntil)
			if err != nil {
				return types.ProcessFileChunkResponse{}, fmt.Errorf("failed to upload chunk to SeaweedFS: %w", err)
			}
			if err := ja.ChunkRepo.UpdateChunkRetainUntil(ctx, resolvedChunk.ID, putRes.RetainUntil, putRes.VersionID); err != nil {
				return types.ProcessFileChunkResponse{}, err
			}
			resolvedChunk.SeaweedVersionID = putRes.VersionID
			resolvedChunk.RetainUntil = &putRes.RetainUntil
		} else {
			if err := ja.Seaweed.PutObjectPlain(ctx, seaweedKey, bytes.NewReader(storedBlob), ""); err != nil {
				return types.ProcessFileChunkResponse{}, fmt.Errorf("failed to upload chunk to SeaweedFS: %w", err)
			}
		}
	}

	// 8. Persist the Intermediary Structural Mapping Reference Matrix Link
	mapping := &schema.ItemChunk{
		ItemID:      input.ItemID,
		ChunkID:     resolvedChunk.ID,
		ChunkIndex:  input.ChunkIndex,
		OffsetBytes: input.OffsetBytes,
		LengthBytes: bytesRead,
	}

	if err := ja.ChunkRepo.CreateItemChunkMapping(ctx, mapping); err != nil {
		return types.ProcessFileChunkResponse{}, fmt.Errorf("failed to save chunk reference routing: %w", err)
	}

	if !createdNew && ja.sourceObjectLockEnabled(input.SourceID) && !retainUntil.IsZero() {
		if err := ja.ensureChunkRetention(ctx, resolvedChunk, retainUntil); err != nil {
			return types.ProcessFileChunkResponse{}, fmt.Errorf("extend chunk object lock: %w", err)
		}
	}

	return types.ProcessFileChunkResponse{
		ChunkID:     resolvedChunk.ID,
		ChunkIndex:  input.ChunkIndex,
		UploadedNew: createdNew,
	}, nil
}
