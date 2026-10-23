package main

import (
	"log"
	"strconv"

	"cosmosbdr/src/config"
	"cosmosbdr/src/db"
	"cosmosbdr/src/routes"
	"cosmosbdr/src/temporal"

	"github.com/gofiber/fiber/v2"
)

func main() {
	log.Println("Initializing CosmosBDR Engine...")

	// 1. Load configuration
	cfg := config.LoadConfig()

	// 2. Initialize PostgreSQL Connection Pool
	dbPool, err := db.InitPostgres(cfg.DatabaseURL)
	if err != nil {
		log.Printf("Warning: Could not connect to PostgreSQL: %v. (Make sure your Postgres instance is running)", err)
	} else {
		defer dbPool.Close()
	}

	// 3. Initialize Temporal Client
	temporalClient, err := temporal.InitClient(cfg)
	if err != nil {
		log.Printf("Warning: Could not connect to Temporal server: %v.", err)
	} else {
		defer temporalClient.Close()
	}

	// 4. Initialize Fiber app instance
	app := fiber.New(fiber.Config{
		AppName: "CosmosBDR API Gateway",
	})

	// 5. Register routes
	routes.RegisterRoutes(app)

	// 6. Start listening on IPv4 loopback / configured port
	address := ":" + strconv.Itoa(cfg.Port)
	log.Printf("CosmosBDR API Gateway running on http://%s...", address)

	if err := app.Listen(address); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
