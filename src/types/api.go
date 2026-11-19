package types

import "time"

// AvailableBackup is one restorable snapshot of a file (maps to payload.source_job_item_id on restore).
type AvailableBackup struct {
	JobItemID     string    `json:"job_item_id"`
	JobID         string    `json:"job_id"`
	ItemPath      string    `json:"item_path"`
	OriginalSize  int64     `json:"original_size"`
	ProcessedSize int64     `json:"processed_size"`
	Checksum      string    `json:"checksum"` // SHA-256 of full file bytes (shasum -a 256)
	BackedUpAt    time.Time `json:"backed_up_at"`
	JobCreatedAt  time.Time `json:"job_created_at"`
}

type ListAvailableBackupsResponse struct {
	ItemPath string            `json:"item_path"`
	Backups  []AvailableBackup `json:"backups"`
}