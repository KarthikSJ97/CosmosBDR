package handlers

import (
	"context"
	"fmt"
	"log"

	"cosmosbdr/src/config"

	"go.uber.org/cadence/.gen/go/shared"
	cadenceClient "go.uber.org/cadence/client"
)

// RegisterBackupSchedules registers Cadence cron workflows for enabled sources.yaml schedule rows.
func RegisterBackupSchedules(ctx context.Context, c cadenceClient.Client, reg *config.SourceRegistry) error {
	schedules := reg.ListEnabledBackupSchedules()
	if len(schedules) == 0 {
		return nil
	}

	for _, sch := range schedules {
		req, err := BackupWorkflowRequestFromSchedule(reg, sch)
		if err != nil {
			return fmt.Errorf("schedule %q (source %q): %w", sch.ID, sch.SourceID, err)
		}

		opts := cadenceClient.StartWorkflowOptions{
			ID:                           ScheduledBackupWorkflowID(sch.ID),
			TaskList:                     TaskQueue,
			CronSchedule:                 sch.Schedule,
			WorkflowIDReusePolicy:        cadenceClient.WorkflowIDReusePolicyRejectDuplicate,
		}

		_, err = StartBackupWorkflow(ctx, c, req, opts)
		if err != nil {
			if _, ok := err.(*shared.WorkflowExecutionAlreadyStartedError); ok {
				log.Printf("Backup schedule %q: Cadence cron already registered (workflow %s)", sch.ID, opts.ID)
				continue
			}
			return fmt.Errorf("schedule %q: register cron: %w", sch.ID, err)
		}
		kind := "file"
		if IsDirectoryBackup(&req) {
			kind = "directory"
		}
		log.Printf("Registered backup schedule %q: source=%s %s backup cron %q (workflow %s)",
			sch.ID, sch.SourceID, kind, sch.Schedule, opts.ID)
	}
	return nil
}