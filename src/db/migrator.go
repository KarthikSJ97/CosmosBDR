package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"cosmosbdr/src/db/schema" // Import your schema package

	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

func RunMigrations(ctx context.Context, dbPool *pgxpool.Pool, gormDB *gorm.DB, migrationDir string) error {
	files, err := os.ReadDir(migrationDir)
	if err != nil {
		return fmt.Errorf("failed to read schema directory: %w", err)
	}

	var sqlFiles []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".sql" {
			sqlFiles = append(sqlFiles, file.Name())
		}
	}

	// 🚀 Fallback: If no .sql files exist, use GORM AutoMigrate to bootstrap!
	if len(sqlFiles) == 0 {
		log.Println("[Migrator] No .sql migration files found. Bootstrapping tables via GORM AutoMigrate...")
		err := gormDB.AutoMigrate(
			// Append all the schema tables here
			&schema.Job{},
			&schema.JobItem{},
			&schema.Chunk{},
			&schema.ItemChunk{},
		)
		if err != nil {
			return fmt.Errorf("gorm auto-migration failed: %w", err)
		}
		log.Println("[Migrator] GORM bootstrap tables created successfully!")
		return nil
	}

	// Otherwise, run the standard versioned SQL files
	sort.Strings(sqlFiles)

	for _, filename := range sqlFiles {
		fullPath := filepath.Join(migrationDir, filename)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("failed to read schema file %s: %w", filename, err)
		}

		log.Printf("[Migrator] Applying schema: %s", filename)
		_, err = dbPool.Exec(ctx, string(content))
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", filename, err)
		}
	}

	log.Println("[Migrator] All database schemas applied successfully!")
	return nil
}