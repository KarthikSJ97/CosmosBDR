package handlers

import (
	"strings"

	"cosmosbdr/src/config"
	"cosmosbdr/src/types"

	"github.com/gofiber/fiber/v2"
	cadenceClient "go.uber.org/cadence/client"
)

type ScheduleHandler struct {
	CadenceClient cadenceClient.Client
	Registry      *config.SourceRegistry
}

func NewScheduleHandler(c cadenceClient.Client, reg *config.SourceRegistry) *ScheduleHandler {
	return &ScheduleHandler{CadenceClient: c, Registry: reg}
}

func (h *ScheduleHandler) requireCadence(c *fiber.Ctx) error {
	if h.CadenceClient == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Cadence client is not available",
		})
	}
	return nil
}

func (h *ScheduleHandler) ListSchedules(c *fiber.Ctx) error {
	if err := h.requireCadence(c); err != nil {
		return err
	}

	out := make([]types.ScheduleSummary, 0)
	for _, sch := range h.Registry.ListAllBackupSchedules() {
		summary := scheduleSummaryFromYAML(sch)
		state, err := describeScheduleRegistration(c.Context(), h.CadenceClient, sch.ID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to describe schedule " + sch.ID + ": " + err.Error(),
			})
		}
		applyRegistrationState(&summary, state)
		out = append(out, summary)
	}
	return c.JSON(types.ListSchedulesResponse{Schedules: out})
}

func scheduleSummaryFromYAML(sch config.ResolvedBackupSchedule) types.ScheduleSummary {
	return types.ScheduleSummary{
		ID:                sch.ID,
		SourceID:          sch.SourceID,
		RelativePath:      sch.RelativePath,
		Schedule:          sch.Schedule,
		YAMLEnabled:       config.YAMLEnabled(sch.BackupSchedule),
		CadenceWorkflowID: ScheduledBackupWorkflowID(sch.ID),
		DirectoryBackup:   sch.Directory != nil,
	}
}

func applyRegistrationState(summary *types.ScheduleSummary, state scheduleRegistration) {
	summary.Registered = state.Registered
	summary.CadenceRunID = state.CadenceRunID
	summary.RegisteredCron = state.RegisteredCron
}

func (h *ScheduleHandler) RegisterSchedule(c *fiber.Ctx) error {
	if err := h.requireCadence(c); err != nil {
		return err
	}
	scheduleID := strings.TrimSpace(c.Params("id"))
	if scheduleID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "schedule id is required"})
	}
	sch, ok := h.Registry.FindBackupSchedule(scheduleID)
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "unknown schedule id " + scheduleID,
		})
	}

	state, err := describeScheduleRegistration(c.Context(), h.CadenceClient, scheduleID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if state.Registered && !scheduleNeedsRefresh(sch, state) {
		return c.JSON(types.ScheduleControlResponse{
			ID:                scheduleID,
			CadenceWorkflowID: state.CadenceWorkflowID,
			Registered:        true,
			Message:           "schedule already registered",
		})
	}
	if state.Registered {
		if err := DeregisterBackupSchedule(c.Context(), h.CadenceClient, scheduleID, "register: refresh cron"); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}
	if err := RegisterBackupSchedule(c.Context(), h.CadenceClient, h.Registry, sch); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusAccepted).JSON(types.ScheduleControlResponse{
		ID:                scheduleID,
		CadenceWorkflowID: ScheduledBackupWorkflowID(scheduleID),
		Registered:        true,
		Message:           "schedule registered",
	})
}

type deregisterScheduleBody struct {
	Reason string `json:"reason"`
}

func (h *ScheduleHandler) DeregisterSchedule(c *fiber.Ctx) error {
	if err := h.requireCadence(c); err != nil {
		return err
	}
	scheduleID := strings.TrimSpace(c.Params("id"))
	if scheduleID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "schedule id is required"})
	}
	if _, ok := h.Registry.FindBackupSchedule(scheduleID); !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "unknown schedule id " + scheduleID,
		})
	}

	reason := "deregistered via CosmosBDR API"
	var body deregisterScheduleBody
	if c.Request().Header.ContentLength() > 0 {
		_ = c.BodyParser(&body)
		if strings.TrimSpace(body.Reason) != "" {
			reason = strings.TrimSpace(body.Reason)
		}
	}

	if err := DeregisterBackupSchedule(c.Context(), h.CadenceClient, scheduleID, reason); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(types.ScheduleControlResponse{
		ID:                scheduleID,
		CadenceWorkflowID: ScheduledBackupWorkflowID(scheduleID),
		Registered:        false,
		Message:           "schedule deregistered",
	})
}

func (h *ScheduleHandler) ReconcileSchedules(c *fiber.Ctx) error {
	if err := h.requireCadence(c); err != nil {
		return err
	}
	ids, registered, deregistered, refreshed, err := ReconcileBackupSchedules(c.Context(), h.CadenceClient, h.Registry)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(types.ReconcileSchedulesResponse{
		Registered:   registered,
		Deregistered: deregistered,
		Refreshed:    refreshed,
		ScheduleIDs:  ids,
		Message:      "schedules reconciled with sources.yaml",
	})
}