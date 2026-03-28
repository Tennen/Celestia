package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/core/control"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
	"github.com/google/uuid"
)

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := s.runtime.Registry.List(r.Context(), storage.DeviceFilter{
		PluginID: r.URL.Query().Get("plugin_id"),
		Kind:     r.URL.Query().Get("kind"),
		Query:    r.URL.Query().Get("q"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	states, err := s.runtime.State.List(r.Context(), storage.StateFilter{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	stateMap := map[string]models.DeviceStateSnapshot{}
	for _, item := range states {
		stateMap[item.DeviceID] = item
	}
	out := make([]models.DeviceView, 0, len(devices))
	for _, device := range devices {
		view, err := s.deviceView(r.Context(), device, stateMap[device.ID])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, view)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDevice(w http.ResponseWriter, r *http.Request) {
	device, ok, err := s.runtime.Registry.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("device not found"))
		return
	}
	state, _, err := s.runtime.State.Get(r.Context(), device.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	view, err := s.deviceView(r.Context(), device, state)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleUpdateDevicePreference(w http.ResponseWriter, r *http.Request) {
	device, ok, err := s.runtime.Registry.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("device not found"))
		return
	}

	var payload struct {
		Alias string `json:"alias"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	pref := models.DevicePreference{
		DeviceID:  device.ID,
		Alias:     strings.TrimSpace(payload.Alias),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.runtime.Store.UpsertDevicePreference(r.Context(), pref); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, pref)
}

func (s *Server) handleUpdateControlPreference(w http.ResponseWriter, r *http.Request) {
	device, ok, err := s.runtime.Registry.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("device not found"))
		return
	}
	state, _, err := s.runtime.State.Get(r.Context(), device.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	view, err := s.deviceView(r.Context(), device, state)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	controlID := strings.TrimSpace(r.PathValue("controlId"))
	if !hasControl(view.Controls, controlID) {
		writeError(w, http.StatusNotFound, errors.New("control not found"))
		return
	}

	var payload struct {
		Alias   string `json:"alias"`
		Visible *bool  `json:"visible"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	visible := true
	if payload.Visible != nil {
		visible = *payload.Visible
	}
	pref := models.DeviceControlPreference{
		DeviceID:  device.ID,
		ControlID: controlID,
		Alias:     strings.TrimSpace(payload.Alias),
		Visible:   visible,
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.runtime.Store.UpsertDeviceControlPreference(r.Context(), pref); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, pref)
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	device, ok, err := s.runtime.Registry.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("device not found"))
		return
	}
	var payload struct {
		Action string         `json:"action"`
		Params map[string]any `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.executeDeviceCommand(w, r, device, models.CommandRequest{
		DeviceID:  device.ID,
		Action:    payload.Action,
		Params:    payload.Params,
		RequestID: uuid.NewString(),
	})
}

func (s *Server) handleToggleOn(w http.ResponseWriter, r *http.Request) {
	s.handleToggleControl(w, r, true)
}

func (s *Server) handleToggleOff(w http.ResponseWriter, r *http.Request) {
	s.handleToggleControl(w, r, false)
}

func (s *Server) handleToggleControl(w http.ResponseWriter, r *http.Request, on bool) {
	device, state, controlID, ok := s.resolveControlTarget(w, r)
	if !ok {
		return
	}
	req, err := s.runtime.Controls.ResolveToggle(device, state, controlID, on)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.RequestID = uuid.NewString()
	s.executeDeviceCommand(w, r, device, req)
}

func (s *Server) handleActionControl(w http.ResponseWriter, r *http.Request) {
	device, state, controlID, ok := s.resolveControlTarget(w, r)
	if !ok {
		return
	}
	req, err := s.runtime.Controls.ResolveAction(device, state, controlID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.RequestID = uuid.NewString()
	s.executeDeviceCommand(w, r, device, req)
}

func (s *Server) resolveControlTarget(w http.ResponseWriter, r *http.Request) (models.Device, models.DeviceStateSnapshot, string, bool) {
	deviceID, controlID, err := control.ParseCompoundControlID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return models.Device{}, models.DeviceStateSnapshot{}, "", false
	}
	device, ok, err := s.runtime.Registry.Get(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return models.Device{}, models.DeviceStateSnapshot{}, "", false
	}
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("device not found"))
		return models.Device{}, models.DeviceStateSnapshot{}, "", false
	}
	state, _, err := s.runtime.State.Get(r.Context(), device.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return models.Device{}, models.DeviceStateSnapshot{}, "", false
	}
	return device, state, controlID, true
}

func (s *Server) executeDeviceCommand(w http.ResponseWriter, r *http.Request, device models.Device, req models.CommandRequest) {
	decision := s.runtime.Policy.Evaluate(actorFromRequest(r), req.Action)
	auditRecord := models.AuditRecord{
		ID:        uuid.NewString(),
		Actor:     actorFromRequest(r),
		DeviceID:  device.ID,
		Action:    req.Action,
		Params:    req.Params,
		Allowed:   decision.Allowed,
		RiskLevel: decision.RiskLevel,
		CreatedAt: time.Now().UTC(),
	}
	if !decision.Allowed {
		auditRecord.Result = "denied"
		_ = s.runtime.Audit.Append(r.Context(), auditRecord)
		writeJSON(w, http.StatusForbidden, map[string]any{
			"allowed": false,
			"reason":  decision.Reason,
		})
		return
	}
	resp, err := s.runtime.PluginMgr.ExecuteCommand(r.Context(), device, req)
	if err != nil {
		auditRecord.Result = "failed"
		_ = s.runtime.Audit.Append(r.Context(), auditRecord)
		writeError(w, http.StatusBadGateway, err)
		return
	}
	auditRecord.Result = "accepted"
	if err := s.runtime.Audit.Append(r.Context(), auditRecord); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"decision": decision,
		"result":   resp,
	})
}

func (s *Server) deviceView(ctx context.Context, device models.Device, state models.DeviceStateSnapshot) (models.DeviceView, error) {
	view := s.runtime.Controls.BuildView(device, state)
	devicePref, _, err := s.runtime.Store.GetDevicePreference(ctx, device.ID)
	if err != nil {
		return models.DeviceView{}, err
	}
	view.Device = applyDevicePreference(view.Device, devicePref)
	prefs, err := s.runtime.Store.ListDeviceControlPreferences(ctx, device.ID)
	if err != nil {
		return models.DeviceView{}, err
	}
	return s.runtime.Controls.ApplyPreferences(view, prefs), nil
}

func applyDevicePreference(device models.Device, pref models.DevicePreference) models.Device {
	alias := strings.TrimSpace(pref.Alias)
	if alias == "" {
		return device
	}
	device.DefaultName = device.Name
	device.Alias = alias
	device.Name = alias
	return device
}

func hasControl(controls []models.DeviceControl, controlID string) bool {
	for _, control := range controls {
		if control.ID == controlID {
			return true
		}
	}
	return false
}
