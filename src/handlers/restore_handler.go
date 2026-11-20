package handlers

import (
	"cosmosbdr/src/cadence/workflows"
	"cosmosbdr/src/config"
	"cosmosbdr/src/db/schema"
	"cosmosbdr/src/types"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	cadenceClient "go.uber.org/cadence/client"
)

type RestoreHandler struct {
	CadenceClient cadenceClient.Client
	Registry      *config.SourceRegistry
}

func NewRestoreHandler(cadenceClient cadenceClient.Client, reg *config.SourceRegistry) *RestoreHandler {
	return &RestoreHandler{CadenceClient: cadenceClient, Registry: reg}
}

func (h RestoreHandler) TriggerRestore(c *fiber.Ctx) error {
	var req types.BackupWorkflowRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body: " + err.Error(),
		})
	}

	if req.FlowType == "" {
		req.FlowType = string(schema.FlowRestore)
	}
	if req.FlowType != string(schema.FlowRestore) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("flow_type must be %q for restore", schema.FlowRestore),
		})
	}
	if strings.TrimSpace(req.Payload.SourceID) != "" {
		if err := resolveBackupWorkflowRequest(h.Registry, &req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
	} else {
		if req.Payload.FileName == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "payload.file_name is required",
			})
		}
		if req.SourcePath == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "source_path is required (used with file_name to locate the backup), or use source_id + relative_path",
			})
		}
	}

	workflowID := fmt.Sprintf("restore_%s_%s", req.FlowType, uuid.New().String())
	options := cadenceClient.StartWorkflowOptions{
		ID:                           workflowID,
		TaskList:                     "cosmosbdr-task-queue",
		ExecutionStartToCloseTimeout: 1 * time.Hour,
	}

	we, err := h.CadenceClient.ExecuteWorkflow(c.Context(), options, workflows.RestoreWorkflow, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to start restore workflow: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message":     "Restore workflow triggered successfully",
		"workflow_id": we.GetID(),
		"run_id":      we.GetRunID(),
	})
}