package types

// BackupJobPayload defines the specific file details inside the payload
type BackupJobPayload struct {
	FileName     string `json:"file_name"`
	RelativePath string `json:"relative_path,omitempty"`
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
