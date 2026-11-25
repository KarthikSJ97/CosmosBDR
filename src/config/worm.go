package config

import "time"

// RetainUntilForSnapshot returns snapshotTime + source TTL when object lock applies; zero time otherwise.
func RetainUntilForSnapshot(objectLockEnabled bool, ttlDays int, snapshotTime time.Time) time.Time {
	if !objectLockEnabled || ttlDays <= 0 {
		return time.Time{}
	}
	return snapshotTime.UTC().Add(time.Duration(ttlDays) * 24 * time.Hour)
}

// SourceObjectLockEnabled is true when the global flag is on and this source has ttl_days > 0.
func SourceObjectLockEnabled(globalEnabled bool, retention SourceRetention) bool {
	return globalEnabled && retention.GCEnabled()
}

// RetainUntilForSource resolves retain-until for a backup snapshot on the given source.
func (r *SourceRegistry) RetainUntilForSource(sourceID string, globalObjectLock bool, snapshotTime time.Time) time.Time {
	if r == nil {
		return time.Time{}
	}
	src, ok := r.GetSource(sourceID)
	if !ok {
		return time.Time{}
	}
	if !SourceObjectLockEnabled(globalObjectLock, src.Retention) {
		return time.Time{}
	}
	return RetainUntilForSnapshot(true, src.Retention.TTLDays, snapshotTime)
}