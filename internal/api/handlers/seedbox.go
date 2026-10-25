// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/gjhuerte/qui/internal/services/seedbox"
)

// SeedboxHandler serves the MyAnonamouse dynamic seedbox page.
type SeedboxHandler struct {
	service *seedbox.Service
}

func NewSeedboxHandler(service *seedbox.Service) *SeedboxHandler {
	return &SeedboxHandler{service: service}
}

func (h *SeedboxHandler) Routes(r chi.Router) {
	r.Route("/seedbox", func(r chi.Router) {
		r.Get("/", h.GetStatus)
		r.Put("/mam-id", h.SetMamID)
		r.Delete("/mam-id", h.ClearMamID)
		r.Put("/settings", h.UpdateSettings)
		r.Post("/sync", h.Sync)
	})
}

const maxSeedboxBodySize = 16 << 10

// GetStatus handles GET /api/seedbox
func (h *SeedboxHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.Status(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("seedbox: failed to load status")
		RespondError(w, http.StatusInternalServerError, "failed to load seedbox status")
		return
	}
	RespondJSON(w, http.StatusOK, status)
}

// SetMamID handles PUT /api/seedbox/mam-id. It saves the cookie and contacts MAM.
func (h *SeedboxHandler) SetMamID(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MamID string `json:"mamId"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSeedboxBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	contact, err := h.service.SetMamID(r.Context(), req.MamID)
	if errors.Is(err, seedbox.ErrInvalidMamID) {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("seedbox: failed to save mam_id")
		RespondError(w, http.StatusInternalServerError, "failed to save mam_id")
		return
	}
	RespondJSON(w, http.StatusOK, contact)
}

// ClearMamID handles DELETE /api/seedbox/mam-id
func (h *SeedboxHandler) ClearMamID(w http.ResponseWriter, r *http.Request) {
	if err := h.service.ClearMamID(r.Context()); err != nil {
		log.Error().Err(err).Msg("seedbox: failed to remove mam_id")
		RespondError(w, http.StatusInternalServerError, "failed to remove mam_id")
		return
	}
	RespondJSON(w, http.StatusNoContent, nil)
}

// UpdateSettings handles PUT /api/seedbox/settings
func (h *SeedboxHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled         bool `json:"enabled"`
		IntervalSeconds int  `json:"intervalSeconds"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSeedboxBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.service.UpdateSchedule(r.Context(), req.Enabled, req.IntervalSeconds)
	if errors.Is(err, seedbox.ErrInvalidInterval) {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("seedbox: failed to save settings")
		RespondError(w, http.StatusInternalServerError, "failed to save seedbox settings")
		return
	}
	h.GetStatus(w, r)
}

// Sync handles POST /api/seedbox/sync
func (h *SeedboxHandler) Sync(w http.ResponseWriter, r *http.Request) {
	contact, err := h.service.Sync(r.Context(), seedbox.SourceManual)
	if err != nil {
		log.Error().Err(err).Msg("seedbox: manual update failed")
		RespondError(w, http.StatusInternalServerError, "failed to contact MAM")
		return
	}
	RespondJSON(w, http.StatusOK, contact)
}
