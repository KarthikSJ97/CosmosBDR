package activities

import (
	"context"
	"cosmosbdr/src/db/schema"
	"fmt"
	"os"
)

type InspectFileRequest struct {
	JobID    string
	ItemPath string // Absolute path of the file on localhost
}

type InspectFileResponse struct {
	Action  string // "SKIP" or "PROCEED_TO_CHUNK"
	JobItem *schema.JobItem
	Reason  string
}

func (ja *JobActivities) InspectFileActivity(ctx context.Context, input InspectFileRequest) (InspectFileResponse, error) {
	if input.ItemPath == "" {
		return InspectFileResponse{}, fmt.Errorf("itemPath cannot be empty")
	}

	// 1. Check if source file exists on localhost
	info, err := os.Stat(input.ItemPath)
	if err != nil {
		if os.IsNotExist(err) {
			return InspectFileResponse{}, fmt.Errorf("source file does not exist at path: %s", input.ItemPath)
		}
		return InspectFileResponse{}, fmt.Errorf("failed to stat source file: %w", err)
	}

	if info.IsDir() {
		return InspectFileResponse{}, fmt.Errorf("path is a directory, expected a file for current E2E scope: %s", input.ItemPath)
	}

	currentSize := info.Size()

	// 2. Fetch historical records
	historicalItem, err := ja.JobItemRepo.GetLatestCompletedItemByPath(ctx, input.ItemPath)
	if err != nil {
		return InspectFileResponse{}, err
	}

	// 3. Initialize the new JobItem skeleton
	newJobItem := &schema.JobItem{
		JobID:        input.JobID,
		ItemPath:     input.ItemPath,
		OriginalSize: currentSize,
		Status:       "PENDING", // Default status for new/modified files
	}

	var action string
	var reason string
	var shouldCloneChunks bool

	// 4. Decision Ladder (Pure Logic, No Writes)
	if historicalItem == nil {
		// CASE 1: Completely New File
		action = "PROCEED_TO_CHUNK"
		reason = "No previous backup history found for this file"

	} else if historicalItem.OriginalSize == currentSize {
		// CASE 2: Unchanged File -> SKIPPED
		action = "SKIP"
		reason = "File size matches historical record; no changes detected"

		// Set properties inherited from history
		newJobItem.Status = "SKIPPED"
		newJobItem.ProcessedSize = historicalItem.ProcessedSize
		newJobItem.Checksum = historicalItem.Checksum
		newJobItem.ItemMetadata = historicalItem.ItemMetadata
		shouldCloneChunks = true

	} else {
		// CASE 3: Modified File -> PROCEED_TO_CHUNK
		action = "PROCEED_TO_CHUNK"
		reason = fmt.Sprintf("File size changed from %d to %d bytes", historicalItem.OriginalSize, currentSize)
	}

	// 5. Single Database Write (Unified execution)
	if err := ja.JobItemRepo.CreateJobItem(newJobItem); err != nil {
		return InspectFileResponse{}, fmt.Errorf("failed to create job item: %w", err)
	}

	// 6. Post-write operational side-effects (Optional chunk replication)
	if shouldCloneChunks {
		if err := ja.JobItemRepo.CloneChunksFromItem(ctx, historicalItem.ID, newJobItem.ID); err != nil {
			return InspectFileResponse{}, fmt.Errorf("failed to link historical chunks: %w", err)
		}
	}

	return InspectFileResponse{
		Action:  action,
		JobItem: newJobItem,
		Reason:  reason,
	}, nil
}
