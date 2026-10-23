package main

import (
	"log"
	"strconv"

	"cosmosbdr/src/config"
	"cosmosbdr/src/routes"

	"github.com/gofiber/fiber/v2"
)

func main() {
	log.Println("Initializing CosmosBDR Engine...")

	// 1. Load configuration
	cfg := config.LoadConfig()

	// 2. Initialize Fiber app instance
	app := fiber.New(fiber.Config{
		AppName: "CosmosBDR API Gateway",
	})

	// 3. Register routes
	routes.RegisterRoutes(app)

	// 4. Start listening on IPv4 loopback / configured port
	address := ":" + strconv.Itoa(cfg.Port)
	log.Printf("CosmosBDR API Gateway running on http://%s...", address)

	if err := app.Listen(address); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
