// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/gjhuerte/qui/internal/update"
)

type restartRequester interface {
	TryLock() bool
	Unlock()
	Request() error
}

// SystemHandler serves the Restart action.
type SystemHandler struct {
	availability update.Availability
	restarter    restartRequester
}

func NewSystemHandler(availability update.Availability, restarter restartRequester) *SystemHandler {
	return &SystemHandler{availability: availability, restarter: restarter}
}

// Restart checks the binary and returns 202. The serve loop then runs the
// graceful shutdown and restarts qui in place.
func (h *SystemHandler) Restart(w http.ResponseWriter, _ *http.Request) {
	if !h.availability.Restart {
		RespondError(w, http.StatusForbidden, "Restart is not available")
		return
	}
	if !h.restarter.TryLock() {
		RespondError(w, http.StatusConflict, "A restart is already running")
		return
	}
	if err := h.restarter.Request(); err != nil {
		h.restarter.Unlock()
		log.Error().Err(err).Msg("refused restart")
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
