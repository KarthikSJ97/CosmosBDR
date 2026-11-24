package main

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"cosmosbdr/src/cadence"
	"cosmosbdr/src/cadence/activities"
	"cosmosbdr/src/cadence/workflows"
	"cosmosbdr/src/chunkcodec"
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

	// 1. Load configuration
	cfg := config.LoadConfig()

	sourceRegistry, err := config.LoadSourceRegistry(cfg)
	if err != nil {
		log.Fatalf("Failed to load sources config: %v", err)
	}
	log.Printf("Loaded %d source(s)", len(sourceRegistry.Sources))

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
		log.Fatalf("Failed to initialize SeaweedFS storage: %v", err)
	}

	// 4. Initialize Cadence Client[cite: 1]
	cadenceClient, serviceClient, err := cadence.InitClient(cfg)
	if err != nil {
		log.Printf("Warning: Could not connect to Cadence server: %v.", err)
		cadenceClient = nil
		serviceClient = nil
	}

	encKey, err := chunkcodec.ParseAES256Key(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("Invalid ENCRYPTION_KEY: %v", err)
	}
	chunkCodec, err := chunkcodec.New(encKey, cfg.CompressionAlgorithm)
	if err != nil {
		log.Fatalf("Invalid chunk codec config: %v", err)
	}
	log.Printf("Chunk pipeline: compression=%s, encryption=AES-256-GCM", chunkCodec.CompressionAlgorithm())

	// 5. Initialize Repositories and Activities for the Worker
	jobRepo := repositories.NewJobRepository(gormDB)
	jobItemRepo := repositories.NewJobItemRepository(gormDB)
	chunkRepo := repositories.NewChunkRepository(gormDB)
	jobActivities := &activities.JobActivities{
		JobRepo:                              jobRepo,
		JobItemRepo:                          jobItemRepo,
		ChunkRepo:                            chunkRepo,
		Seaweed:                              seaweedClient,
		ChunkCodec:                           chunkCodec,
		Registry:                             sourceRegistry,
		CadenceMaxConcurrentWorkflows:        cfg.EffectiveCadenceMaxConcurrentWorkflows(),
		CadenceWorkerMaxConcurrentActivities: cfg.EffectiveCadenceWorkerMaxConcurrentActivities(),
		RetentionGCItemBatchSize:             cfg.EffectiveRetentionGCItemBatchSize(),
		RetentionGCOrphanBatchSize:           cfg.EffectiveRetentionGCOrphanBatchSize(),
	}

	// 6. Start Cadence Worker in a background goroutine using NewV2
	domain := "samples-domain"
	taskQueueName := handlers.TaskQueue

	workerOpts := worker.Options{
		MaxConcurrentActivityExecutionSize: cfg.EffectiveCadenceWorkerMaxConcurrentActivities(),
	}

	if serviceClient == nil {
		if len(sourceRegistry.ListEnabledBackupSchedules()) > 0 {
			log.Fatalf("sources.yaml has enabled backup schedules but Cadence is unavailable")
		}
		log.Printf("Cadence worker not started (no service client)")
	} else {
		w, err := worker.NewV2(
			serviceClient,
			domain,
			taskQueueName,
			workerOpts,
		)
		if err != nil {
			log.Fatalf("Failed to create Cadence worker: %v", err)
		}

		w.RegisterWorkflow(workflows.BackupWorkflow)
		w.RegisterWorkflow(workflows.DirectoryBackupWorkflow)
		w.RegisterWorkflow(workflows.DirectoryRestoreWorkflow)
		w.RegisterWorkflow(workflows.RestoreWorkflow)
		w.RegisterWorkflow(workflows.RetentionGCWorkflow)
		w.RegisterActivity(jobActivities.InitJobActivity)
		w.RegisterActivity(jobActivities.UpdateJobStatusActivity)
		w.RegisterActivity(jobActivities.RunPreBackupActivity)
		w.RegisterActivity(jobActivities.CleanupPreBackupActivity)
		w.RegisterActivity(jobActivities.InspectFileActivity)
		w.RegisterActivity(jobActivities.ProcessFileChunkActivity)
		w.RegisterActivity(jobActivities.CompleteJobItemActivity)
		w.RegisterActivity(jobActivities.PublishSnapshotManifestActivity)
		w.RegisterActivity(jobActivities.FinalizeSkippedBackupItemActivity)
		w.RegisterActivity(jobActivities.FinalizeJobActivity)
		w.RegisterActivity(jobActivities.ListDirectoryActivity)
		w.RegisterActivity(jobActivities.ListDirectoryRestoreActivity)
		w.RegisterActivity(jobActivities.GetCadenceRuntimeLimitsActivity)
		w.RegisterActivity(jobActivities.FailJobExecutionActivity)
		w.RegisterActivity(jobActivities.ResolveRestoreSourceActivity)
		w.RegisterActivity(jobActivities.PrepareRestoreTargetActivity)
		w.RegisterActivity(jobActivities.RestoreFileChunkActivity)
		w.RegisterActivity(jobActivities.RetentionGCActivity)

		go func() {
			log.Println("Starting Cadence Worker...")
			if err := w.Start(); err != nil {
				log.Fatalf("Cadence worker failed to start: %v", err)
			}
		}()

		if err := handlers.RegisterBackupSchedules(context.Background(), cadenceClient, sourceRegistry); err != nil {
			log.Fatalf("Failed to register backup schedules: %v", err)
		}
		if err := handlers.RegisterRetentionGCCron(context.Background(), cadenceClient, cfg); err != nil {
			log.Fatalf("Failed to register retention GC cron: %v", err)
		}
	}

	// 7. Initialize Fiber app instance[cite: 1]
	app := fiber.New(fiber.Config{
		AppName: "CosmosBDR API Gateway",
	})

	// 8. Register handlers
	handlers.HandlerRegistry(app, cadenceClient, jobItemRepo, sourceRegistry)

	// 9. Start listening on IPv4 loopback / configured port[cite: 1]
	address := ":" + strconv.Itoa(cfg.Port)
	log.Printf("CosmosBDR API Gateway running on http://%s...", address)

	if err := app.Listen(address); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
