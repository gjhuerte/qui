// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/gjhuerte/qui/internal/models"
	"github.com/gjhuerte/qui/internal/services/gluetun"
)

// GluetunHandler serves the Gluetun page: VPN status, logs and restarts.
type GluetunHandler struct {
	service *gluetun.Service
}

func NewGluetunHandler(service *gluetun.Service) *GluetunHandler {
	return &GluetunHandler{service: service}
}

func (h *GluetunHandler) Routes(r chi.Router) {
	r.Route("/gluetun", func(r chi.Router) {
		r.Get("/status", h.GetStatus)
		r.Get("/logs", h.GetLogs)
		r.Get("/vpn-settings", h.GetVPNSettings)
		r.Get("/settings", h.GetSettings)
		r.Put("/settings", h.UpdateSettings)
		r.Put("/vpn", h.SetVPNStatus)
		r.Post("/vpn/restart", h.RestartVPN)
		r.Post("/restart", h.RestartContainer)
	})
}

const maxGluetunBodySize = 16 << 10

type gluetunSettingsResponse struct {
	ControlURL    string `json:"controlUrl"`
	AuthType      string `json:"authType"`
	Username      string `json:"username"`
	HasSecret     bool   `json:"hasSecret"`
	DockerHost    string `json:"dockerHost"`
	ContainerName string `json:"containerName"`
}

type gluetunSettingsRequest struct {
	ControlURL string `json:"controlUrl"`
	AuthType   string `json:"authType"`
	Username   string `json:"username"`
	// Secret is the API key or password. Omit it to keep the stored one; an
	// empty string clears it.
	Secret        *string `json:"secret"`
	DockerHost    string  `json:"dockerHost"`
	ContainerName string  `json:"containerName"`
}

func toGluetunSettingsResponse(s *models.GluetunSettings) gluetunSettingsResponse {
	return gluetunSettingsResponse{
		ControlURL:    s.ControlURL,
		AuthType:      s.AuthType,
		Username:      s.Username,
		HasSecret:     s.Secret != "",
		DockerHost:    s.DockerHost,
		ContainerName: s.ContainerName,
	}
}

// GetStatus handles GET /api/gluetun/status
func (h *GluetunHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.Status(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("gluetun: failed to load status")
		RespondError(w, http.StatusInternalServerError, "failed to load gluetun status")
		return
	}
	RespondJSON(w, http.StatusOK, status)
}

// GetLogs handles GET /api/gluetun/logs?tail=500
func (h *GluetunHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	tail := 500
	if raw := r.URL.Query().Get("tail"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 5000 {
			RespondError(w, http.StatusBadRequest, "tail must be between 1 and 5000")
			return
		}
		tail = value
	}

	lines, err := h.service.Logs(r.Context(), tail)
	if err != nil {
		RespondError(w, http.StatusBadGateway, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]any{"lines": lines})
}

// GetVPNSettings handles GET /api/gluetun/vpn-settings
func (h *GluetunHandler) GetVPNSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.VPNSettings(r.Context())
	if err != nil {
		RespondError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(settings)
}

// GetSettings handles GET /api/gluetun/settings
func (h *GluetunHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.Settings(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("gluetun: failed to load settings")
		RespondError(w, http.StatusInternalServerError, "failed to load gluetun settings")
		return
	}
	RespondJSON(w, http.StatusOK, toGluetunSettingsResponse(settings))
}

// UpdateSettings handles PUT /api/gluetun/settings
func (h *GluetunHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req gluetunSettingsRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxGluetunBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	current, err := h.service.Settings(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "failed to load gluetun settings")
		return
	}
	next := &models.GluetunSettings{
		ControlURL:    req.ControlURL,
		AuthType:      req.AuthType,
		Username:      req.Username,
		Secret:        current.Secret,
		DockerHost:    req.DockerHost,
		ContainerName: req.ContainerName,
	}
	if req.Secret != nil {
		next.Secret = *req.Secret
	}
	if next.AuthType == models.GluetunAuthNone {
		next.Username, next.Secret = "", ""
	}

	if err := h.service.SaveSettings(r.Context(), next); err != nil {
		if errors.Is(err, gluetun.ErrInvalidSettings) {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Error().Err(err).Msg("gluetun: failed to save settings")
		RespondError(w, http.StatusInternalServerError, "failed to save gluetun settings")
		return
	}
	RespondJSON(w, http.StatusOK, toGluetunSettingsResponse(next))
}

// SetVPNStatus handles PUT /api/gluetun/vpn with {"status":"running"|"stopped"}
func (h *GluetunHandler) SetVPNStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status string `json:"status"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxGluetunBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.SetVPNStatus(r.Context(), req.Status); err != nil {
		if errors.Is(err, gluetun.ErrInvalidSettings) {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(w, http.StatusBadGateway, err.Error())
		return
	}
	RespondJSON(w, http.StatusNoContent, nil)
}

// RestartVPN handles POST /api/gluetun/vpn/restart
func (h *GluetunHandler) RestartVPN(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RestartVPN(r.Context()); err != nil {
		RespondError(w, http.StatusBadGateway, err.Error())
		return
	}
	RespondJSON(w, http.StatusNoContent, nil)
}

// RestartContainer handles POST /api/gluetun/restart with {"withDependents":true}.
// The restart runs in the background; GET /status reports its progress.
func (h *GluetunHandler) RestartContainer(w http.ResponseWriter, r *http.Request) {
	req := struct {
		WithDependents bool `json:"withDependents"`
	}{WithDependents: true}
	r.Body = http.MaxBytesReader(w, r.Body, maxGluetunBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	state, err := h.service.RestartContainer(r.Context(), req.WithDependents)
	switch {
	case errors.Is(err, gluetun.ErrRestartRunning):
		RespondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, gluetun.ErrInvalidSettings):
		RespondError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		RespondError(w, http.StatusBadGateway, err.Error())
	default:
		RespondJSON(w, http.StatusAccepted, state)
	}
}
