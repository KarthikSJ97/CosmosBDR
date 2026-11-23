package types

// BackupJobPayload is shared by backup and restore workflows (flow_type discriminates behavior).
type BackupJobPayload struct {
	FileName        string          `json:"file_name"`
	RelativePath    string          `json:"relative_path,omitempty"`
	SourceID        string          `json:"source_id,omitempty"`
	BackupKey       string          `json:"backup_key,omitempty"`
	RemoteIO        bool            `json:"remote_io,omitempty"`
	SourceJobItemID string          `json:"source_job_item_id,omitempty"` // restore: pin a backup version; empty = latest eligible backup
	Directory       *DirectoryScope `json:"directory,omitempty"`          // set → directory parent workflow
	ParentJobID     string          `json:"parent_job_id,omitempty"`      // child file backup of a directory job
	ScheduleJobID   string          `json:"schedule_job_id,omitempty"`    // sources.yaml schedules[].id when cron-fired
	PreBackup       bool            `json:"pre_backup,omitempty"`         // Phase H: SSH dump before inspect/chunks
}

type BackupWorkflowRequest struct {
	FlowType   string           `json:"flow_type"`
	SourcePath string           `json:"source_path"`
	TargetPath string           `json:"target_path"`
	ListRootPath string         `json:"-"` // directory backup: set by API resolver only
	Payload    BackupJobPayload `json:"payload"`
}

type BackupWorkflowResponse struct {
	JobID string `json:"job_id"`
}
