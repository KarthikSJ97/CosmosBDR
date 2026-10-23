package main

import (
	"fmt"
	"log"
	"strconv"

	"cosmosbdr/src/config"
	"cosmosbdr/src/db"
	"cosmosbdr/src/routes"
	"cosmosbdr/src/storage"
	"cosmosbdr/src/temporal"

	"github.com/gofiber/fiber/v2"
)

func main() {
	log.Println("Initializing CosmosBDR Engine...")

	// 1. Load configuration
	cfg := config.LoadConfig()

	// 2. Initialize PostgreSQL Connection Pool
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%v/%s?sslmode=%v",
		cfg.PostgresDBUserName, cfg.PostgresDBPassword, cfg.PostgresDBHost, cfg.PostgresDBPort, cfg.PostgresDBName, cfg.PostgresDBSSL,
	)
	dbPool, err := db.InitPostgres(dbURL)
	if err != nil {
		log.Printf("Warning: Could not connect to PostgreSQL: %v. (Make sure your Postgres instance is running)", err)
	} else {
		defer dbPool.Close()
	}

	// 3. Initialize SeaweedFS Storage Client
	seaweedClient, err := storage.InitSeaweedFS(
		cfg.SeaweedEndpoint,
		cfg.SeaweedRegion,
		cfg.SeaweedAccessKey,
		cfg.SeaweedSecretKey,
		cfg.SeaweedBucket,
	)
	if err != nil {
		log.Printf("Warning: Could not initialize SeaweedFS client: %v.", err)
	} else {
		_ = seaweedClient // Ready for chunk storage operations
	}

	// 4. Initialize Temporal Client
	temporalClient, err := temporal.InitClient(cfg)
	if err != nil {
		log.Printf("Warning: Could not connect to Temporal server: %v.", err)
	} else {
		defer temporalClient.Close()
	}

	// 5. Initialize Fiber app instance
	app := fiber.New(fiber.Config{
		AppName: "CosmosBDR API Gateway",
	})

	// 6. Register routes
	routes.RegisterRoutes(app)

	// 7. Start listening on IPv4 loopback / configured port
	address := ":" + strconv.Itoa(cfg.Port)
	log.Printf("CosmosBDR API Gateway running on http://%s...", address)

	if err := app.Listen(address); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
