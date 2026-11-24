package handlers

import (
	"context"
	"log"

	"cosmosbdr/src/config"

	cadenceClient "go.uber.org/cadence/client"
)

// RegisterBackupSchedules registers Cadence cron workflows for enabled sources.yaml schedule rows.
// Prefer ReconcileBackupSchedules at startup so disabled rows and cron changes are applied.
func RegisterBackupSchedules(ctx context.Context, c cadenceClient.Client, reg *config.SourceRegistry) error {
	_, registered, deregistered, refreshed, err := ReconcileBackupSchedules(ctx, c, reg)
	if err != nil {
		return err
	}
	if registered == 0 && deregistered == 0 && refreshed == 0 {
		return nil
	}
	log.Printf("Startup schedule reconcile: registered=%d deregistered=%d refreshed=%d", registered, deregistered, refreshed)
	return nil
}