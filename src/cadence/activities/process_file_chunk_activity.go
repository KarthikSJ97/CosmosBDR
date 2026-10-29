package activities

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (ja *JobActivities) ProcessFileChunkActivity(ctx context.Context, input types.ProcessFileChunkRequest) (types.ProcessFileChunkResponse, error) {
	// 1. Open the source file
	file, err := os.Open(input.ItemPath)
	if err != nil {
		return types.ProcessFileChunkResponse{}, fmt.Errorf("failed to open file for chunking: %w", err)
	}
	defer file.Close()

	// 2. Seek straight to the target segment offset boundary
	_, err = file.Seek(input.OffsetBytes, io.SeekStart)
	if err != nil {
		return types.ProcessFileChunkResponse{}, fmt.Errorf("failed to seek to chunk offset: %w", err)
	}

	// 3. Read exactly LengthBytes into a buffer segment
	buffer := make([]byte, input.LengthBytes)
	bytesRead, err := io.ReadFull(file, buffer)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return types.ProcessFileChunkResponse{}, fmt.Errorf("failed to read block segment: %w", err)
	}

	// Truncate buffer to actual read length if we hit the end of the file early
	buffer = buffer[:bytesRead]

	// 4. Content-Addressable Validation: Compute Block Signature Hash (SHA-256)
	hasher := sha256.New()
	hasher.Write(buffer)
	chunkHash := hex.EncodeToString(hasher.Sum(nil))

	// 5. Define the correct SeaweedFS storage key upfront
	seaweedKey := fmt.Sprintf("chunks/%s", chunkHash)

	// Initialize the potential new global Chunk model structure
	potentialChunk := &schema.Chunk{
		ChunkHash:           chunkHash,
		SeaweedKey:          seaweedKey,
		SizeBytes:           int64(bytesRead),
		CompressedSizeBytes: int64(bytesRead), // Update when adding compression middleware
	}

	// 6. DB Deduplication Check: Check or insert the master global row atomically
	resolvedChunk, createdNew, err := ja.ChunkRepo.FindOrCreateChunk(ctx, potentialChunk)
	if err != nil {
		return types.ProcessFileChunkResponse{}, fmt.Errorf("deduplication routine failure: %w", err)
	}

	// 7. Physical Storage Engine Upload (If the block is completely unique)
	if createdNew {
		if ja.Seaweed == nil || ja.Seaweed.Client == nil {
			return types.ProcessFileChunkResponse{}, fmt.Errorf("seaweed client is not initialized")
		}

		_, err := ja.Seaweed.Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(ja.Seaweed.Bucket),
			Key:    aws.String(seaweedKey),
			Body:   bytes.NewReader(buffer),
		})
		if err != nil {
			return types.ProcessFileChunkResponse{}, fmt.Errorf("failed to upload chunk to SeaweedFS: %w", err)
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

	return types.ProcessFileChunkResponse{
		ChunkID:     resolvedChunk.ID,
		ChunkIndex:  input.ChunkIndex,
		UploadedNew: createdNew,
	}, nil
}
