package handlers

import (
	"cosmosbdr/src/cadence/workflows"
	"cosmosbdr/src/repositories"
	"cosmosbdr/src/types"
	"fmt"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	cadenceClient "go.uber.org/cadence/client"
)

type BackupHandler struct {
	CadenceClient cadenceClient.Client
	JobItemRepo   *repositories.JobItemRepository
}

func NewBackupHandler(cadenceClient cadenceClient.Client, jobItemRepo *repositories.JobItemRepository) *BackupHandler {
	return &BackupHandler{CadenceClient: cadenceClient, JobItemRepo: jobItemRepo}
}

func (h BackupHandler) ListBackups(c *fiber.Ctx) error {
	itemPath := c.Query("item_path")
	if itemPath == "" {
		sourcePath := c.Query("source_path")
		fileName := c.Query("file_name")
		if sourcePath == "" || fileName == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "provide item_path, or both source_path and file_name",
			})
		}
		itemPath = filepath.Join(sourcePath, fileName)
	}

	items, err := h.JobItemRepo.ListEligibleBackupItemsByPath(c.Context(), itemPath)
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
			ItemPath:      item.ItemPath,
			OriginalSize:  item.OriginalSize,
			ProcessedSize: item.ProcessedSize,
			Checksum:      item.Checksum,
			BackedUpAt:    item.CreatedAt,
			JobCreatedAt:  item.Job.CreatedAt,
		})
	}

	return c.JSON(types.ListAvailableBackupsResponse{
		ItemPath: itemPath,
		Backups:  backups,
	})
}

func (h BackupHandler) TriggerBackup(c *fiber.Ctx) error {
	var req types.BackupWorkflowRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body: " + err.Error(),
		})
	}

	// Generate a unique Workflow ID based on your domain
	workflowID := fmt.Sprintf("backup_%s_%s", req.FlowType, uuid.New().String())

	options := cadenceClient.StartWorkflowOptions{
		ID:                           workflowID,
		TaskList:                     "cosmosbdr-task-queue",
		ExecutionStartToCloseTimeout: 1 * time.Hour,
	}

	// Asynchronously start the workflow
	we, err := h.CadenceClient.ExecuteWorkflow(c.Context(), options, workflows.BackupWorkflow, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to start backup workflow: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message":     "Backup workflow triggered successfully",
		"workflow_id": we.GetID(),
		"run_id":      we.GetRunID(),
	})
}
