package types

// DirectoryScope configures directory backup selection (see docs/architecture.md).
type DirectoryScope struct {
	Selection    string        `json:"selection" yaml:"selection"` // "all" | "matched"
	Include      []PathPattern `json:"include,omitempty" yaml:"include,omitempty"`
	Exclude      []PathPattern `json:"exclude,omitempty" yaml:"exclude,omitempty"`
	MaxDepth     *int          `json:"max_depth" yaml:"max_depth"` // nil = unlimited
	MinFileBytes *int64        `json:"min_file_bytes,omitempty" yaml:"min_file_bytes,omitempty"`
	MaxFileBytes *int64        `json:"max_file_bytes,omitempty" yaml:"max_file_bytes,omitempty"`
}

// PathPattern is one glob or regex rule against paths relative to the directory root.
type PathPattern struct {
	Type    string `json:"type" yaml:"type"` // "glob" | "regex"
	Pattern string `json:"pattern" yaml:"pattern"`
}

// ListedBackupFile is one file selected for backup under a directory job.
type ListedBackupFile struct {
	RelativeToSource string `json:"relative_to_source"`
	RelativeToDir    string `json:"relative_to_dir"`
	SizeBytes        int64  `json:"size_bytes"`
}

type ListDirectoryRequest struct {
	SourceID      string          `json:"source_id"`
	DirRelative   string          `json:"dir_relative"`
	Scope         DirectoryScope  `json:"scope"`
	ListRootPath  string          `json:"list_root_path"`
	SourcePrefix  string          `json:"source_prefix"`
	RemoteIO      bool            `json:"remote_io"`
}

type ListDirectoryResponse struct {
	Files []ListedBackupFile `json:"files"`
}

type DirectoryBackupWorkflowResponse struct {
	JobID          string `json:"job_id"`
	FilesTotal     int    `json:"files_total"`
	FilesSucceeded int    `json:"files_succeeded"`
	FilesFailed    int    `json:"files_failed"`
}

// DirectoryRestoreTarget is one file to restore under a directory restore job.
type DirectoryRestoreTarget struct {
	RelativePath    string `json:"relative_path"`
	BackupKey       string `json:"backup_key"`
	SourceJobItemID string `json:"source_job_item_id"`
	SizeBytes       int64  `json:"size_bytes"`
}

type ListDirectoryRestoreRequest struct {
	SourceID             string          `json:"source_id"`
	DirRelative          string          `json:"dir_relative"`
	SourceDirectoryJobID string          `json:"source_directory_job_id,omitempty"`
	Scope                *DirectoryScope `json:"scope,omitempty"`
}

type ListDirectoryRestoreResponse struct {
	Files []DirectoryRestoreTarget `json:"files"`
}

type DirectoryRestoreWorkflowResponse struct {
	JobID          string `json:"job_id"`
	FilesTotal     int    `json:"files_total"`
	FilesSucceeded int    `json:"files_succeeded"`
	FilesFailed    int    `json:"files_failed"`
}