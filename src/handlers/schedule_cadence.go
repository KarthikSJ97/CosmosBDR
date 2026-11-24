package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cosmosbdr/src/config"

	"go.uber.org/cadence"
	"go.uber.org/cadence/.gen/go/shared"
	cadenceClient "go.uber.org/cadence/client"
)

type scheduleRegistration struct {
	Registered        bool
	CadenceWorkflowID string
	CadenceRunID      string
	RegisteredCron    string
}

func describeScheduleRegistration(ctx context.Context, c cadenceClient.Client, scheduleID string) (scheduleRegistration, error) {
	wfID := ScheduledBackupWorkflowID(scheduleID)
	out := scheduleRegistration{CadenceWorkflowID: wfID}
	resp, err := c.DescribeWorkflowExecution(ctx, wfID, "")
	if err != nil {
		if isWorkflowDescribeNotFound(err) {
			return out, nil
		}
		return out, err
	}
	if resp == nil || resp.WorkflowExecutionInfo == nil {
		return out, nil
	}
	info := resp.WorkflowExecutionInfo
	if info.Execution != nil {
		out.CadenceRunID = info.Execution.GetRunId()
	}
	if info.CronSchedule != nil {
		out.RegisteredCron = *info.CronSchedule
	}
	out.Registered = scheduleCronWorkflowOpen(info)
	return out, nil
}

func scheduleCronWorkflowOpen(info *shared.WorkflowExecutionInfo) bool {
	if info == nil || info.CronSchedule == nil || strings.TrimSpace(*info.CronSchedule) == "" {
		return false
	}
	return info.CloseStatus == nil
}

func isWorkflowDescribeNotFound(err error) bool {
	if err == nil {
		return false
	}
	var notExists *shared.EntityNotExistsError
	if errors.As(err, &notExists) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not exist") || strings.Contains(msg, "not found")
}

// RegisterBackupSchedule starts the Cadence cron for one sources.yaml schedule row.
func RegisterBackupSchedule(ctx context.Context, c cadenceClient.Client, reg *config.SourceRegistry, sch config.ResolvedBackupSchedule) error {
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
			return nil
		}
		if cadence.IsWorkflowExecutionAlreadyStartedError(err) {
			return nil
		}
		return fmt.Errorf("schedule %q: register cron: %w", sch.ID, err)
	}
	return nil
}

// DeregisterBackupSchedule terminates the Cadence cron workflow for a schedule id.
func DeregisterBackupSchedule(ctx context.Context, c cadenceClient.Client, scheduleID, reason string) error {
	if reason == "" {
		reason = "deregistered via CosmosBDR API"
	}
	wfID := ScheduledBackupWorkflowID(scheduleID)
	reg, err := describeScheduleRegistration(ctx, c, scheduleID)
	if err != nil {
		return err
	}
	if !reg.Registered {
		return nil
	}
	return c.TerminateWorkflow(ctx, wfID, "", reason, nil)
}

func scheduleNeedsRefresh(sch config.ResolvedBackupSchedule, reg scheduleRegistration) bool {
	if !reg.Registered {
		return false
	}
	want := strings.TrimSpace(sch.Schedule)
	got := strings.TrimSpace(reg.RegisteredCron)
	return got != "" && want != got
}