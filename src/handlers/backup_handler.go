package handlers

import (
	"cosmosbdr/src/config"
	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/repositories"
	"cosmosbdr/src/types"
	"fmt"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	cadenceClient "go.uber.org/cadence/client"
)

type BackupHandler struct {
	CadenceClient cadenceClient.Client
	JobItemRepo   *repositories.JobItemRepository
	Registry      *config.SourceRegistry
}

func NewBackupHandler(cadenceClient cadenceClient.Client, jobItemRepo *repositories.JobItemRepository, reg *config.SourceRegistry) *BackupHandler {
	return &BackupHandler{
		CadenceClient: cadenceClient,
		JobItemRepo:   jobItemRepo,
		Registry:      reg,
	}
}

func (h BackupHandler) ListBackups(c *fiber.Ctx) error {
	backupKey := c.Query("backup_key")
	itemPath := c.Query("item_path")

	var (
		items []schema.JobItem
		err   error
	)

	if backupKey != "" {
		items, err = h.JobItemRepo.ListEligibleBackupItemsByBackupKey(c.Context(), backupKey)
	} else {
		if itemPath == "" {
			sourcePath := c.Query("source_path")
			fileName := c.Query("file_name")
			if sourcePath == "" || fileName == "" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "provide backup_key, item_path, or both source_path and file_name",
				})
			}
			itemPath = filepath.Join(sourcePath, fileName)
		}
		items, err = h.JobItemRepo.ListEligibleBackupItemsByPath(c.Context(), itemPath)
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list backups: " + err.Error(),
		})
	}

	backups := make([]types.AvailableBackup, 0, len(items))
	for _, item := range items {
		backups = append(backups, types.AvailableBackup{
			JobItemID:     item.ID,
			JobID:         item.JobID,
			BackupKey:     item.BackupKey,
			ItemPath:      item.ItemPath,
			Status:        item.Status,
			Unchanged:     item.Status == "SKIPPED",
			OriginalSize:  item.OriginalSize,
			ProcessedSize: item.ProcessedSize,
			Checksum:      item.Checksum,
			BackedUpAt:    item.CreatedAt,
			JobCreatedAt:  item.Job.CreatedAt,
		})
	}

	return c.JSON(types.ListAvailableBackupsResponse{
		BackupKey: backupKey,
		ItemPath:  itemPath,
		Backups:   backups,
	})
}

func (h BackupHandler) TriggerBackup(c *fiber.Ctx) error {
	var req types.BackupWorkflowRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body: " + err.Error(),
		})
	}

	if err := ResolveBackupWorkflowRequest(h.Registry, &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if req.FlowType != string(schema.FlowBackup) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("flow_type is required and must be %q", schema.FlowBackup),
		})
	}

	workflowLabel := "Backup"
	options := cadenceClient.StartWorkflowOptions{TaskList: TaskQueue}
	if IsDirectoryBackup(&req) {
		options.ID = fmt.Sprintf("backup_dir_%s", uuid.New().String())
		workflowLabel = "Directory backup"
	} else {
		options.ID = fmt.Sprintf("backup_%s_%s", req.FlowType, uuid.New().String())
	}

	we, err := StartBackupWorkflow(c.Context(), h.CadenceClient, req, options)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to start backup workflow: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message":     workflowLabel + " workflow triggered successfully",
		"workflow_id": we.GetID(),
		"run_id":      we.GetRunID(),
	})
}
