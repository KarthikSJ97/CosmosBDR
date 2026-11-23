package activities

import (
	"context"
	"cosmosbdr/src/config"
	"cosmosbdr/src/types"
	"encoding/json"
	"fmt"
	"time"

	"cosmosbdr/src/db/schema"

	"gorm.io/datatypes"
)

func (ja *JobActivities) InspectFileActivity(ctx context.Context, input types.InspectFileRequest) (types.InspectFileResponse, error) {
	if input.ItemPath == "" {
		return types.InspectFileResponse{}, fmt.Errorf("itemPath cannot be empty")
	}

	reader, err := ja.openSourceReader(ctx, input.SourceID, input.ItemPath)
	if err != nil {
		return types.InspectFileResponse{}, fmt.Errorf("open source: %w", err)
	}
	defer reader.Close()

	stat, err := reader.Stat(ctx)
	if err != nil {
		return types.InspectFileResponse{}, fmt.Errorf("failed to stat file: %w", err)
	}
	if stat.IsDir {
		return types.InspectFileResponse{}, fmt.Errorf("path is a directory: %s", input.ItemPath)
	}

	currentSize := stat.Size
	currentModTime := stat.ModTime.UTC().Truncate(time.Second)

	if ja.Registry != nil && input.SourceID != "" {
		if src, ok := ja.Registry.GetSource(input.SourceID); ok {
			rel := config.RelativePathForInspect(input.SourceID, input.BackupKey, input.ItemPath, ja.Registry)
			if maxAge, match := src.DumpFreshnessMaxAge(rel); match {
				if err := config.CheckDumpArtifactFreshness(maxAge, currentModTime, rel); err != nil {
					return types.InspectFileResponse{}, err
				}
			}
		}
	}

	// 2. Fetch latest historical backup record
	var historicalItem *schema.JobItem
	if input.BackupKey != "" {
		historicalItem, err = ja.JobItemRepo.GetLatestCompletedItemByBackupKey(ctx, input.BackupKey)
	} else {
		historicalItem, err = ja.JobItemRepo.GetLatestCompletedItemByPath(ctx, input.ItemPath)
	}
	if err != nil {
		return types.InspectFileResponse{}, err
	}

	// 3. Initialize the new JobItem model skeleton [4]
	newJobItem := &schema.JobItem{
		JobID:        input.JobID,
		ItemPath:     input.ItemPath,
		BackupKey:    input.BackupKey,
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

		histModTime := histMeta.LastModifiedAt.UTC().Truncate(time.Second)

		// Tier 1 & 2 Check
		if historicalItem.OriginalSize == currentSize && histModTime.Equal(currentModTime) {

			// Tier 3: Verify File Integrity with a Hash Check
			currentHash, err := reader.ContentSHA256(ctx)
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
