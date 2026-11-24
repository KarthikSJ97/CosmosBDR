package handlers

import (
	"context"
	"time"

	"cosmosbdr/src/cadence/workflows"
	"cosmosbdr/src/types"

	cadenceClient "go.uber.org/cadence/client"
)

// StartRestoreWorkflow starts RestoreWorkflow or DirectoryRestoreWorkflow from a resolved request.
func StartRestoreWorkflow(ctx context.Context, c cadenceClient.Client, req types.BackupWorkflowRequest, opts cadenceClient.StartWorkflowOptions) (cadenceClient.WorkflowRun, error) {
	if opts.TaskList == "" {
		opts.TaskList = TaskQueue
	}
	if IsDirectoryRestore(&req) {
		if opts.ExecutionStartToCloseTimeout == 0 {
			opts.ExecutionStartToCloseTimeout = 6 * time.Hour
		}
		return c.ExecuteWorkflow(ctx, opts, workflows.DirectoryRestoreWorkflow, req)
	}
	if opts.ExecutionStartToCloseTimeout == 0 {
		opts.ExecutionStartToCloseTimeout = time.Hour
	}
	return c.ExecuteWorkflow(ctx, opts, workflows.RestoreWorkflow, req)
}