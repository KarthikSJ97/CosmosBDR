package activities

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"cosmosbdr/src/db/schema"
)

type ProcessFileChunkRequest struct {
	JobID       string `json:"job_id"`
	ItemID      string `json:"item_id"`
	ItemPath    string `json:"item_path"`
	ChunkIndex  int    `json:"chunk_index"`
	OffsetBytes int64  `json:"offset_bytes"`
	LengthBytes int    `json:"length_bytes"`
}

type ProcessFileChunkResponse struct {
	ChunkID     string `json:"chunk_id"`
	ChunkIndex  int    `json:"chunk_index"`
	UploadedNew bool   `json:"uploaded_new"`
}

func (ja *JobActivities) ProcessFileChunkActivity(ctx context.Context, input ProcessFileChunkRequest) (ProcessFileChunkResponse, error) {
	// 1. Open the source file
	file, err := os.Open(input.ItemPath)
	if err != nil {
		return ProcessFileChunkResponse{}, fmt.Errorf("failed to open file for chunking: %w", err)
	}
	defer file.Close()

	// 2. Seek straight to the target segment offset boundary
	_, err = file.Seek(input.OffsetBytes, io.SeekStart)
	if err != nil {
		return ProcessFileChunkResponse{}, fmt.Errorf("failed to seek to chunk offset: %w", err)
	}

	// 3. Read exactly LengthBytes into a buffer segment
	buffer := make([]byte, input.LengthBytes)
	bytesRead, err := io.ReadFull(file, buffer)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return ProcessFileChunkResponse{}, fmt.Errorf("failed to read block segment: %w", err)
	}

	// Truncate buffer to actual read length if we hit the end of the file early
	buffer = buffer[:bytesRead]

	// 4. Content-Addressable Validation: Compute Block Signature Hash (SHA-256)
	hasher := sha256.New()
	hasher.Write(buffer)
	chunkHash := hex.EncodeToString(hasher.Sum(nil))

	// 5. Initialize the potential new global Chunk model structure
	// Note: For now, we mock SeaweedKey with a unique string or the hash itself.
	// You will later replace "mock-key-"+chunkHash with the actual SeaweedFS client upload key result.
	potentialChunk := &schema.Chunk{
		ChunkHash:           chunkHash,
		SeaweedKey:          "seaweed://chunks/" + chunkHash,
		SizeBytes:           int64(bytesRead),
		CompressedSizeBytes: int64(bytesRead), // Update when adding compression middleware
	}

	// 6. DB Deduplication Check: Check or insert the master global row atomically
	resolvedChunk, createdNew, err := ja.ChunkRepo.FindOrCreateChunk(ctx, potentialChunk)
	if err != nil {
		return ProcessFileChunkResponse{}, fmt.Errorf("deduplication routine failure: %w", err)
	}

	// 7. Physical Storage Engine Upload (If the block is completely unique)
	if createdNew {
		// TODO: Call your SeaweedFS client upload wrapper here using 'buffer'
		// fmt.Printf("Uploading unique block to SeaweedFS: %s\n", resolvedChunk.ChunkHash)
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
		return ProcessFileChunkResponse{}, fmt.Errorf("failed to save chunk reference routing: %w", err)
	}

	return ProcessFileChunkResponse{
		ChunkID:     resolvedChunk.ID,
		ChunkIndex:  input.ChunkIndex,
		UploadedNew: createdNew,
	}, nil
}
