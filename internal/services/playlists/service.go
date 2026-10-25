// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// Package playlists keeps playlists that an AI assistant wrote and copies
// them to Jellyfin, Plex, Spotify and YouTube Music. qui exports the music
// library as text for the assistant, reads its answer back, and finds each
// track on a service by artist and title. A sync makes the copy on a service
// equal to the playlist; a compare lists both sides so the user can choose
// what each side keeps.
package playlists

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gjhuerte/qui/internal/models"
	"github.com/gjhuerte/qui/internal/services/mediasync"
)

// PlaylistStore persists playlists and their tracks (models.PlaylistStore).
type PlaylistStore interface {
	ListPlaylists(ctx context.Context) ([]*models.Playlist, error)
	GetPlaylist(ctx context.Context, id int) (*models.Playlist, error)
	CreatePlaylist(ctx context.Context, p *models.Playlist) (*models.Playlist, error)
	UpdatePlaylist(ctx context.Context, p *models.Playlist) (*models.Playlist, error)
	DeletePlaylist(ctx context.Context, id int) error
}

// LinkStore persists the copies of a playlist on services (models.PlaylistStore).
type LinkStore interface {
	GetLink(ctx context.Context, playlistID, linkID int) (*models.PlaylistLink, error)
	CreateLink(ctx context.Context, l *models.PlaylistLink) (*models.PlaylistLink, error)
	SaveSyncResult(ctx context.Context, l *models.PlaylistLink) (*models.PlaylistLink, error)
	DeleteLink(ctx context.Context, playlistID, linkID int) error
}

// SettingsStore persists the Spotify and YouTube Music sign-ins (models.PlaylistStore).
type SettingsStore interface {
	GetServiceSettings(ctx context.Context, service models.PlaylistService) (*models.PlaylistServiceSettings, error)
	SaveServiceSettings(ctx context.Context, s *models.PlaylistServiceSettings) error
	DeleteServiceSettings(ctx context.Context, service models.PlaylistService) error
}

type Store interface {
	PlaylistStore
	LinkStore
	SettingsStore
}

// ServerStore looks up Jellyfin and Plex servers (models.MediaSyncStore).
type ServerStore interface {
	GetServer(ctx context.Context, id int) (*models.MediaServer, error)
}

const (
	requestTimeout = 30 * time.Second
	// Long enough to search a few hundred tracks on Spotify or YouTube Music.
	syncTimeout = 15 * time.Minute
	// messageTitles is how many missing titles the link message names.
	messageTitles = 10
)

// ErrInvalid is a validation error the handler maps to 400.
var ErrInvalid = errors.New("invalid input")

// RemotePlaylist is a playlist that already exists on a service.
type RemotePlaylist struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// RemoteItem is one entry of a playlist on a service. TrackID names the
// track; EntryID names its place in the playlist, which removal needs.
type RemoteItem struct {
	TrackID string
	EntryID string
	Artist  string
	Title   string
	Album   string
}

// target is one service account that holds playlists.
type target interface {
	// match finds each track on the service; "" marks a track it lacks.
	match(ctx context.Context, tracks []models.PlaylistTrack) ([]string, error)
	playlists(ctx context.Context) ([]RemotePlaylist, error)
	items(ctx context.Context, playlistID string) ([]RemoteItem, error)
	create(ctx context.Context, name, description string, trackIDs []string) (string, error)
	edit(ctx context.Context, playlistID string, add []string, remove []RemoteItem) error
}

type Service struct {
	store   Store
	servers ServerStore

	// Replaced in tests.
	http      *http.Client
	plexToken func(ctx context.Context, srv *models.MediaServer, userID string) (string, error)
	spotify   spotifyURLs
	ytBase    string

	mu sync.Mutex
	// spotifyStates holds the state of each Spotify sign-in started in the
	// last hour, so a pasted address must come from this qui.
	spotifyStates map[string]time.Time
}

func NewService(store Store, servers ServerStore) *Service {
	return &Service{
		store:         store,
		servers:       servers,
		http:          &http.Client{Timeout: requestTimeout},
		plexToken:     mediasync.PlexUserToken,
		spotify:       defaultSpotifyURLs,
		ytBase:        ytMusicBase,
		spotifyStates: map[string]time.Time{},
	}
}

// Playlists

func (s *Service) List(ctx context.Context) ([]*models.Playlist, error) {
	return s.store.ListPlaylists(ctx)
}

func (s *Service) Get(ctx context.Context, id int) (*models.Playlist, error) {
	return s.store.GetPlaylist(ctx, id)
}

// ImportInput is the pasted answer of an AI assistant. Name, when set,
// replaces the name the assistant chose.
type ImportInput struct {
	Text    string `json:"text"`
	Name    string `json:"name"`
	Request string `json:"request"`
}

func (s *Service) Import(ctx context.Context, in ImportInput) (*models.Playlist, error) {
	parsed, err := ParseAIAnswer(in.Text)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = parsed.Name
	}
	if name == "" {
		name = nameFromRequest(in.Request)
	}
	if name == "" {
		name = "AI playlist " + time.Now().Format("2006-01-02 15:04")
	}
	return s.store.CreatePlaylist(ctx, &models.Playlist{
		Name: name, Description: parsed.Description, Request: strings.TrimSpace(in.Request), Tracks: parsed.Tracks,
	})
}

// nameFromRequest names a playlist after what the user asked for, cut at a
// word near 60 characters.
func nameFromRequest(request string) string {
	name := strings.Join(strings.Fields(request), " ")
	if len(name) <= 60 {
		return name
	}
	if i := strings.LastIndex(name[:60], " "); i > 20 {
		return name[:i] + "…"
	}
	return name[:60] + "…"
}

// UpdateInput replaces the name, description and tracks of a playlist.
type UpdateInput struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Tracks      []models.PlaylistTrack `json:"tracks"`
}

func (s *Service) Update(ctx context.Context, id int, in UpdateInput) (*models.Playlist, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalid)
	}
	return s.store.UpdatePlaylist(ctx, &models.Playlist{
		ID: id, Name: name, Description: strings.TrimSpace(in.Description), Tracks: cleanTracks(in.Tracks),
	})
}

func (s *Service) Delete(ctx context.Context, id int) error {
	return s.store.DeletePlaylist(ctx, id)
}

// Accounts on services

// Account names a service account: for Jellyfin and Plex a server and an
// account on it; for Spotify and YouTube Music the signed-in account.
type Account struct {
	Service   models.PlaylistService `json:"service"`
	ServerID  *int                   `json:"serverId"`
	AccountID string                 `json:"accountId"`
}

func (a *Account) validate() error {
	if !a.Service.Valid() {
		return fmt.Errorf("%w: unknown service %q", ErrInvalid, a.Service)
	}
	if a.Service.OnMediaServer() && (a.ServerID == nil || strings.TrimSpace(a.AccountID) == "") {
		return fmt.Errorf("%w: pick a server and an account", ErrInvalid)
	}
	if !a.Service.OnMediaServer() {
		a.ServerID, a.AccountID = nil, ""
	}
	return nil
}

// target builds the client for an account.
func (s *Service) target(ctx context.Context, a Account) (target, error) {
	if err := a.validate(); err != nil {
		return nil, err
	}
	switch a.Service {
	case models.PlaylistJellyfin, models.PlaylistPlex:
		srv, err := s.servers.GetServer(ctx, *a.ServerID)
		if err != nil {
			return nil, err
		}
		if string(srv.Type) != string(a.Service) {
			return nil, fmt.Errorf("%w: %s is not a %s server", ErrInvalid, srv.Name, a.Service)
		}
		if a.Service == models.PlaylistJellyfin {
			return &jellyfin{srv: srv, userID: a.AccountID, http: s.http}, nil
		}
		token, err := s.plexToken(ctx, srv, a.AccountID)
		if err != nil {
			return nil, err
		}
		return &plex{srv: srv, token: token, http: s.http}, nil
	case models.PlaylistSpotify:
		return s.spotifyTarget(ctx)
	default:
		return s.ytMusicTarget(ctx)
	}
}

// ExportLibrary returns the music library of a Jellyfin or Plex account as
// text for an AI assistant.
func (s *Service) ExportLibrary(ctx context.Context, a Account) (string, error) {
	if !a.Service.OnMediaServer() {
		return "", fmt.Errorf("%w: export from a Jellyfin or Plex server", ErrInvalid)
	}
	t, err := s.target(ctx, a)
	if err != nil {
		return "", err
	}
	lib, ok := t.(interface {
		library(ctx context.Context) ([]Song, error)
	})
	if !ok {
		return "", fmt.Errorf("%w: this service has no library to export", ErrInvalid)
	}
	songs, err := lib.library(ctx)
	if err != nil {
		return "", err
	}
	if len(songs) == 0 {
		return "", fmt.Errorf("%w: no music found for this account", ErrInvalid)
	}
	srv, err := s.servers.GetServer(ctx, *a.ServerID)
	if err != nil {
		return "", err
	}
	return FormatLibrary(srv.Name, songs), nil
}

// RemotePlaylists lists the playlists that already exist on an account.
func (s *Service) RemotePlaylists(ctx context.Context, a Account) ([]RemotePlaylist, error) {
	t, err := s.target(ctx, a)
	if err != nil {
		return nil, err
	}
	return t.playlists(ctx)
}

// Links

// LinkInput adds a service to a playlist. RemoteID, when set, links a
// playlist that already exists there; empty creates one on the first sync.
type LinkInput struct {
	Account
	AccountName string `json:"accountName"`
	RemoteID    string `json:"remoteId"`
	RemoteName  string `json:"remoteName"`
}

func (s *Service) CreateLink(ctx context.Context, playlistID int, in LinkInput) (*models.PlaylistLink, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	if _, err := s.store.GetPlaylist(ctx, playlistID); err != nil {
		return nil, err
	}
	if in.Service.OnMediaServer() {
		if _, err := s.servers.GetServer(ctx, *in.ServerID); err != nil {
			return nil, err
		}
	} else {
		settings, err := s.store.GetServiceSettings(ctx, in.Service)
		if err != nil {
			return nil, err
		}
		if settings == nil {
			return nil, fmt.Errorf("%w: connect %s on the Playlist services page first", ErrInvalid, serviceName(in.Service))
		}
		in.AccountName = settings.AccountName
	}
	return s.store.CreateLink(ctx, &models.PlaylistLink{
		PlaylistID: playlistID, Service: in.Service, ServerID: in.ServerID, AccountID: in.AccountID,
		AccountName: strings.TrimSpace(in.AccountName), RemoteID: strings.TrimSpace(in.RemoteID), RemoteName: strings.TrimSpace(in.RemoteName),
	})
}

func (s *Service) DeleteLink(ctx context.Context, playlistID, linkID int) error {
	return s.store.DeleteLink(ctx, playlistID, linkID)
}

func serviceName(service models.PlaylistService) string {
	switch service {
	case models.PlaylistJellyfin:
		return "Jellyfin"
	case models.PlaylistPlex:
		return "Plex"
	case models.PlaylistSpotify:
		return "Spotify"
	default:
		return "YouTube Music"
	}
}

// open loads a playlist, one of its links and the client of the link's account.
func (s *Service) open(ctx context.Context, playlistID, linkID int) (*models.Playlist, *models.PlaylistLink, target, error) {
	p, err := s.store.GetPlaylist(ctx, playlistID)
	if err != nil {
		return nil, nil, nil, err
	}
	link, err := s.store.GetLink(ctx, playlistID, linkID)
	if err != nil {
		return nil, nil, nil, err
	}
	t, err := s.target(ctx, Account{Service: link.Service, ServerID: link.ServerID, AccountID: link.AccountID})
	if err != nil {
		return nil, nil, nil, err
	}
	return p, link, t, nil
}

// detach keeps a sync running when the browser tab closes, so a playlist is
// not left half written.
func detach(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), syncTimeout)
}

// SyncResult is the outcome of a sync: the updated link and the tracks the
// service does not have.
type SyncResult struct {
	Link    *models.PlaylistLink   `json:"link"`
	Missing []models.PlaylistTrack `json:"missing"`
}

// Sync makes the copy on the service equal to the playlist: it creates the
// copy on the first sync, then adds and removes tracks. Tracks the service
// lacks are skipped and reported.
func (s *Service) Sync(ctx context.Context, playlistID, linkID int) (*SyncResult, error) {
	ctx, cancel := detach(ctx)
	defer cancel()

	p, link, t, err := s.open(ctx, playlistID, linkID)
	if err != nil {
		return nil, err
	}
	ids, err := t.match(ctx, p.Tracks)
	if err != nil {
		return nil, err
	}

	var found []string
	var missing []models.PlaylistTrack
	seen := map[string]bool{}
	for i, id := range ids {
		switch {
		case id == "":
			missing = append(missing, p.Tracks[i])
		case !seen[id]:
			seen[id] = true
			found = append(found, id)
		}
	}

	if link.RemoteID == "" {
		if len(found) == 0 {
			return nil, fmt.Errorf("%w: none of the tracks were found on %s", ErrInvalid, serviceName(link.Service))
		}
		id, err := t.create(ctx, p.Name, p.Description, found)
		if err != nil {
			return nil, err
		}
		link.RemoteID, link.RemoteName = id, p.Name
	} else {
		current, err := t.items(ctx, link.RemoteID)
		if err != nil {
			return nil, err
		}
		// Duplicates on the service stay: Spotify removes by track, so
		// removing a duplicate would remove every copy.
		have := map[string]bool{}
		var remove []RemoteItem
		for _, item := range current {
			if !seen[item.TrackID] {
				remove = append(remove, item)
				continue
			}
			have[item.TrackID] = true
		}
		var add []string
		for _, id := range found {
			if !have[id] {
				add = append(add, id)
			}
		}
		if err := t.edit(ctx, link.RemoteID, add, remove); err != nil {
			return nil, err
		}
	}

	link.Matched, link.Missing = len(found), len(missing)
	link.Message = missingMessage(missing)
	saved, err := s.store.SaveSyncResult(ctx, link)
	if err != nil {
		return nil, err
	}
	if missing == nil {
		missing = []models.PlaylistTrack{}
	}
	return &SyncResult{Link: saved, Missing: missing}, nil
}

func missingMessage(missing []models.PlaylistTrack) string {
	if len(missing) == 0 {
		return ""
	}
	names := make([]string, 0, messageTitles)
	for i, t := range missing {
		if i == messageTitles {
			names = append(names, fmt.Sprintf("and %d more", len(missing)-messageTitles))
			break
		}
		names = append(names, trackName(t))
	}
	return "Not found: " + strings.Join(names, "; ")
}

func trackName(t models.PlaylistTrack) string {
	if t.Artist == "" {
		return t.Title
	}
	return t.Artist + " - " + t.Title
}

// CompareRow is one track in a compare. Local and Remote say where it is
// now; Keep is the user's choice on apply. TrackID is empty for a playlist
// track the service lacks.
type CompareRow struct {
	Key     string `json:"key"`
	Artist  string `json:"artist"`
	Title   string `json:"title"`
	Album   string `json:"album,omitempty"`
	TrackID string `json:"trackId,omitempty"`
	EntryID string `json:"entryId,omitempty"`
	Local   bool   `json:"local"`
	Remote  bool   `json:"remote"`
	Keep    bool   `json:"keep"`
}

// Compare lists the playlist tracks and the tracks of its copy on the
// service side by side. Keep starts true for every row.
func (s *Service) Compare(ctx context.Context, playlistID, linkID int) ([]CompareRow, error) {
	ctx, cancel := detach(ctx)
	defer cancel()

	p, link, t, err := s.open(ctx, playlistID, linkID)
	if err != nil {
		return nil, err
	}
	if link.RemoteID == "" {
		return nil, fmt.Errorf("%w: sync once to create the playlist on %s, or link an existing one", ErrInvalid, serviceName(link.Service))
	}
	ids, err := t.match(ctx, p.Tracks)
	if err != nil {
		return nil, err
	}
	items, err := t.items(ctx, link.RemoteID)
	if err != nil {
		return nil, err
	}
	return compareRows(p.Tracks, ids, items), nil
}

// compareRows pairs playlist tracks with remote items by track ID. Each
// remote item pairs with at most one track; the rest are remote-only rows.
func compareRows(tracks []models.PlaylistTrack, ids []string, items []RemoteItem) []CompareRow {
	byTrack := map[string][]int{}
	for i, item := range items {
		byTrack[item.TrackID] = append(byTrack[item.TrackID], i)
	}
	used := make([]bool, len(items))

	rows := make([]CompareRow, 0, len(tracks)+len(items))
	for i, t := range tracks {
		row := CompareRow{Key: fmt.Sprintf("l%d", i), Artist: t.Artist, Title: t.Title, Album: t.Album, TrackID: ids[i], Local: true, Keep: true}
		if ids[i] != "" {
			for _, j := range byTrack[ids[i]] {
				if !used[j] {
					used[j] = true
					row.Remote, row.EntryID = true, items[j].EntryID
					break
				}
			}
		}
		rows = append(rows, row)
	}
	for j, item := range items {
		if used[j] {
			continue
		}
		rows = append(rows, CompareRow{
			Key: fmt.Sprintf("r%d", j), Artist: item.Artist, Title: item.Title, Album: item.Album,
			TrackID: item.TrackID, EntryID: item.EntryID, Remote: true, Keep: true,
		})
	}
	return rows
}

// Apply writes the choices of a compare to both sides: the playlist keeps
// the rows marked Keep, in order; the service gains the kept rows it lacks
// and loses the rows not kept.
func (s *Service) Apply(ctx context.Context, playlistID, linkID int, rows []CompareRow) (*models.Playlist, error) {
	ctx, cancel := detach(ctx)
	defer cancel()

	p, link, t, err := s.open(ctx, playlistID, linkID)
	if err != nil {
		return nil, err
	}
	if link.RemoteID == "" {
		return nil, fmt.Errorf("%w: this link has no playlist on %s yet", ErrInvalid, serviceName(link.Service))
	}

	// A track kept in one row is not removed for another row: Spotify
	// removes by track, which would remove the kept copy too.
	keptRemote := map[string]bool{}
	for _, r := range rows {
		if r.Keep && r.Remote {
			keptRemote[r.TrackID] = true
		}
	}

	var add []string
	var remove []RemoteItem
	var tracks []models.PlaylistTrack
	matched, missing := 0, []models.PlaylistTrack{}
	for _, r := range rows {
		switch {
		case r.Keep && !r.Remote && r.TrackID != "" && !keptRemote[r.TrackID]:
			keptRemote[r.TrackID] = true
			add = append(add, r.TrackID)
		case !r.Keep && r.Remote && !keptRemote[r.TrackID]:
			remove = append(remove, RemoteItem{TrackID: r.TrackID, EntryID: r.EntryID})
		}
		if !r.Keep {
			continue
		}
		track := models.PlaylistTrack{Artist: r.Artist, Title: r.Title, Album: r.Album}
		tracks = append(tracks, track)
		if r.TrackID != "" {
			matched++
		} else {
			missing = append(missing, track)
		}
	}

	if err := t.edit(ctx, link.RemoteID, add, remove); err != nil {
		return nil, err
	}
	if _, err := s.store.UpdatePlaylist(ctx, &models.Playlist{ID: p.ID, Name: p.Name, Description: p.Description, Tracks: cleanTracks(tracks)}); err != nil {
		return nil, err
	}
	link.Matched, link.Missing, link.Message = matched, len(missing), missingMessage(missing)
	if _, err := s.store.SaveSyncResult(ctx, link); err != nil {
		return nil, err
	}
	return s.store.GetPlaylist(ctx, playlistID)
}
