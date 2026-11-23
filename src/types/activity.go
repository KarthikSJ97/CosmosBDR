package types

import (
	"cosmosbdr/src/db/schema"
	"time"
)

// ItemMetadata defines structural values for GORM jsonb parameters
type ItemMetadata struct {
	LastModifiedAt time.Time `json:"last_modified_at"`
	FingerprintAlg string    `json:"fingerprint_alg"` // e.g., "sha256"
	BackupOutcome  string    `json:"backup_outcome,omitempty"` // e.g. "unchanged"
	SkipReason     string    `json:"skip_reason,omitempty"`
	ManifestKey    string    `json:"manifest_key,omitempty"`
}

// CadenceRuntimeLimits is resolved on the worker from server config (not API input).
type CadenceRuntimeLimits struct {
	MaxConcurrentWorkflows       int `json:"max_concurrent_workflows"`
	WorkerMaxConcurrentActivities int `json:"worker_max_concurrent_activities"`
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
	JobID     string `json:"job_id"`
	ItemPath  string `json:"item_path"`
	SourceID  string `json:"source_id,omitempty"`
	BackupKey string `json:"backup_key,omitempty"`
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
	SourceID    string `json:"source_id,omitempty"`
	FileName    string `json:"file_name"` // relative path used for SeaweedFS object prefix
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

type FinalizeSkippedBackupItemRequest struct {
	ItemID       string `json:"item_id"`
	SkipReason   string `json:"skip_reason"`
	ManifestKey  string `json:"manifest_key"`
}

type FinalizeSkippedBackupItemResponse struct {
	ItemID string `json:"item_id"`
	Status string `json:"status"`
}

type PublishSnapshotManifestRequest struct {
	ItemID   string `json:"item_id"`
	FileName string `json:"file_name"`
}

type PublishSnapshotManifestResponse struct {
	ItemID      string `json:"item_id"`
	ManifestKey string `json:"manifest_key"`
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

// FailJobExecutionRequest marks incomplete job items FAILED before job finalization.
type FailJobExecutionRequest struct {
	JobID     string `json:"job_id"`
	JobItemID string `json:"job_item_id,omitempty"` // empty = all PENDING items on job
	Reason    string `json:"reason"`
}

// RestoreChunkPlan describes one ordered segment needed to assemble the file on disk.
type RestoreChunkPlan struct {
	ChunkIndex     int    `json:"chunk_index"`
	OffsetBytes    int64  `json:"offset_bytes"`
	LengthBytes    int    `json:"length_bytes"`
	ChunkHash      string `json:"chunk_hash"`
	SeaweedKey     string `json:"seaweed_key"`
	CompressionAlg string `json:"compression_alg"`
	Encrypted      bool   `json:"encrypted"`
}

type ResolveRestoreSourceRequest struct {
	JobID           string `json:"job_id"`
	ItemPath        string `json:"item_path"`
	BackupKey       string `json:"backup_key,omitempty"`
	SourceJobItemID string `json:"source_job_item_id,omitempty"`
	TargetPath      string `json:"target_path,omitempty"` // optional local override on CosmosBDR host
}

type ResolveRestoreSourceResponse struct {
	DestinationPath string          `json:"destination_path"`
	SourceID        string          `json:"source_id,omitempty"`
	RemoteIO        bool            `json:"remote_io"`
	SourceJobItemID string          `json:"source_job_item_id"`
	RestoreJobItem  *schema.JobItem `json:"restore_job_item"`
	OriginalSize    int64           `json:"original_size"`
	Chunks          []RestoreChunkPlan `json:"chunks"`
}

type PrepareRestoreTargetRequest struct {
	DestinationPath string `json:"destination_path"`
	SourceID        string `json:"source_id,omitempty"`
	RemoteIO        bool   `json:"remote_io"`
	OriginalSize    int64  `json:"original_size"`
}

type PrepareRestoreTargetResponse struct {
	DestinationPath string `json:"destination_path"`
	OriginalSize    int64  `json:"original_size"`
}

type RestoreFileChunkRequest struct {
	DestinationPath string `json:"destination_path"`
	SourceID        string `json:"source_id,omitempty"`
	RemoteIO        bool   `json:"remote_io"`
	ChunkIndex      int    `json:"chunk_index"`
	OffsetBytes     int64  `json:"offset_bytes"`
	LengthBytes     int    `json:"length_bytes"`
	ChunkHash        string `json:"chunk_hash"`
	SeaweedKey       string `json:"seaweed_key"`
	CompressionAlg   string `json:"compression_alg"`
	Encrypted        bool   `json:"encrypted"`
}

type RestoreFileChunkResponse struct {
	ChunkIndex   int `json:"chunk_index"`
	BytesWritten int `json:"bytes_written"`
}

// RetentionGCRequest drives one retention pass (AsOf is wall clock for TTL; zero means time.Now()).
type RetentionGCRequest struct {
	AsOf time.Time `json:"as_of,omitempty"`
}

// RetentionGCResponse summarizes one retention GC activity run.
type RetentionGCResponse struct {
	ExpiredItemsDeleted  int `json:"expired_items_deleted"`
	ManifestsDeleted     int `json:"manifests_deleted"`
	ChunksDeleted        int `json:"chunks_deleted"`
	OrphanChunksDeleted  int `json:"orphan_chunks_deleted"`
}
