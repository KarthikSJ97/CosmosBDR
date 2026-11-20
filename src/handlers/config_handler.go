package handlers

import (
	"cosmosbdr/src/config"
	"cosmosbdr/src/types"

	"github.com/gofiber/fiber/v2"
)

type ConfigHandler struct {
	Registry *config.SourceRegistry
}

func NewConfigHandler(reg *config.SourceRegistry) *ConfigHandler {
	return &ConfigHandler{Registry: reg}
}

func (h *ConfigHandler) ListSources(c *fiber.Ctx) error {
	sources := h.Registry.ListSources()
	out := make([]types.SourceSummary, 0, len(sources))
	for _, s := range sources {
		out = append(out, types.SourceSummary{
			ID:             s.ID,
			Type:           s.Type,
			Domain:         s.Domain,
			ContainerMount: s.ContainerMount,
			Host:           s.Host,
			TTLDays:        s.Retention.TTLDays,
		})
	}
	return c.JSON(fiber.Map{"sources": out})
}