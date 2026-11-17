package types

// BackupJobPayload is shared by backup and restore workflows (flow_type discriminates behavior).
type BackupJobPayload struct {
	FileName          string `json:"file_name"`
	RelativePath      string `json:"relative_path,omitempty"`
	SourceJobItemID   string `json:"source_job_item_id,omitempty"` // restore: pin a backup version; empty = latest eligible backup
}

type BackupWorkflowRequest struct {
	FlowType   string           `json:"flow_type"`
	SourcePath string           `json:"source_path"`
	TargetPath string           `json:"target_path"`
	Payload    BackupJobPayload `json:"payload"`
}

type BackupWorkflowResponse struct {
	JobID string `json:"job_id"`
}
