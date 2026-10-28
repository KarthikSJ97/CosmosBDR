package types

import (
	"cosmosbdr/src/db/schema"
	"time"
)

// ItemMetadata defines structural values for GORM jsonb parameters
type ItemMetadata struct {
	LastModifiedAt time.Time `json:"last_modified_at"`
	FingerprintAlg string    `json:"fingerprint_alg"` // e.g., "sha256"
}

type InitJobRequest struct {
	FlowType   string           `json:"flow_type"`
	SourcePath string           `json:"source_path"`
	TargetPath string           `json:"target_path"`
	Payload    BackupJobPayload `json:"payload"`
}

type InitJobResponse struct {
	Job *schema.Job `json:"job"`
}

type UpdateJobStatusRequest struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

type InspectFileRequest struct {
	JobID    string `json:"job_id"`
	ItemPath string `json:"item_path"`
}

type InspectFileResponse struct {
	Action  string          `json:"action"` // "SKIP" or "PROCEED_TO_CHUNK"
	JobItem *schema.JobItem `json:"job_item"`
	Reason  string          `json:"reason"`
}

type ProcessFileChunkRequest struct {
	JobID       string `json:"job_id"`
	ItemID      string `json:"item_id"`
	ItemPath    string `json:"item_path"`
	ChunkIndex  int    `json:"chunk_index"`
	OffsetBytes int64  `json:"offset_bytes"`
	LengthBytes int    `json:"length_bytes"`
}

type ProcessFileChunkResponse struct {
	ChunkID     string `json:"chunk_id"`
	ChunkIndex  int    `json:"chunk_index"`
	UploadedNew bool   `json:"uploaded_new"`
}

type CompleteJobItemRequest struct {
	JobID  string `json:"job_id"`
	ItemID string `json:"item_id"`
}

type CompleteJobItemResponse struct {
	ItemID string `json:"item_id"`
	Status string `json:"status"`
}

type FinalizeJobRequest struct {
	JobID        string  `json:"job_id"`
	Status       string  `json:"status"`        // "SUCCESS" or "FAILED"
	ErrorMessage *string `json:"error_message"` // Populated if status is FAILED
}
