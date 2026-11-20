package types

import "time"

// AvailableBackup is one restorable snapshot of a file (maps to payload.source_job_item_id on restore).
type AvailableBackup struct {
	JobItemID     string    `json:"job_item_id"`
	JobID         string    `json:"job_id"`
	BackupKey     string    `json:"backup_key,omitempty"`
	ItemPath      string    `json:"item_path"`
	Status        string    `json:"status"` // COMPLETED or SKIPPED (unchanged dedup)
	Unchanged     bool      `json:"unchanged"`
	OriginalSize  int64     `json:"original_size"`
	ProcessedSize int64     `json:"processed_size"`
	Checksum      string    `json:"checksum"` // SHA-256 of full file bytes (shasum -a 256)
	BackedUpAt    time.Time `json:"backed_up_at"`
	JobCreatedAt  time.Time `json:"job_created_at"`
}

type ListAvailableBackupsResponse struct {
	ItemPath  string            `json:"item_path,omitempty"`
	BackupKey string            `json:"backup_key,omitempty"`
	Backups   []AvailableBackup `json:"backups"`
}

// SourceSummary is a non-secret view of a configured source.
type SourceSummary struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Domain         string `json:"domain,omitempty"`
	ContainerMount string `json:"container_mount,omitempty"`
	Host           string `json:"host,omitempty"`
	TTLDays        int    `json:"ttl_days,omitempty"`
}