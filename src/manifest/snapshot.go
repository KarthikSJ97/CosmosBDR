package manifest

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	ObjectKeyManifest = "manifest.json"
	// ChunkSizeBytes is the fixed backup grid size (must match backup_workflow segmentation).
	ChunkSizeBytes = 4 * 1024 * 1024
)

// FileSnapshot is the point-in-time manifest stored under snapshots/<id>/manifest.json.
type FileSnapshot struct {
	JobItemID          string     `json:"job_item_id"`
	ItemPath           string     `json:"item_path"`
	BackedUpAt         time.Time  `json:"backed_up_at"`
	FileChecksumSHA256 string     `json:"file_checksum_sha256"`
	FileSizeBytes      int64      `json:"file_size_bytes"`
	ChunkSizeBytes     int64      `json:"chunk_size_bytes"`
	Chunks             []ChunkRef `json:"chunks"`
}

// Meta carries file-level snapshot metadata written alongside chunk pointers.
type Meta struct {
	JobItemID          string
	ItemPath           string
	BackedUpAt         time.Time
	FileChecksumSHA256 string
	FileSizeBytes      int64
	ChunkSizeBytes     int64
}

// ChunkRef maps a fixed grid slot to a content-addressed object under the file prefix.
type ChunkRef struct {
	Index  int    `json:"index"`
	Object string `json:"object"`
}

// ChunkInput is the minimal row needed to build a manifest entry.
type ChunkInput struct {
	Index     int
	ChunkHash string
}

// Build validates contiguous indices 0..n-1 and emits object paths chunks/<hash>.
func Build(rows []ChunkInput, meta Meta) (*FileSnapshot, error) {
	if meta.JobItemID == "" {
		return nil, fmt.Errorf("job_item_id is required")
	}
	if meta.FileChecksumSHA256 == "" {
		return nil, fmt.Errorf("file_checksum_sha256 is required")
	}
	if meta.ChunkSizeBytes <= 0 {
		return nil, fmt.Errorf("chunk_size_bytes must be positive")
	}
	meta.BackedUpAt = meta.BackedUpAt.UTC()

	out := &FileSnapshot{
		JobItemID:          meta.JobItemID,
		ItemPath:           meta.ItemPath,
		BackedUpAt:         meta.BackedUpAt,
		FileChecksumSHA256: meta.FileChecksumSHA256,
		FileSizeBytes:      meta.FileSizeBytes,
		ChunkSizeBytes:     meta.ChunkSizeBytes,
		Chunks:             make([]ChunkRef, 0, len(rows)),
	}
	for i, row := range rows {
		if row.ChunkHash == "" {
			return nil, fmt.Errorf("chunk hash missing for index %d", row.Index)
		}
		if row.Index != i {
			return nil, fmt.Errorf("non-contiguous chunk index: expected %d, got %d", i, row.Index)
		}
		out.Chunks = append(out.Chunks, ChunkRef{
			Index:  row.Index,
			Object: fmt.Sprintf("chunks/%s", row.ChunkHash),
		})
	}
	return out, nil
}

// MarshalJSON returns indented JSON for upload to object storage.
func (s *FileSnapshot) MarshalJSONBytes() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// SnapshotFolderName returns a unique snapshot directory name (UTC time + job item id).
func SnapshotFolderName(itemID string, backedUpAt time.Time) string {
	return fmt.Sprintf("%s__%s", backedUpAt.UTC().Format("2006-01-02T15-04-05Z"), itemID)
}

// SnapshotObjectKey is the full object key under the bucket (file prefix + snapshots + manifest).
func SnapshotObjectKey(filePrefix, folderName string) string {
	return fmt.Sprintf("%s/snapshots/%s/%s", filePrefix, folderName, ObjectKeyManifest)
}