package routes

import (
	"github.com/gofiber/fiber/v2"
	temporalClient "go.temporal.io/sdk/client"
)

// HandlerRegistry registers all API endpoints onto the Fiber app instance
func HandlerRegistry(app *fiber.App, temporalClient temporalClient.Client) {
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
	backupHandler := NewBackupHandler(temporalClient)
	v1.Post("/backups", backupHandler.TriggerBackup)
}
