package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cosmosbdr/src/config"
	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/types"
)

// RetentionGCActivity expires snapshot metadata past per-source TTL and reclaims unreferenced chunk blobs.
func (ja *JobActivities) RetentionGCActivity(ctx context.Context, input types.RetentionGCRequest) (types.RetentionGCResponse, error) {
	if ja.Registry == nil {
		return types.RetentionGCResponse{}, fmt.Errorf("source registry is not configured")
	}
	asOf := input.AsOf
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}

	var resp types.RetentionGCResponse
	itemBatch := ja.RetentionGCItemBatchSize
	if itemBatch <= 0 {
		itemBatch = config.DefaultRetentionGCItemBatchSize
	}
	orphanBatch := ja.RetentionGCOrphanBatchSize
	if orphanBatch <= 0 {
		orphanBatch = config.DefaultRetentionGCOrphanBatchSize
	}

	for _, src := range ja.Registry.ListRetentionGCSources() {
		expireBefore := asOf.Add(-time.Duration(src.TTLDays) * 24 * time.Hour)
		for {
			items, err := ja.JobItemRepo.ListExpiredSnapshotItems(ctx, src.SourceID, expireBefore, itemBatch)
			if err != nil {
				return resp, err
			}
			if len(items) == 0 {
				break
			}
			for i := range items {
				n, m, c, err := ja.purgeExpiredSnapshot(ctx, &items[i])
				if err != nil {
					return resp, fmt.Errorf("purge snapshot %s (source %s): %w", items[i].ID, src.SourceID, err)
				}
				resp.ExpiredItemsDeleted += n
				resp.ManifestsDeleted += m
				resp.ChunksDeleted += c
			}
			if len(items) < itemBatch {
				break
			}
		}
	}

	for {
		orphans, err := ja.ChunkRepo.ListUnreferencedChunks(ctx, orphanBatch)
		if err != nil {
			return resp, err
		}
		if len(orphans) == 0 {
			break
		}
		for _, ch := range orphans {
			if err := ja.deleteChunkBlobAndRow(ctx, &ch); err != nil {
				return resp, fmt.Errorf("delete orphan chunk %s: %w", ch.ID, err)
			}
			resp.OrphanChunksDeleted++
			resp.ChunksDeleted++
		}
		if len(orphans) < orphanBatch {
			break
		}
	}

	return resp, nil
}

func (ja *JobActivities) purgeExpiredSnapshot(ctx context.Context, item *schema.JobItem) (itemsDeleted, manifestsDeleted, chunksDeleted int, err error) {
	chunkIDs, err := ja.JobItemRepo.ChunkIDsForItem(ctx, item.ID)
	if err != nil {
		return 0, 0, 0, err
	}

	if key := manifestKeyFromJobItem(item); key != "" {
		if ja.Seaweed != nil {
			if err := ja.Seaweed.DeleteObject(ctx, key); err != nil {
				return 0, 0, 0, err
			}
		}
		manifestsDeleted = 1
	}

	if err := ja.JobItemRepo.HardDeleteJobItem(ctx, item.ID); err != nil {
		return 0, manifestsDeleted, 0, err
	}
	itemsDeleted = 1

	for _, chunkID := range chunkIDs {
		deleted, err := ja.tryDeleteUnreferencedChunk(ctx, chunkID)
		if err != nil {
			return itemsDeleted, manifestsDeleted, chunksDeleted, err
		}
		if deleted {
			chunksDeleted++
		}
	}
	return itemsDeleted, manifestsDeleted, chunksDeleted, nil
}

func (ja *JobActivities) tryDeleteUnreferencedChunk(ctx context.Context, chunkID string) (bool, error) {
	count, err := ja.ChunkRepo.CountItemChunkReferences(ctx, chunkID)
	if err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	chunk, err := ja.ChunkRepo.GetChunkByID(ctx, chunkID)
	if err != nil {
		return false, err
	}
	if chunk == nil {
		return false, nil
	}
	if err := ja.deleteChunkBlobAndRow(ctx, chunk); err != nil {
		return false, err
	}
	return true, nil
}

func (ja *JobActivities) deleteChunkBlobAndRow(ctx context.Context, chunk *schema.Chunk) error {
	if chunk == nil {
		return nil
	}
	if ja.Seaweed != nil && strings.TrimSpace(chunk.SeaweedKey) != "" {
		if err := ja.Seaweed.DeleteObject(ctx, chunk.SeaweedKey); err != nil {
			return err
		}
	}
	return ja.ChunkRepo.DeleteChunkRow(ctx, chunk.ID)
}

func manifestKeyFromJobItem(item *schema.JobItem) string {
	if item == nil || len(item.ItemMetadata) == 0 {
		return ""
	}
	var meta types.ItemMetadata
	if err := json.Unmarshal(item.ItemMetadata, &meta); err != nil {
		return ""
	}
	return strings.TrimSpace(meta.ManifestKey)
}