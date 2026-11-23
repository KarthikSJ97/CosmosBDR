package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"cosmosbdr/src/cadence/workflows"
	"cosmosbdr/src/config"
	"cosmosbdr/src/types"

	"go.uber.org/cadence/.gen/go/shared"
	cadenceClient "go.uber.org/cadence/client"
)

// RegisterRetentionGCCron starts the daily retention + chunk GC workflow when Cadence is available.
func RegisterRetentionGCCron(ctx context.Context, c cadenceClient.Client, cfg *config.Config) error {
	if c == nil {
		return nil
	}

	workflowID := cfg.EffectiveRetentionGCWorkflowID()
	cronSchedule := cfg.EffectiveRetentionGCCronSchedule()
	execTimeout := time.Duration(cfg.EffectiveRetentionGCExecutionTimeoutHours()) * time.Hour

	opts := cadenceClient.StartWorkflowOptions{
		ID:                           workflowID,
		TaskList:                     TaskQueue,
		CronSchedule:                 cronSchedule,
		WorkflowIDReusePolicy:        cadenceClient.WorkflowIDReusePolicyRejectDuplicate,
		ExecutionStartToCloseTimeout: execTimeout,
	}

	req := types.RetentionGCRequest{}
	_, err := c.ExecuteWorkflow(ctx, opts, workflows.RetentionGCWorkflow, req)
	if err != nil {
		if _, ok := err.(*shared.WorkflowExecutionAlreadyStartedError); ok {
			log.Printf("Retention GC cron already registered (workflow %s)", opts.ID)
			return nil
		}
		return fmt.Errorf("register retention GC cron: %w", err)
	}
	log.Printf("Registered retention GC cron %q (workflow %s)", cronSchedule, opts.ID)
	return nil
}