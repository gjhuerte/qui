// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/gjhuerte/qui/internal/models"
	"github.com/gjhuerte/qui/internal/services/playlists"
)

// PlaylistsHandler serves AI-made playlists and their copies on Jellyfin,
// Plex, Spotify and YouTube Music.
type PlaylistsHandler struct {
	service *playlists.Service
}

func NewPlaylistsHandler(service *playlists.Service) *PlaylistsHandler {
	return &PlaylistsHandler{service: service}
}

func (h *PlaylistsHandler) Routes(r chi.Router) {
	r.Route("/playlists", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/import", h.Import)
		r.Get("/library", h.ExportLibrary)
		r.Get("/remote", h.RemotePlaylists)
		r.Get("/services", h.Services)
		r.Put("/services/spotify", h.SaveSpotify)
		r.Post("/services/spotify/authorize", h.SpotifyAuthorize)
		r.Post("/services/spotify/connect", h.SpotifyConnect)
		r.Get("/services/spotify/callback", h.SpotifyCallback)
		r.Put("/services/ytmusic", h.SaveYTMusic)
		r.Delete("/services/{service}", h.Disconnect)
		r.Get("/{id}", h.Get)
		r.Put("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)
		r.Post("/{id}/links", h.CreateLink)
		r.Delete("/{id}/links/{linkId}", h.DeleteLink)
		r.Post("/{id}/links/{linkId}/sync", h.Sync)
		r.Post("/{id}/links/{linkId}/compare", h.Compare)
		r.Post("/{id}/links/{linkId}/apply", h.Apply)
	})
}

// A pasted AI answer or browser headers can be long.
const maxPlaylistBodySize = 2 << 20

func decodePlaylistBody(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxPlaylistBodySize)
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		RespondError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func playlistParam(w http.ResponseWriter, r *http.Request, name string) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, name))
	if err != nil || id <= 0 {
		RespondError(w, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return id, true
}

func playlistAndLink(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	id, ok := playlistParam(w, r, "id")
	if !ok {
		return 0, 0, false
	}
	linkID, ok := playlistParam(w, r, "linkId")
	return id, linkID, ok
}

// respondPlaylistError maps errors to statuses. An error from a service
// (Spotify, a server, plex.tv) is shown as is with 502, so the user reads
// what the service said.
func respondPlaylistError(w http.ResponseWriter, err error, action string) {
	switch {
	case errors.Is(err, playlists.ErrInvalid):
		RespondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, models.ErrPlaylistNotFound), errors.Is(err, models.ErrPlaylistLinkNotFound), errors.Is(err, models.ErrMediaServerNotFound):
		RespondError(w, http.StatusNotFound, err.Error())
	default:
		log.Warn().Err(err).Msg("playlists: failed to " + action)
		RespondError(w, http.StatusBadGateway, "failed to "+action+": "+err.Error())
	}
}

// accountFromQuery reads ?service=&serverId=&accountId=.
func accountFromQuery(r *http.Request) playlists.Account {
	q := r.URL.Query()
	a := playlists.Account{Service: models.PlaylistService(q.Get("service")), AccountID: q.Get("accountId")}
	if id, err := strconv.Atoi(q.Get("serverId")); err == nil {
		a.ServerID = &id
	}
	return a
}

// List handles GET /api/playlists
func (h *PlaylistsHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.Context())
	if err != nil {
		respondPlaylistError(w, err, "list playlists")
		return
	}
	RespondJSON(w, http.StatusOK, list)
}

// Import handles POST /api/playlists/import with the answer of an AI assistant.
func (h *PlaylistsHandler) Import(w http.ResponseWriter, r *http.Request) {
	var in playlists.ImportInput
	if !decodePlaylistBody(w, r, &in) {
		return
	}
	p, err := h.service.Import(r.Context(), in)
	if err != nil {
		respondPlaylistError(w, err, "import the playlist")
		return
	}
	RespondJSON(w, http.StatusCreated, p)
}

// ExportLibrary handles GET /api/playlists/library?service=&serverId=&accountId=
// and answers with a text file to attach to an AI chat.
func (h *PlaylistsHandler) ExportLibrary(w http.ResponseWriter, r *http.Request) {
	text, err := h.service.ExportLibrary(r.Context(), accountFromQuery(r))
	if err != nil {
		respondPlaylistError(w, err, "export the library")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="music-library-%s.txt"`, time.Now().Format("2006-01-02")))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(text))
}

// RemotePlaylists handles GET /api/playlists/remote?service=&serverId=&accountId=
func (h *PlaylistsHandler) RemotePlaylists(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.RemotePlaylists(r.Context(), accountFromQuery(r))
	if err != nil {
		respondPlaylistError(w, err, "list the playlists on the service")
		return
	}
	RespondJSON(w, http.StatusOK, list)
}

// Get handles GET /api/playlists/{id}
func (h *PlaylistsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := playlistParam(w, r, "id")
	if !ok {
		return
	}
	p, err := h.service.Get(r.Context(), id)
	if err != nil {
		respondPlaylistError(w, err, "load the playlist")
		return
	}
	RespondJSON(w, http.StatusOK, p)
}

// Update handles PUT /api/playlists/{id}
func (h *PlaylistsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := playlistParam(w, r, "id")
	if !ok {
		return
	}
	var in playlists.UpdateInput
	if !decodePlaylistBody(w, r, &in) {
		return
	}
	p, err := h.service.Update(r.Context(), id, in)
	if err != nil {
		respondPlaylistError(w, err, "save the playlist")
		return
	}
	RespondJSON(w, http.StatusOK, p)
}

// Delete handles DELETE /api/playlists/{id}. Copies on services stay.
func (h *PlaylistsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := playlistParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		respondPlaylistError(w, err, "delete the playlist")
		return
	}
	RespondJSON(w, http.StatusNoContent, nil)
}

// CreateLink handles POST /api/playlists/{id}/links
func (h *PlaylistsHandler) CreateLink(w http.ResponseWriter, r *http.Request) {
	id, ok := playlistParam(w, r, "id")
	if !ok {
		return
	}
	var in playlists.LinkInput
	if !decodePlaylistBody(w, r, &in) {
		return
	}
	link, err := h.service.CreateLink(r.Context(), id, in)
	if err != nil {
		respondPlaylistError(w, err, "add the service")
		return
	}
	RespondJSON(w, http.StatusCreated, link)
}

// DeleteLink handles DELETE /api/playlists/{id}/links/{linkId}. The copy on the service stays.
func (h *PlaylistsHandler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	id, linkID, ok := playlistAndLink(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteLink(r.Context(), id, linkID); err != nil {
		respondPlaylistError(w, err, "remove the service")
		return
	}
	RespondJSON(w, http.StatusNoContent, nil)
}

// Sync handles POST /api/playlists/{id}/links/{linkId}/sync
func (h *PlaylistsHandler) Sync(w http.ResponseWriter, r *http.Request) {
	id, linkID, ok := playlistAndLink(w, r)
	if !ok {
		return
	}
	res, err := h.service.Sync(r.Context(), id, linkID)
	if err != nil {
		respondPlaylistError(w, err, "sync the playlist")
		return
	}
	RespondJSON(w, http.StatusOK, res)
}

// Compare handles POST /api/playlists/{id}/links/{linkId}/compare. It changes nothing.
func (h *PlaylistsHandler) Compare(w http.ResponseWriter, r *http.Request) {
	id, linkID, ok := playlistAndLink(w, r)
	if !ok {
		return
	}
	rows, err := h.service.Compare(r.Context(), id, linkID)
	if err != nil {
		respondPlaylistError(w, err, "compare the playlists")
		return
	}
	RespondJSON(w, http.StatusOK, rows)
}

// Apply handles POST /api/playlists/{id}/links/{linkId}/apply with the compare rows and the user's choices.
func (h *PlaylistsHandler) Apply(w http.ResponseWriter, r *http.Request) {
	id, linkID, ok := playlistAndLink(w, r)
	if !ok {
		return
	}
	var body struct {
		Rows []playlists.CompareRow `json:"rows"`
	}
	if !decodePlaylistBody(w, r, &body) {
		return
	}
	p, err := h.service.Apply(r.Context(), id, linkID, body.Rows)
	if err != nil {
		respondPlaylistError(w, err, "apply the changes")
		return
	}
	RespondJSON(w, http.StatusOK, p)
}

// Services handles GET /api/playlists/services
func (h *PlaylistsHandler) Services(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.Services(r.Context())
	if err != nil {
		respondPlaylistError(w, err, "load the services")
		return
	}
	RespondJSON(w, http.StatusOK, status)
}

// SaveSpotify handles PUT /api/playlists/services/spotify. An empty secret keeps the stored one.
func (h *PlaylistsHandler) SaveSpotify(w http.ResponseWriter, r *http.Request) {
	var body playlists.SpotifyApp
	if !decodePlaylistBody(w, r, &body) {
		return
	}
	if err := h.service.SaveSpotifyApp(r.Context(), body); err != nil {
		respondPlaylistError(w, err, "save the Spotify app")
		return
	}
	h.Services(w, r)
}

// SpotifyAuthorize handles POST /api/playlists/services/spotify/authorize
func (h *PlaylistsHandler) SpotifyAuthorize(w http.ResponseWriter, r *http.Request) {
	authURL, err := h.service.SpotifyAuthorizeURL(r.Context())
	if err != nil {
		respondPlaylistError(w, err, "start the Spotify sign-in")
		return
	}
	RespondJSON(w, http.StatusOK, map[string]string{"url": authURL})
}

// SpotifyConnect handles POST /api/playlists/services/spotify/connect with the address the browser landed on.
func (h *PlaylistsHandler) SpotifyConnect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RedirectURL string `json:"redirectUrl"`
	}
	if !decodePlaylistBody(w, r, &body) {
		return
	}
	if _, err := h.service.ConnectSpotify(r.Context(), body.RedirectURL); err != nil {
		respondPlaylistError(w, err, "sign in to Spotify")
		return
	}
	h.Services(w, r)
}

// spotifyCallbackToPage leads from .../api/playlists/services/spotify/callback
// to the Services page, whatever base URL qui runs under.
const spotifyCallbackToPage = "../../../../services?tab=playlist-services"

// SpotifyCallback handles GET /api/playlists/services/spotify/callback, the
// redirect URI when it is qui's own address. Spotify sends the browser here
// after the sign-in; qui finishes it and returns to the Services page.
func (h *PlaylistsHandler) SpotifyCallback(w http.ResponseWriter, r *http.Request) {
	target := spotifyCallbackToPage
	if _, err := h.service.ConnectSpotify(r.Context(), "?"+r.URL.RawQuery); err != nil {
		log.Warn().Err(err).Msg("playlists: Spotify sign-in failed")
		target += "&spotifyError=" + url.QueryEscape(err.Error())
	}
	// A relative Location, resolved by the browser against the address it
	// used: http.Redirect would resolve it against r.URL.Path, which lacks
	// the base URL when qui runs under one.
	w.Header().Set("Location", target)
	w.WriteHeader(http.StatusFound)
}

// SaveYTMusic handles PUT /api/playlists/services/ytmusic with headers copied from the browser.
func (h *PlaylistsHandler) SaveYTMusic(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Headers string `json:"headers"`
	}
	if !decodePlaylistBody(w, r, &body) {
		return
	}
	if _, err := h.service.SaveYTMusic(r.Context(), body.Headers); err != nil {
		respondPlaylistError(w, err, "connect YouTube Music")
		return
	}
	h.Services(w, r)
}

// Disconnect handles DELETE /api/playlists/services/{service}
func (h *PlaylistsHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	service := models.PlaylistService(strings.ToLower(chi.URLParam(r, "service")))
	if err := h.service.Disconnect(r.Context(), service); err != nil {
		respondPlaylistError(w, err, "disconnect the service")
		return
	}
	RespondJSON(w, http.StatusNoContent, nil)
}
