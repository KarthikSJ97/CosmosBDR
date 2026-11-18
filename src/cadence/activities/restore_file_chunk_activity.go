package activities

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"cosmosbdr/src/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (ja *JobActivities) RestoreFileChunkActivity(ctx context.Context, input types.RestoreFileChunkRequest) (types.RestoreFileChunkResponse, error) {
	if input.DestinationPath == "" {
		return types.RestoreFileChunkResponse{}, fmt.Errorf("destination_path is required")
	}
	if input.SeaweedKey == "" {
		return types.RestoreFileChunkResponse{}, fmt.Errorf("seaweed_key is required")
	}
	if input.ChunkHash == "" {
		return types.RestoreFileChunkResponse{}, fmt.Errorf("chunk_hash is required")
	}
	if input.LengthBytes <= 0 {
		return types.RestoreFileChunkResponse{}, fmt.Errorf("length_bytes must be positive")
	}
	if ja.Seaweed == nil || ja.Seaweed.Client == nil {
		return types.RestoreFileChunkResponse{}, fmt.Errorf("seaweed client is not initialized")
	}

	out, err := ja.Seaweed.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(ja.Seaweed.Bucket),
		Key:    aws.String(input.SeaweedKey),
	})
	if err != nil {
		return types.RestoreFileChunkResponse{}, fmt.Errorf("failed to download chunk from SeaweedFS (index %d): %w", input.ChunkIndex, err)
	}
	defer out.Body.Close()

	buffer, err := io.ReadAll(out.Body)
	if err != nil {
		return types.RestoreFileChunkResponse{}, fmt.Errorf("failed to read chunk body (index %d): %w", input.ChunkIndex, err)
	}

	if len(buffer) != input.LengthBytes {
		return types.RestoreFileChunkResponse{}, fmt.Errorf(
			"chunk %d size mismatch: expected %d bytes, got %d",
			input.ChunkIndex, input.LengthBytes, len(buffer),
		)
	}

	hasher := sha256.New()
	hasher.Write(buffer)
	downloadedHash := hex.EncodeToString(hasher.Sum(nil))
	if downloadedHash != input.ChunkHash {
		return types.RestoreFileChunkResponse{}, fmt.Errorf(
			"chunk %d hash mismatch: expected %s, got %s",
			input.ChunkIndex, input.ChunkHash, downloadedHash,
		)
	}

	file, err := os.OpenFile(input.DestinationPath, os.O_RDWR, 0)
	if err != nil {
		return types.RestoreFileChunkResponse{}, fmt.Errorf("failed to open restore target for writing (index %d): %w", input.ChunkIndex, err)
	}
	defer file.Close()

	written, err := file.WriteAt(buffer, input.OffsetBytes)
	if err != nil {
		return types.RestoreFileChunkResponse{}, fmt.Errorf("failed to write chunk at offset %d (index %d): %w", input.OffsetBytes, input.ChunkIndex, err)
	}
	if written != len(buffer) {
		return types.RestoreFileChunkResponse{}, fmt.Errorf(
			"chunk %d short write at offset %d: wrote %d of %d bytes",
			input.ChunkIndex, input.OffsetBytes, written, len(buffer),
		)
	}

	return types.RestoreFileChunkResponse{
		ChunkIndex:   input.ChunkIndex,
		BytesWritten: written,
	}, nil
}