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

	// 4. Decision Ladder (Pure Logic, No Writes)
	if historicalItem == nil {
		action = "PROCEED_TO_CHUNK"
		reason = "No previous backup history found for this file"

	} else {
		// Use GORM's native UnmarshalJSON method to parse the datatype cleanly
		var histMeta types.ItemMetadata
		if err := historicalItem.ItemMetadata.UnmarshalJSON([]byte(historicalItem.ItemMetadata)); err != nil {
			// Fallback option in case your GORM version prefers direct unmarshal on the target pointer address
			if err := json.Unmarshal([]byte(historicalItem.ItemMetadata), &histMeta); err != nil {
				return types.InspectFileResponse{}, fmt.Errorf("failed to unmarshal historical item metadata: %w", err)
			}
		} else {
			// If UnmarshalJSON ran on the instance, unmarshal the raw bytes directly into your struct target
			_ = json.Unmarshal([]byte(historicalItem.ItemMetadata), &histMeta)
		}

		// --- DIAGNOSTIC LOG (TEMPORARY) ---
		// Add this print statement to see if your struct properties are finally populated:
		fmt.Printf("[DEBUG] Parsed ModTime from DB: %s, Current ModTime: %s\n",
			histMeta.LastModifiedAt.UTC().Format(time.RFC3339),
			info.ModTime().UTC().Format(time.RFC3339),
		)
		// ----------------------------------

		// Standardize time formatting down to the second
		currentModTime := info.ModTime().UTC().Truncate(time.Second)
		histModTime := histMeta.LastModifiedAt.UTC().Truncate(time.Second)

		// Tier 1 & 2 Check
		if historicalItem.OriginalSize == currentSize && histModTime.Equal(currentModTime) {

			// Tier 3: Verify File Integrity with a Hash Check
			currentHash, err := calculateFileHash(input.ItemPath, currentSize)
			if err != nil {
				return types.InspectFileResponse{}, fmt.Errorf("failed to compute file hash: %w", err)
			}

			if historicalItem.Checksum == currentHash {
				action = "SKIP"
				reason = "File size, modification time, and cryptographic hash match perfectly; skipping backup"

				newJobItem.Status = "SKIPPED"
				newJobItem.ProcessedSize = historicalItem.ProcessedSize
				newJobItem.Checksum = historicalItem.Checksum
				newJobItem.ItemMetadata = historicalItem.ItemMetadata
				shouldCloneChunks = true
			} else {
				action = "PROCEED_TO_CHUNK"
				// Clear Indicator: Content inside changed without resizing the container array bounds
				reason = "File size and mod-time matched, but cryptographic content checksum validation failed (internal byte contents modified)"
			}
		} else {
			action = "PROCEED_TO_CHUNK"
			// Clear Indicator: Distinguish between volume alterations and timestamp updates
			if historicalItem.OriginalSize == currentSize {
				reason = fmt.Sprintf("File size matches (%d bytes), but modification time shifted. Old: %s, New: %s", 
					currentSize, histModTime.Format(time.RFC3339), currentModTime.Format(time.RFC3339))
			} else {
				reason = fmt.Sprintf("File content properties altered. Old Size: %d bytes, New Size: %d bytes", 
					historicalItem.OriginalSize, currentSize)
			}
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
func calculateFileHash(filePath string, currentSize int64) (string, error) {
	const chunkSize int64 = 4 * 1024 * 1024
	var offset int64 = 0
	h := sha256.New()

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	for offset < currentSize {
		length := chunkSize
		if offset+length > currentSize {
			length = currentSize - offset
		}

		buf := make([]byte, length)
		_, _ = io.ReadFull(file, buf)

		// Compute signature for this virtual block segment
		blockHasher := sha256.New()
		blockHasher.Write(buf)
		blockHash := hex.EncodeToString(blockHasher.Sum(nil))

		h.Write([]byte(blockHash))
		offset += chunkSize
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
