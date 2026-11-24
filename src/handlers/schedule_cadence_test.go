package handlers

import (
	"testing"

	"cosmosbdr/src/config"

	"go.uber.org/cadence/.gen/go/shared"
)

func TestWorkflowExecutionOpen_InFlightCronTick(t *testing.T) {
	if !workflowExecutionOpen(&shared.WorkflowExecutionInfo{CloseStatus: nil}) {
		t.Fatal("open run without CronSchedule should count as registered for deregister")
	}
	terminated := shared.WorkflowExecutionCloseStatusTerminated
	if workflowExecutionOpen(&shared.WorkflowExecutionInfo{CloseStatus: &terminated}) {
		t.Fatal("closed run should not be open")
	}
}

func TestScheduleIdleCronRegistered(t *testing.T) {
	sch := config.ResolvedBackupSchedule{
		BackupSchedule: config.BackupSchedule{Schedule: "0 3 * * *"},
	}
	open := scheduleRegistration{Registered: true, RegisteredCron: "0 3 * * *"}
	if !scheduleIdleCronRegistered(open, sch) {
		t.Fatal("idle cron should count as registered")
	}
	inFlight := scheduleRegistration{Registered: true}
	if scheduleIdleCronRegistered(inFlight, sch) {
		t.Fatal("open run without cron metadata is not idle cron")
	}
}

func TestScheduleNeedsRefresh(t *testing.T) {
	sch := config.ResolvedBackupSchedule{
		BackupSchedule: config.BackupSchedule{Schedule: "0 2 * * *"},
	}
	if scheduleNeedsRefresh(sch, scheduleRegistration{Registered: true, RegisteredCron: "0 2 * * *"}) {
		t.Fatal("same cron should not refresh")
	}
	if !scheduleNeedsRefresh(sch, scheduleRegistration{Registered: true, RegisteredCron: "0 3 * * *"}) {
		t.Fatal("diff cron should refresh")
	}
}