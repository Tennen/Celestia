package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
	"github.com/chentianyu/celestia/internal/models"
)

const adminStreamRecentLimit = 80

type adminStreamFrame struct {
	Reason       string                      `json:"reason,omitempty"`
	Dashboard    *models.DashboardSummary    `json:"dashboard,omitempty"`
	Plugins      *[]models.PluginRuntimeView `json:"plugins,omitempty"`
	Capabilities *[]models.Capability        `json:"capabilities,omitempty"`
	Automations  *[]models.Automation        `json:"automations,omitempty"`
	Devices      *[]models.DeviceView        `json:"devices,omitempty"`
	Events       *[]models.Event             `json:"events,omitempty"`
	Audits       *[]models.AuditRecord       `json:"audits,omitempty"`
	Event        *models.Event               `json:"event,omitempty"`
	Audit        *models.AuditRecord         `json:"audit,omitempty"`
}

func (s *Server) handleAdminStream(w http.ResponseWriter, r *http.Request) {
	if s.runtime == nil || s.runtime.EventBus == nil || s.runtime.Audit == nil {
		writeError(w, http.StatusInternalServerError, errors.New("admin stream unavailable"))
		return
	}

	snapshot, err := s.adminStreamSnapshot(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if err := writeSSEJSON(w, flusher, "sync", snapshot); err != nil {
		return
	}

	eventSubID, eventCh := s.runtime.EventBus.Subscribe(64)
	defer s.runtime.EventBus.Unsubscribe(eventSubID)
	auditSubID, auditCh := s.runtime.Audit.Subscribe(64)
	defer s.runtime.Audit.Unsubscribe(auditSubID)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			frame, err := s.adminStreamFrameForEvent(r.Context(), event)
			if err != nil {
				log.Printf("admin stream: event frame failed type=%s: %v", event.Type, err)
				continue
			}
			if err := writeSSEJSON(w, flusher, "update", frame); err != nil {
				return
			}
		case audit, ok := <-auditCh:
			if !ok {
				return
			}
			frame, err := s.adminStreamFrameForAudit(r.Context(), audit)
			if err != nil {
				log.Printf("admin stream: audit frame failed id=%s: %v", audit.ID, err)
				continue
			}
			if err := writeSSEJSON(w, flusher, "update", frame); err != nil {
				return
			}
		}
	}
}

func (s *Server) adminStreamSnapshot(ctx context.Context) (adminStreamFrame, error) {
	dashboard, err := s.gateway.Dashboard(ctx)
	if err != nil {
		return adminStreamFrame{}, err
	}
	plugins, err := s.gateway.ListPlugins(ctx)
	if err != nil {
		return adminStreamFrame{}, err
	}
	capabilities, err := s.gateway.ListCapabilities(ctx)
	if err != nil {
		return adminStreamFrame{}, err
	}
	automations, err := s.gateway.ListAutomations(ctx)
	if err != nil {
		return adminStreamFrame{}, err
	}
	devices, err := s.gateway.ListDevices(ctx, gatewayapi.DeviceFilter{})
	if err != nil {
		return adminStreamFrame{}, err
	}
	events, err := s.gateway.ListEvents(ctx, gatewayapi.EventFilter{Limit: adminStreamRecentLimit})
	if err != nil {
		return adminStreamFrame{}, err
	}
	audits, err := s.gateway.ListAudits(ctx, gatewayapi.AuditFilter{Limit: adminStreamRecentLimit})
	if err != nil {
		return adminStreamFrame{}, err
	}
	return adminStreamFrame{
		Reason:       "sync",
		Dashboard:    &dashboard,
		Plugins:      slicePtr(plugins),
		Capabilities: slicePtr(capabilities),
		Automations:  slicePtr(automations),
		Devices:      slicePtr(devices),
		Events:       slicePtr(events),
		Audits:       slicePtr(audits),
	}, nil
}

func (s *Server) adminStreamFrameForEvent(ctx context.Context, event models.Event) (adminStreamFrame, error) {
	dashboard, err := s.gateway.Dashboard(ctx)
	if err != nil {
		return adminStreamFrame{}, err
	}
	frame := adminStreamFrame{
		Reason:    string(event.Type),
		Dashboard: &dashboard,
		Event:     &event,
	}

	switch event.Type {
	case models.EventDeviceDiscovered,
		models.EventDeviceUpdated,
		models.EventDeviceStateChanged,
		models.EventDeviceOccurred,
		models.EventDeviceCommandAccept,
		models.EventDeviceCommandFailed:
		devices, err := s.gateway.ListDevices(ctx, gatewayapi.DeviceFilter{})
		if err != nil {
			return adminStreamFrame{}, err
		}
		frame.Devices = slicePtr(devices)
	case models.EventPluginLifecycleState:
		plugins, err := s.gateway.ListPlugins(ctx)
		if err != nil {
			return adminStreamFrame{}, err
		}
		devices, err := s.gateway.ListDevices(ctx, gatewayapi.DeviceFilter{})
		if err != nil {
			return adminStreamFrame{}, err
		}
		frame.Plugins = slicePtr(plugins)
		frame.Devices = slicePtr(devices)
	case models.EventCapabilityStatusChanged:
		capabilities, err := s.gateway.ListCapabilities(ctx)
		if err != nil {
			return adminStreamFrame{}, err
		}
		frame.Capabilities = slicePtr(capabilities)
	case models.EventAutomationTriggered, models.EventAutomationFailed:
		automations, err := s.gateway.ListAutomations(ctx)
		if err != nil {
			return adminStreamFrame{}, err
		}
		frame.Automations = slicePtr(automations)
	}

	return frame, nil
}

func (s *Server) adminStreamFrameForAudit(ctx context.Context, audit models.AuditRecord) (adminStreamFrame, error) {
	dashboard, err := s.gateway.Dashboard(ctx)
	if err != nil {
		return adminStreamFrame{}, err
	}
	return adminStreamFrame{
		Reason:    "audit.recorded",
		Dashboard: &dashboard,
		Audit:     &audit,
	}, nil
}

func writeSSEJSON(w http.ResponseWriter, flusher http.Flusher, eventName string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", eventName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func slicePtr[T any](items []T) *[]T {
	if items == nil {
		items = []T{}
	}
	return &items
}
