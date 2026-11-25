package activities

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmosbdr/src/types"

	"gorm.io/datatypes"
)

// FinalizeSkippedBackupItemActivity records that a backup job ran but file bytes were unchanged (dedup skip).
func (ja *JobActivities) FinalizeSkippedBackupItemActivity(ctx context.Context, input types.FinalizeSkippedBackupItemRequest) (types.FinalizeSkippedBackupItemResponse, error) {
	if input.ItemID == "" {
		return types.FinalizeSkippedBackupItemResponse{}, fmt.Errorf("item id is required")
	}
	if input.ManifestKey == "" {
		return types.FinalizeSkippedBackupItemResponse{}, fmt.Errorf("manifest_key is required")
	}

	item, err := ja.JobItemRepo.GetJobItemByID(ctx, input.ItemID)
	if err != nil {
		return types.FinalizeSkippedBackupItemResponse{}, err
	}
	if item == nil {
		return types.FinalizeSkippedBackupItemResponse{}, fmt.Errorf("job item not found: %s", input.ItemID)
	}

	var meta types.ItemMetadata
	if len(item.ItemMetadata) > 0 {
		if err := json.Unmarshal([]byte(item.ItemMetadata), &meta); err != nil {
			return types.FinalizeSkippedBackupItemResponse{}, fmt.Errorf("unmarshal item metadata: %w", err)
		}
	}

	meta.BackupOutcome = "unchanged"
	meta.SkipReason = input.SkipReason
	if input.ManifestKey != "" {
		meta.ManifestKey = input.ManifestKey
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return types.FinalizeSkippedBackupItemResponse{}, fmt.Errorf("marshal item metadata: %w", err)
	}

	err = ja.JobItemRepo.UpdateSkippedBackupItemProof(ctx, input.ItemID, datatypes.JSON(metaBytes))
	if err != nil {
		return types.FinalizeSkippedBackupItemResponse{}, err
	}

	return types.FinalizeSkippedBackupItemResponse{
		ItemID: input.ItemID,
		Status: "SKIPPED",
	}, nil
}