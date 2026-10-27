package routes

import (
	"cosmosbdr/src/temporal/workflows"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

type BackupHandler struct {
	TemporalClient client.Client
}

func NewBackupHandler(temporalClient client.Client) *BackupHandler {
	return &BackupHandler{TemporalClient: temporalClient}
}

func (h *BackupHandler) TriggerBackup(c *fiber.Ctx) error {
	var req workflows.BackupWorkflowRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body: " + err.Error(),
		})
	}

	// Generate a unique Workflow ID based on your domain
	workflowID := fmt.Sprintf("backup-%s-%s", req.FlowType, uuid.New().String())

	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "cosmosbdr-task-queue",
	}

	// Asynchronously start the workflow
	we, err := h.TemporalClient.ExecuteWorkflow(c.Context(), options, workflows.BackupWorkflow, req)
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
