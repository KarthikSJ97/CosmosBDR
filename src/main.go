package main

import (
	"fmt"
	"log"
	"strconv"

	"cosmosbdr/src/config"
	"cosmosbdr/src/db"
	"cosmosbdr/src/repositories"
	"cosmosbdr/src/handlers"
	"cosmosbdr/src/storage"
	"cosmosbdr/src/temporal"
	"cosmosbdr/src/temporal/activities"
	"cosmosbdr/src/temporal/workflows"

	"go.temporal.io/sdk/worker"

	"github.com/gofiber/fiber/v2"
)

func main() {
	log.Println("Initializing CosmosBDR Engine...")

	// 1. Load configuration
	cfg := config.LoadConfig()

	// 2. Initialize PostgreSQL Connection & Pool via GORM
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%v/%s?sslmode=%s",
		cfg.PostgresDBUserName, cfg.PostgresDBPassword, cfg.PostgresDBHost, cfg.PostgresDBPort, cfg.PostgresDBName, cfg.PostgresDBSSL,
	)

	gormDB, err := db.InitGORM(dbURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	sqlDB, _ := gormDB.DB()
	defer sqlDB.Close()

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

	// 5. Initialize Repositories and Activities for the Worker
	jobRepo := repositories.NewJobRepository(gormDB)
	jobActivities := &activities.JobActivities{JobRepo: jobRepo}

	// 6. Start Temporal Worker in a background goroutine
	w := worker.New(temporalClient, "cosmosbdr-task-queue", worker.Options{})
	w.RegisterWorkflow(workflows.BackupWorkflow)
	w.RegisterActivity(jobActivities.InitJobActivity)

	go func() {
		log.Println("Starting Temporal Worker...")
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Temporal worker failed: %v", err)
		}
	}()

	// 7. Initialize Fiber app instance
	app := fiber.New(fiber.Config{
		AppName: "CosmosBDR API Gateway",
	})

	// 8. Register routes
	routes.HandlerRegistry(app, temporalClient)

	// 9. Start listening on IPv4 loopback / configured port
	address := ":" + strconv.Itoa(cfg.Port)
	log.Printf("CosmosBDR API Gateway running on http://%s...", address)

	if err := app.Listen(address); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
