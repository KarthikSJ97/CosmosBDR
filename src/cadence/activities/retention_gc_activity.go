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
				n, m, c, skipped, err := ja.purgeExpiredSnapshot(ctx, &items[i], asOf)
				if err != nil {
					return resp, fmt.Errorf("purge snapshot %s (source %s): %w", items[i].ID, src.SourceID, err)
				}
				resp.ExpiredItemsDeleted += n
				resp.ManifestsDeleted += m
				resp.ChunksDeleted += c
				resp.BlobsSkippedObjectLock += skipped
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
			deleted, skipped, err := ja.deleteChunkBlobAndRow(ctx, &ch, asOf)
			if err != nil {
				return resp, fmt.Errorf("delete orphan chunk %s: %w", ch.ID, err)
			}
			if skipped {
				resp.BlobsSkippedObjectLock++
				continue
			}
			if deleted {
				resp.OrphanChunksDeleted++
				resp.ChunksDeleted++
			}
		}
		if len(orphans) < orphanBatch {
			break
		}
	}

	return resp, nil
}

func (ja *JobActivities) purgeExpiredSnapshot(ctx context.Context, item *schema.JobItem, asOf time.Time) (itemsDeleted, manifestsDeleted, chunksDeleted, skipped int, err error) {
	manifestMeta := manifestMetaFromJobItem(item)
	if retainUntilActive(manifestMeta.RetainUntil, asOf) {
		return 0, 0, 0, 1, nil
	}

	chunkIDs, err := ja.JobItemRepo.ChunkIDsForItem(ctx, item.ID)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	if manifestMeta.Key != "" {
		deleted, blocked, err := ja.deleteSeaweedObject(ctx, manifestMeta.Key, manifestMeta.VersionID, manifestMeta.RetainUntil, asOf)
		if err != nil {
			return 0, 0, 0, 0, err
		}
		if blocked || !deleted {
			return 0, 0, 0, 1, nil
		}
		manifestsDeleted = 1
	}

	if err := ja.JobItemRepo.HardDeleteJobItem(ctx, item.ID); err != nil {
		return 0, manifestsDeleted, 0, 0, err
	}
	itemsDeleted = 1

	for _, chunkID := range chunkIDs {
		del, skip, err := ja.tryDeleteUnreferencedChunk(ctx, chunkID, asOf)
		if err != nil {
			return itemsDeleted, manifestsDeleted, chunksDeleted, skipped, err
		}
		if skip {
			skipped++
		}
		if del {
			chunksDeleted++
		}
	}
	return itemsDeleted, manifestsDeleted, chunksDeleted, skipped, nil
}

func (ja *JobActivities) tryDeleteUnreferencedChunk(ctx context.Context, chunkID string, asOf time.Time) (deleted bool, skipped bool, err error) {
	count, err := ja.ChunkRepo.CountItemChunkReferences(ctx, chunkID)
	if err != nil {
		return false, false, err
	}
	if count > 0 {
		return false, false, nil
	}
	chunk, err := ja.ChunkRepo.GetChunkByID(ctx, chunkID)
	if err != nil {
		return false, false, err
	}
	if chunk == nil {
		return false, false, nil
	}
	return ja.deleteChunkBlobAndRow(ctx, chunk, asOf)
}

func (ja *JobActivities) deleteChunkBlobAndRow(ctx context.Context, chunk *schema.Chunk, asOf time.Time) (deleted bool, skipped bool, err error) {
	if chunk == nil {
		return false, false, nil
	}
	if retainUntilActive(chunkRetainUntil(chunk), asOf) {
		return false, true, nil
	}
	if ja.Seaweed != nil && strings.TrimSpace(chunk.SeaweedKey) != "" {
		del, blocked, err := ja.deleteSeaweedObject(ctx, chunk.SeaweedKey, chunk.SeaweedVersionID, chunkRetainUntil(chunk), asOf)
		if err != nil {
			return false, false, err
		}
		if blocked {
			return false, true, nil
		}
		if !del {
			return false, true, nil
		}
	}
	if err := ja.ChunkRepo.DeleteChunkRow(ctx, chunk.ID); err != nil {
		return false, false, err
	}
	return true, false, nil
}

func (ja *JobActivities) deleteSeaweedObject(ctx context.Context, key, versionID string, retainUntil time.Time, asOf time.Time) (deleted bool, blocked bool, err error) {
	if ja.Seaweed == nil {
		return true, false, nil
	}
	if retainUntilActive(retainUntil, asOf) {
		return false, false, nil
	}
	outcome, err := ja.Seaweed.DeleteObjectVersion(ctx, key, versionID)
	if err != nil {
		return false, false, err
	}
	if outcome.Blocked {
		return false, true, nil
	}
	return outcome.Deleted, false, nil
}

func retainUntilActive(retainUntil, asOf time.Time) bool {
	return !retainUntil.IsZero() && asOf.Before(retainUntil.UTC())
}

func chunkRetainUntil(chunk *schema.Chunk) time.Time {
	if chunk == nil || chunk.RetainUntil == nil {
		return time.Time{}
	}
	return chunk.RetainUntil.UTC()
}

type manifestObjectMeta struct {
	Key         string
	VersionID   string
	RetainUntil time.Time
}

func manifestMetaFromJobItem(item *schema.JobItem) manifestObjectMeta {
	if item == nil || len(item.ItemMetadata) == 0 {
		return manifestObjectMeta{}
	}
	var meta types.ItemMetadata
	if err := json.Unmarshal(item.ItemMetadata, &meta); err != nil {
		return manifestObjectMeta{}
	}
	return manifestObjectMeta{
		Key:         strings.TrimSpace(meta.ManifestKey),
		VersionID:   strings.TrimSpace(meta.ManifestVersionID),
		RetainUntil: meta.ManifestRetainUntil.UTC(),
	}
}

func manifestKeyFromJobItem(item *schema.JobItem) string {
	return manifestMetaFromJobItem(item).Key
}