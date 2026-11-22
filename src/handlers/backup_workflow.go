package handlers

import (
	"context"
	"fmt"
	"time"

	"cosmosbdr/src/cadence/workflows"
	"cosmosbdr/src/config"
	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/types"

	cadenceClient "go.uber.org/cadence/client"
)

// BackupWorkflowRequestFromSchedule builds a resolved workflow request for a sources.yaml schedule row.
func BackupWorkflowRequestFromSchedule(reg *config.SourceRegistry, s config.ResolvedBackupSchedule) (types.BackupWorkflowRequest, error) {
	req := types.BackupWorkflowRequest{
		FlowType: string(schema.FlowBackup),
		Payload: types.BackupJobPayload{
			SourceID:      s.SourceID,
			RelativePath:  s.RelativePath,
			ScheduleJobID: s.ID,
			Directory:     s.Directory,
		},
	}
	if err := ResolveBackupWorkflowRequest(reg, &req); err != nil {
		return types.BackupWorkflowRequest{}, err
	}
	return req, nil
}

// StartBackupWorkflow starts BackupWorkflow or DirectoryBackupWorkflow from a resolved request.
func StartBackupWorkflow(ctx context.Context, c cadenceClient.Client, req types.BackupWorkflowRequest, opts cadenceClient.StartWorkflowOptions) (cadenceClient.WorkflowRun, error) {
	if opts.TaskList == "" {
		opts.TaskList = TaskQueue
	}
	if IsDirectoryBackup(&req) {
		if opts.ExecutionStartToCloseTimeout == 0 {
			opts.ExecutionStartToCloseTimeout = 6 * time.Hour
		}
		return c.ExecuteWorkflow(ctx, opts, workflows.DirectoryBackupWorkflow, req)
	}
	if opts.ExecutionStartToCloseTimeout == 0 {
		opts.ExecutionStartToCloseTimeout = time.Hour
	}
	return c.ExecuteWorkflow(ctx, opts, workflows.BackupWorkflow, req)
}

// ScheduledBackupWorkflowID is the stable Cadence workflow id for a schedule row.
func ScheduledBackupWorkflowID(scheduleID string) string {
	return fmt.Sprintf("scheduled_backup_%s", scheduleID)
}