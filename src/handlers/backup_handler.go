package handlers

import (
	"cosmosbdr/src/cadence/workflows"
	"cosmosbdr/src/types"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	cadenceClient "go.uber.org/cadence/client"
)

type BackupHandler struct {
	CadenceClient cadenceClient.Client
}

func NewBackupHandler(cadenceClient cadenceClient.Client) *BackupHandler {
	return &BackupHandler{CadenceClient: cadenceClient}
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
