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
	"github.com/gjhuerte/qui/internal/services/mediasync"
)

// MediaSyncHandler serves the media servers (Jellyfin, Plex) and the
// watched-state sync between them.
type MediaSyncHandler struct {
	service *mediasync.Service
}

func NewMediaSyncHandler(service *mediasync.Service) *MediaSyncHandler {
	return &MediaSyncHandler{service: service}
}

func (h *MediaSyncHandler) Routes(r chi.Router) {
	r.Route("/media-sync", func(r chi.Router) {
		r.Get("/status", h.Status)
		r.Get("/servers", h.ListServers)
		r.Post("/servers", h.CreateServer)
		r.Post("/servers/test", h.TestServer)
		r.Put("/servers/{id}", h.UpdateServer)
		r.Delete("/servers/{id}", h.DeleteServer)
		r.Get("/servers/{id}/users", h.ServerUsers)
		r.Get("/pairs", h.ListPairs)
		r.Post("/pairs", h.CreatePair)
		r.Put("/pairs/{id}", h.UpdatePair)
		r.Delete("/pairs/{id}", h.DeletePair)
		r.Get("/pairs/{id}/users", h.UserMaps)
		r.Put("/pairs/{id}/users", h.SetUserMaps)
		r.Post("/pairs/{id}/run", h.RunNow)
		r.Post("/pairs/{id}/cancel", h.Cancel)
		r.Get("/runs", h.ListRuns)
		r.Get("/runs/{id}", h.GetRun)
	})
}

const maxMediaSyncBodySize = 64 << 10

func decodeMediaSync(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxMediaSyncBodySize)
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		RespondError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func mediaSyncID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		RespondError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func respondMediaSyncError(w http.ResponseWriter, err error, action string) {
	switch {
	case errors.Is(err, mediasync.ErrInvalid):
		RespondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, mediasync.ErrRunning):
		RespondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, models.ErrMediaServerNotFound), errors.Is(err, models.ErrSyncPairNotFound), errors.Is(err, models.ErrSyncRunNotFound):
		RespondError(w, http.StatusNotFound, err.Error())
	default:
		log.Error().Err(err).Msg("media sync: failed to " + action)
		RespondError(w, http.StatusInternalServerError, "failed to "+action)
	}
}

// Status handles GET /api/media-sync/status?refresh=1
func (h *MediaSyncHandler) Status(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.service.Status(r.Context(), r.URL.Query().Get("refresh") == "1")
	if err != nil {
		respondMediaSyncError(w, err, "load server status")
		return
	}
	RespondJSON(w, http.StatusOK, statuses)
}

// ListServers handles GET /api/media-sync/servers
func (h *MediaSyncHandler) ListServers(w http.ResponseWriter, r *http.Request) {
	servers, err := h.service.ListServers(r.Context())
	if err != nil {
		respondMediaSyncError(w, err, "list servers")
		return
	}
	RespondJSON(w, http.StatusOK, servers)
}

type serverTestRequest struct {
	mediasync.ServerInput
	// ID lets a test reuse the stored token of an existing server.
	ID int `json:"id"`
}

// CreateServer handles POST /api/media-sync/servers
func (h *MediaSyncHandler) CreateServer(w http.ResponseWriter, r *http.Request) {
	var in mediasync.ServerInput
	if !decodeMediaSync(w, r, &in) {
		return
	}
	srv, err := h.service.CreateServer(r.Context(), in)
	if err != nil {
		respondMediaSyncError(w, err, "add the server")
		return
	}
	RespondJSON(w, http.StatusCreated, srv)
}

// TestServer handles POST /api/media-sync/servers/test
func (h *MediaSyncHandler) TestServer(w http.ResponseWriter, r *http.Request) {
	var req serverTestRequest
	if !decodeMediaSync(w, r, &req) {
		return
	}
	st, err := h.service.TestServer(r.Context(), req.ID, req.ServerInput)
	if err != nil {
		respondMediaSyncError(w, err, "test the connection")
		return
	}
	RespondJSON(w, http.StatusOK, st)
}

// UpdateServer handles PUT /api/media-sync/servers/{id}. An empty token keeps the stored one.
func (h *MediaSyncHandler) UpdateServer(w http.ResponseWriter, r *http.Request) {
	id, ok := mediaSyncID(w, r)
	if !ok {
		return
	}
	var in mediasync.ServerInput
	if !decodeMediaSync(w, r, &in) {
		return
	}
	srv, err := h.service.UpdateServer(r.Context(), id, in)
	if err != nil {
		respondMediaSyncError(w, err, "update the server")
		return
	}
	RespondJSON(w, http.StatusOK, srv)
}

// DeleteServer handles DELETE /api/media-sync/servers/{id}
func (h *MediaSyncHandler) DeleteServer(w http.ResponseWriter, r *http.Request) {
	id, ok := mediaSyncID(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteServer(r.Context(), id); err != nil {
		respondMediaSyncError(w, err, "delete the server")
		return
	}
	RespondJSON(w, http.StatusNoContent, nil)
}

// ServerUsers handles GET /api/media-sync/servers/{id}/users
func (h *MediaSyncHandler) ServerUsers(w http.ResponseWriter, r *http.Request) {
	id, ok := mediaSyncID(w, r)
	if !ok {
		return
	}
	users, err := h.service.Users(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrMediaServerNotFound) {
			respondMediaSyncError(w, err, "list accounts")
			return
		}
		// The server or plex.tv answered with an error: show it as is.
		RespondError(w, http.StatusBadGateway, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, users)
}

// ListPairs handles GET /api/media-sync/pairs
func (h *MediaSyncHandler) ListPairs(w http.ResponseWriter, r *http.Request) {
	pairs, err := h.service.ListPairs(r.Context())
	if err != nil {
		respondMediaSyncError(w, err, "list sync pairs")
		return
	}
	RespondJSON(w, http.StatusOK, pairs)
}

// CreatePair handles POST /api/media-sync/pairs
func (h *MediaSyncHandler) CreatePair(w http.ResponseWriter, r *http.Request) {
	var in mediasync.PairInput
	if !decodeMediaSync(w, r, &in) {
		return
	}
	pair, err := h.service.CreatePair(r.Context(), in)
	if err != nil {
		respondMediaSyncError(w, err, "add the sync pair")
		return
	}
	RespondJSON(w, http.StatusCreated, pair)
}

// UpdatePair handles PUT /api/media-sync/pairs/{id}. The servers stay as created.
func (h *MediaSyncHandler) UpdatePair(w http.ResponseWriter, r *http.Request) {
	id, ok := mediaSyncID(w, r)
	if !ok {
		return
	}
	var in mediasync.PairInput
	if !decodeMediaSync(w, r, &in) {
		return
	}
	pair, err := h.service.UpdatePair(r.Context(), id, in)
	if err != nil {
		respondMediaSyncError(w, err, "update the sync pair")
		return
	}
	RespondJSON(w, http.StatusOK, pair)
}

// DeletePair handles DELETE /api/media-sync/pairs/{id}
func (h *MediaSyncHandler) DeletePair(w http.ResponseWriter, r *http.Request) {
	id, ok := mediaSyncID(w, r)
	if !ok {
		return
	}
	if err := h.service.DeletePair(r.Context(), id); err != nil {
		respondMediaSyncError(w, err, "delete the sync pair")
		return
	}
	RespondJSON(w, http.StatusNoContent, nil)
}

// UserMaps handles GET /api/media-sync/pairs/{id}/users
func (h *MediaSyncHandler) UserMaps(w http.ResponseWriter, r *http.Request) {
	id, ok := mediaSyncID(w, r)
	if !ok {
		return
	}
	maps, err := h.service.UserMaps(r.Context(), id)
	if err != nil {
		respondMediaSyncError(w, err, "load the account maps")
		return
	}
	RespondJSON(w, http.StatusOK, maps)
}

// SetUserMaps handles PUT /api/media-sync/pairs/{id}/users with the full list.
func (h *MediaSyncHandler) SetUserMaps(w http.ResponseWriter, r *http.Request) {
	id, ok := mediaSyncID(w, r)
	if !ok {
		return
	}
	var maps []models.SyncUserMap
	if !decodeMediaSync(w, r, &maps) {
		return
	}
	saved, err := h.service.SetUserMaps(r.Context(), id, maps)
	if err != nil {
		respondMediaSyncError(w, err, "save the account maps")
		return
	}
	RespondJSON(w, http.StatusOK, saved)
}

// RunNow handles POST /api/media-sync/pairs/{id}/run. The sync continues in
// the background; the response is the new run.
func (h *MediaSyncHandler) RunNow(w http.ResponseWriter, r *http.Request) {
	id, ok := mediaSyncID(w, r)
	if !ok {
		return
	}
	run, err := h.service.RunNow(r.Context(), id)
	if err != nil {
		respondMediaSyncError(w, err, "start the sync")
		return
	}
	RespondJSON(w, http.StatusAccepted, run)
}

// Cancel handles POST /api/media-sync/pairs/{id}/cancel
func (h *MediaSyncHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := mediaSyncID(w, r)
	if !ok {
		return
	}
	if err := h.service.Cancel(id); err != nil {
		respondMediaSyncError(w, err, "cancel the sync")
		return
	}
	RespondJSON(w, http.StatusNoContent, nil)
}

// ListRuns handles GET /api/media-sync/runs?pairId=&limit=
func (h *MediaSyncHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	pairID, _ := strconv.Atoi(r.URL.Query().Get("pairId"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	runs, err := h.service.ListRuns(r.Context(), pairID, limit)
	if err != nil {
		respondMediaSyncError(w, err, "list sync runs")
		return
	}
	RespondJSON(w, http.StatusOK, runs)
}

// GetRun handles GET /api/media-sync/runs/{id} and includes the marked titles.
func (h *MediaSyncHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	id, ok := mediaSyncID(w, r)
	if !ok {
		return
	}
	run, err := h.service.GetRun(r.Context(), int64(id))
	if err != nil {
		respondMediaSyncError(w, err, "load the sync run")
		return
	}
	RespondJSON(w, http.StatusOK, run)
}
