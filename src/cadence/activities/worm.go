package activities

import (
	"context"
	"time"

	"cosmosbdr/src/config"
	"cosmosbdr/src/db/schema"
)

func (ja *JobActivities) sourceObjectLockEnabled(sourceID string) bool {
	if ja == nil || !ja.SeaweedObjectLockEnabled || ja.Registry == nil {
		return false
	}
	src, ok := ja.Registry.GetSource(sourceID)
	if !ok {
		return false
	}
	return config.SourceObjectLockEnabled(ja.SeaweedObjectLockEnabled, src.Retention)
}

func (ja *JobActivities) retainUntilForSource(sourceID string, snapshotTime time.Time) time.Time {
	if ja == nil || ja.Registry == nil {
		return time.Time{}
	}
	return ja.Registry.RetainUntilForSource(sourceID, ja.SeaweedObjectLockEnabled, snapshotTime)
}

func (ja *JobActivities) ensureChunkRetention(ctx context.Context, chunk *schema.Chunk, retainUntil time.Time) error {
	if chunk == nil || retainUntil.IsZero() || ja.Seaweed == nil {
		return nil
	}
	if chunk.RetainUntil != nil && !retainUntil.After(*chunk.RetainUntil) {
		return nil
	}
	if chunk.SeaweedVersionID == "" || chunk.SeaweedKey == "" {
		return nil
	}
	if err := ja.Seaweed.ExtendObjectRetention(ctx, chunk.SeaweedKey, chunk.SeaweedVersionID, retainUntil); err != nil {
		return err
	}
	return ja.ChunkRepo.UpdateChunkRetainUntil(ctx, chunk.ID, retainUntil, chunk.SeaweedVersionID)
}