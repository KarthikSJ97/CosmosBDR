package handlers

import (
	"testing"

	"cosmosbdr/src/config"
)

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