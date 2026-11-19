package handlers

import (
	"cosmosbdr/src/repositories"

	"github.com/gofiber/fiber/v2"
	cadenceClient "go.uber.org/cadence/client"
)

// HandlerRegistry registers all API endpoints onto the Fiber app instance
func HandlerRegistry(app *fiber.App, cadenceClient cadenceClient.Client, jobItemRepo *repositories.JobItemRepository) {
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"service": "cosmosbdr-api",
		})
	})

	v1 := app.Group("/api/v1")
	v1.Get("/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "CosmosBDR control plane active",
		})
	})
	backupHandler := NewBackupHandler(cadenceClient, jobItemRepo)
	v1.Get("/backups", backupHandler.ListBackups)
	v1.Post("/backups", backupHandler.TriggerBackup)

	restoreHandler := NewRestoreHandler(cadenceClient)
	v1.Post("/restores", restoreHandler.TriggerRestore)
}
