package main

import (
	"fmt"
	"log"
	"strconv"

	"cosmosbdr/src/cadence"
	"cosmosbdr/src/cadence/activities"
	"cosmosbdr/src/cadence/workflows"
	"cosmosbdr/src/config"
	"cosmosbdr/src/db"
	"cosmosbdr/src/handlers"
	"cosmosbdr/src/repositories"
	"cosmosbdr/src/storage"

	"go.uber.org/cadence/worker"

	"github.com/gofiber/fiber/v2"
)

func main() {
	log.Println("Initializing CosmosBDR Engine...")

	// 1. Load configuration[cite: 1]
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

	// 3. Initialize SeaweedFS Storage Client[cite: 1]
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

	// 4. Initialize Cadence Client[cite: 1]
	cadenceClient, serviceClient, err := cadence.InitClient(cfg)
	if err != nil {
		log.Printf("Warning: Could not connect to Cadence server: %v.", err)
	} else {
		// Cadence client manages connection lifecycle through YARPC dispatcher
		_ = cadenceClient
	}

	// 5. Initialize Repositories and Activities for the Worker
	jobRepo := repositories.NewJobRepository(gormDB)
	jobActivities := &activities.JobActivities{JobRepo: jobRepo}

	// 6. Start Cadence Worker in a background goroutine using NewV2
	domain := "samples-domain"
	taskQueueName := "cosmosbdr-task-queue"

	w, err := worker.NewV2(
		serviceClient,
		domain,
		taskQueueName,
		worker.Options{},
	)
	if err != nil {
		log.Fatalf("Failed to create Cadence worker: %v", err)
	}

	w.RegisterWorkflow(workflows.BackupWorkflow)
	w.RegisterActivity(jobActivities.InitJobActivity)

	go func() {
		log.Println("Starting Cadence Worker...")
		if err := w.Start(); err != nil {
			log.Fatalf("Cadence worker failed to start: %v", err)
		}
	}()

	// 7. Initialize Fiber app instance[cite: 1]
	app := fiber.New(fiber.Config{
		AppName: "CosmosBDR API Gateway",
	})

	// 8. Register handlers
	handlers.HandlerRegistry(app, cadenceClient)

	// 9. Start listening on IPv4 loopback / configured port[cite: 1]
	address := ":" + strconv.Itoa(cfg.Port)
	log.Printf("CosmosBDR API Gateway running on http://%s...", address)

	if err := app.Listen(address); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
