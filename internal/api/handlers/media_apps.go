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
	"github.com/gjhuerte/qui/internal/services/mediaapps"
)

// MediaAppsHandler serves the media apps (Sonarr, Radarr, Prowlarr, Bazarr,
// Tautulli) on the Services page.
type MediaAppsHandler struct {
	service *mediaapps.Service
}

func NewMediaAppsHandler(service *mediaapps.Service) *MediaAppsHandler {
	return &MediaAppsHandler{service: service}
}

func (h *MediaAppsHandler) Routes(r chi.Router) {
	r.Route("/apps", func(r chi.Router) {
		r.Get("/status", h.GetStatus)
		r.Get("/movies", h.GetMovies)
		r.Get("/series", h.GetSeries)
		r.Get("/subtitles", h.GetSubtitles)
		r.Post("/test", h.Test)
		r.Get("/instances", h.List)
		r.Post("/instances", h.Create)
		r.Put("/instances/{id}", h.Update)
		r.Delete("/instances/{id}", h.Delete)
	})
}

const maxMediaAppsBodySize = 16 << 10

func decodeAppInput(w http.ResponseWriter, r *http.Request) (*appInputRequest, bool) {
	var req appInputRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxMediaAppsBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "invalid request body")
		return nil, false
	}
	return &req, true
}

type appInputRequest struct {
	mediaapps.Input
	// ID lets a test reuse the stored API key of an existing instance.
	ID int `json:"id"`
}

func appInstanceID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		RespondError(w, http.StatusBadRequest, "invalid instance id")
		return 0, false
	}
	return id, true
}

func respondAppError(w http.ResponseWriter, err error, action string) {
	switch {
	case errors.Is(err, mediaapps.ErrInvalid):
		RespondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, models.ErrAppInstanceNotFound):
		RespondError(w, http.StatusNotFound, err.Error())
	default:
		log.Error().Err(err).Msg("media apps: failed to " + action)
		RespondError(w, http.StatusInternalServerError, "failed to "+action)
	}
}

// GetStatus handles GET /api/apps/status?refresh=1
func (h *MediaAppsHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.service.Status(r.Context(), r.URL.Query().Get("refresh") == "1")
	if err != nil {
		respondAppError(w, err, "load app status")
		return
	}
	RespondJSON(w, http.StatusOK, statuses)
}

// Test handles POST /api/apps/test
func (h *MediaAppsHandler) Test(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeAppInput(w, r)
	if !ok {
		return
	}
	status, err := h.service.Test(r.Context(), req.ID, req.Input)
	if err != nil {
		respondAppError(w, err, "test the connection")
		return
	}
	RespondJSON(w, http.StatusOK, status)
}

// List handles GET /api/apps/instances
func (h *MediaAppsHandler) List(w http.ResponseWriter, r *http.Request) {
	instances, err := h.service.List(r.Context())
	if err != nil {
		respondAppError(w, err, "list apps")
		return
	}
	RespondJSON(w, http.StatusOK, instances)
}

// Create handles POST /api/apps/instances
func (h *MediaAppsHandler) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeAppInput(w, r)
	if !ok {
		return
	}
	inst, err := h.service.Create(r.Context(), req.Input)
	if err != nil {
		respondAppError(w, err, "add the app")
		return
	}
	RespondJSON(w, http.StatusCreated, inst)
}

// Update handles PUT /api/apps/instances/{id}. An empty apiKey keeps the stored key.
func (h *MediaAppsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := appInstanceID(w, r)
	if !ok {
		return
	}
	req, ok := decodeAppInput(w, r)
	if !ok {
		return
	}
	inst, err := h.service.Update(r.Context(), id, req.Input)
	if err != nil {
		respondAppError(w, err, "update the app")
		return
	}
	RespondJSON(w, http.StatusOK, inst)
}

// Delete handles DELETE /api/apps/instances/{id}
func (h *MediaAppsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := appInstanceID(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		respondAppError(w, err, "delete the app")
		return
	}
	RespondJSON(w, http.StatusNoContent, nil)
}

// GetMovies handles GET /api/apps/movies?refresh=1
func (h *MediaAppsHandler) GetMovies(w http.ResponseWriter, r *http.Request) {
	lib, err := h.service.Movies(r.Context(), r.URL.Query().Get("refresh") == "1")
	if err != nil {
		respondAppError(w, err, "load movies")
		return
	}
	RespondJSON(w, http.StatusOK, lib)
}

// GetSeries handles GET /api/apps/series?refresh=1
func (h *MediaAppsHandler) GetSeries(w http.ResponseWriter, r *http.Request) {
	lib, err := h.service.Series(r.Context(), r.URL.Query().Get("refresh") == "1")
	if err != nil {
		respondAppError(w, err, "load series")
		return
	}
	RespondJSON(w, http.StatusOK, lib)
}

// GetSubtitles handles GET /api/apps/subtitles?refresh=1
func (h *MediaAppsHandler) GetSubtitles(w http.ResponseWriter, r *http.Request) {
	lib, err := h.service.WantedSubtitles(r.Context(), r.URL.Query().Get("refresh") == "1")
	if err != nil {
		respondAppError(w, err, "load wanted subtitles")
		return
	}
	RespondJSON(w, http.StatusOK, lib)
}
