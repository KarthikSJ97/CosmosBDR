package handlers

import (
	"context"
	"fmt"
	"log"

	"cosmosbdr/src/config"

	cadenceClient "go.uber.org/cadence/client"
)

// ReconcileBackupSchedules aligns Cadence cron workflows with sources.yaml (enabled + cron expression).
func ReconcileBackupSchedules(ctx context.Context, c cadenceClient.Client, reg *config.SourceRegistry) ([]string, int, int, int, error) {
	var touched []string
	registered := 0
	deregistered := 0
	refreshed := 0

	for _, sch := range reg.ListAllBackupSchedules() {
		want := config.YAMLEnabled(sch.BackupSchedule)
		state, err := describeScheduleRegistration(ctx, c, sch.ID)
		if err != nil {
			return touched, registered, deregistered, refreshed, fmt.Errorf("schedule %q: %w", sch.ID, err)
		}

		if !want {
			if state.Registered {
				if err := DeregisterBackupSchedule(ctx, c, sch.ID, "reconcile: yaml enabled=false"); err != nil {
					return touched, registered, deregistered, refreshed, fmt.Errorf("schedule %q: deregister: %w", sch.ID, err)
				}
				deregistered++
				touched = append(touched, sch.ID)
				log.Printf("Reconcile: deregistered schedule %q (yaml disabled)", sch.ID)
			}
			continue
		}

		if state.Registered {
			if scheduleNeedsRefresh(sch, state) {
				if err := DeregisterBackupSchedule(ctx, c, sch.ID, "reconcile: cron expression changed"); err != nil {
					return touched, registered, deregistered, refreshed, fmt.Errorf("schedule %q: refresh deregister: %w", sch.ID, err)
				}
				if err := RegisterBackupSchedule(ctx, c, reg, sch); err != nil {
					return touched, registered, deregistered, refreshed, err
				}
				refreshed++
				touched = append(touched, sch.ID)
				log.Printf("Reconcile: refreshed schedule %q cron %q", sch.ID, sch.Schedule)
			}
			continue
		}

		if err := RegisterBackupSchedule(ctx, c, reg, sch); err != nil {
			return touched, registered, deregistered, refreshed, err
		}
		registered++
		touched = append(touched, sch.ID)
		log.Printf("Reconcile: registered schedule %q cron %q", sch.ID, sch.Schedule)
	}

	return touched, registered, deregistered, refreshed, nil
}