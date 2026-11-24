package types

// ScheduleSummary is one row from sources.yaml plus Cadence registration state.
type ScheduleSummary struct {
	ID                 string `json:"id"`
	SourceID           string `json:"source_id"`
	RelativePath       string `json:"relative_path"`
	Schedule           string `json:"schedule"`
	YAMLEnabled        bool   `json:"yaml_enabled"`
	Registered         bool   `json:"registered"`
	CadenceWorkflowID  string `json:"cadence_workflow_id"`
	CadenceRunID       string `json:"cadence_run_id,omitempty"`
	RegisteredCron     string `json:"registered_cron,omitempty"`
	DirectoryBackup    bool   `json:"directory_backup"`
}

// ListSchedulesResponse is returned by GET /schedules.
type ListSchedulesResponse struct {
	Schedules []ScheduleSummary `json:"schedules"`
}

// ScheduleControlResponse is returned by register, deregister, and reconcile.
type ScheduleControlResponse struct {
	ID                string `json:"id"`
	CadenceWorkflowID string `json:"cadence_workflow_id"`
	Registered        bool   `json:"registered"`
	Message           string `json:"message"`
}

// ReconcileSchedulesResponse summarizes a reconcile pass.
type ReconcileSchedulesResponse struct {
	Registered   int      `json:"registered"`
	Deregistered int      `json:"deregistered"`
	Refreshed    int      `json:"refreshed"`
	ScheduleIDs  []string `json:"schedule_ids"`
	Message      string   `json:"message"`
}