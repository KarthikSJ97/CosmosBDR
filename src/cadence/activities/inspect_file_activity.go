package activities

import (
	"context"
	"cosmosbdr/src/types"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"cosmosbdr/src/db/schema"

	"gorm.io/datatypes"
)

func (ja *JobActivities) InspectFileActivity(ctx context.Context, input types.InspectFileRequest) (types.InspectFileResponse, error) {
	if input.ItemPath == "" {
		return types.InspectFileResponse{}, fmt.Errorf("itemPath cannot be empty")
	}

	// 1. Fetch filesystem properties
	info, err := os.Stat(input.ItemPath)
	if err != nil {
		if os.IsNotExist(err) {
			return types.InspectFileResponse{}, fmt.Errorf("source file does not exist: %s", input.ItemPath)
		}
		return types.InspectFileResponse{}, fmt.Errorf("failed to stat file: %w", err)
	}
	if info.IsDir() {
		return types.InspectFileResponse{}, fmt.Errorf("path is a directory: %s", input.ItemPath)
	}

	currentSize := info.Size()
	currentModTime := info.ModTime().UTC().Truncate(time.Second)

	// 2. Fetch latest historical backup record
	historicalItem, err := ja.JobItemRepo.GetLatestCompletedItemByPath(ctx, input.ItemPath)
	if err != nil {
		return types.InspectFileResponse{}, err
	}

	// 3. Initialize the new JobItem model skeleton [4]
	newJobItem := &schema.JobItem{
		JobID:        input.JobID,
		ItemPath:     input.ItemPath,
		OriginalSize: currentSize,
		Status:       "PENDING", // [4]
	}

	var action string
	var reason string
	var shouldCloneChunks bool

	// 4. Enhanced Decision Pipeline
	if historicalItem == nil {
		action = "PROCEED_TO_CHUNK"
		reason = "No previous backup history found for this file"

	} else {
		// Parse existing metadata out of the historical record
		var histMeta types.ItemMetadata
		_ = json.Unmarshal(historicalItem.ItemMetadata, &histMeta)

		// Tier 1 & 2: Match Size AND Modification Time
		histModTime := histMeta.LastModifiedAt.UTC().Truncate(time.Second)
		if historicalItem.OriginalSize == currentSize && histModTime.Equal(currentModTime) {

			// Tier 3: Verify File Integrity with a Hash Check [4]
			currentHash, err := calculateFileHash(input.ItemPath)
			if err != nil {
				return types.InspectFileResponse{}, fmt.Errorf("failed to compute file hash: %w", err)
			}

			if historicalItem.Checksum == currentHash {
				action = "SKIP"
				reason = "File size, modification time, and cryptographic hash match perfectly; skipping backup"

				newJobItem.Status = "SKIPPED" // [4]
				newJobItem.ProcessedSize = historicalItem.ProcessedSize
				newJobItem.Checksum = historicalItem.Checksum
				newJobItem.ItemMetadata = historicalItem.ItemMetadata
				shouldCloneChunks = true
			} else {
				action = "PROCEED_TO_CHUNK"
				reason = "File size and mod-time matched, but cryptographic content checksum changed"
			}
		} else {
			action = "PROCEED_TO_CHUNK"
			reason = fmt.Sprintf("File properties altered. Old Size: %d, New Size: %d", historicalItem.OriginalSize, currentSize)
		}
	}

	// 5. Populate Metadata if compiling a new chunk sequence
	if !shouldCloneChunks {
		metaObj := types.ItemMetadata{
			LastModifiedAt: currentModTime,
			FingerprintAlg: "sha256",
		}
		metaBytes, _ := json.Marshal(metaObj)
		newJobItem.ItemMetadata = datatypes.JSON(metaBytes)
	}

	// 6. Persist to Database [4]
	if err := ja.JobItemRepo.CreateJobItem(newJobItem); err != nil {
		return types.InspectFileResponse{}, fmt.Errorf("failed to create job item: %w", err)
	}

	// 7. Clone reference vectors if skipped [4]
	if shouldCloneChunks {
		if err := ja.JobItemRepo.CloneChunksFromItem(ctx, historicalItem.ID, newJobItem.ID); err != nil {
			return types.InspectFileResponse{}, fmt.Errorf("failed to link historical chunks: %w", err)
		}
	}

	return types.InspectFileResponse{
		Action:  action,
		JobItem: newJobItem,
		Reason:  reason,
	}, nil
}

// Helper function to efficiently compute SHA-256 hash of a file
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
