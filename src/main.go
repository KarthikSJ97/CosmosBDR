package main

import (
	"log"
	"strconv"

	"cosmosbdr/src/config"
	"cosmosbdr/src/routes"
	"cosmosbdr/src/temporal"

	"github.com/gofiber/fiber/v2"
)

func main() {
	log.Println("Initializing CosmosBDR Engine...")

	// 1. Load configuration
	cfg := config.LoadConfig()

	// 2. Initialize Temporal Client
	temporalClient, err := temporal.InitClient(cfg)
	if err != nil {
		log.Printf("Warning: Could not connect to Temporal server: %v. (Make sure Temporal dev server is running)", err)
	} else {
		defer temporalClient.Close()
	}

	// 3. Initialize Fiber app instance
	app := fiber.New(fiber.Config{
		AppName: "CosmosBDR API Gateway",
	})

	// 4. Register routes
	routes.RegisterRoutes(app)

	// 5. Start listening on IPv4 loopback / configured port
	address := ":" + strconv.Itoa(cfg.Port)
	log.Printf("CosmosBDR API Gateway running on http://%s...", address)

	if err := app.Listen(address); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
