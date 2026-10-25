// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gjhuerte/qui/internal/dbinterface"
)

// PlaylistService is a place a playlist can be copied to.
type PlaylistService string

const (
	PlaylistJellyfin PlaylistService = "jellyfin"
	PlaylistPlex     PlaylistService = "plex"
	PlaylistSpotify  PlaylistService = "spotify"
	PlaylistYTMusic  PlaylistService = "ytmusic"
)

func (s PlaylistService) Valid() bool {
	switch s {
	case PlaylistJellyfin, PlaylistPlex, PlaylistSpotify, PlaylistYTMusic:
		return true
	}
	return false
}

// OnMediaServer reports whether the service is one of the media servers
// stored in media_servers.
func (s PlaylistService) OnMediaServer() bool {
	return s == PlaylistJellyfin || s == PlaylistPlex
}

var (
	ErrPlaylistNotFound     = errors.New("playlist not found")
	ErrPlaylistLinkNotFound = errors.New("playlist link not found")
)

type PlaylistTrack struct {
	Artist string `json:"artist"`
	Title  string `json:"title"`
	Album  string `json:"album,omitempty"`
}

// PlaylistLink is the copy of a playlist on one service. RemoteID is empty
// until the first sync creates the playlist there.
type PlaylistLink struct {
	ID         int             `json:"id"`
	PlaylistID int             `json:"playlistId"`
	Service    PlaylistService `json:"service"`
	// ServerID and AccountID are set for Jellyfin and Plex.
	ServerID     *int       `json:"serverId,omitempty"`
	AccountID    string     `json:"accountId,omitempty"`
	AccountName  string     `json:"accountName,omitempty"`
	RemoteID     string     `json:"remoteId,omitempty"`
	RemoteName   string     `json:"remoteName,omitempty"`
	Matched      int        `json:"matched"`
	Missing      int        `json:"missing"`
	Message      string     `json:"message,omitempty"`
	LastSyncedAt *time.Time `json:"lastSyncedAt,omitempty"`
}

type Playlist struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Request is what the user asked the AI assistant for.
	Request    string          `json:"request,omitempty"`
	TrackCount int             `json:"trackCount"`
	Tracks     []PlaylistTrack `json:"tracks,omitempty"`
	Links      []PlaylistLink  `json:"links"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}

// PlaylistServiceSettings is the sign-in of Spotify or YouTube Music.
// Settings is the decrypted JSON object and never leaves the server.
type PlaylistServiceSettings struct {
	Service     PlaylistService
	Settings    string
	AccountName string
	UpdatedAt   time.Time
}

type PlaylistStore struct {
	db     dbinterface.Querier
	cipher *CredentialCipher
}

func NewPlaylistStore(db dbinterface.Querier, encryptionKey []byte, opts ...CredentialCipherOption) (*PlaylistStore, error) {
	cipher, err := NewCredentialCipher(encryptionKey, opts...)
	if err != nil {
		return nil, err
	}
	return &PlaylistStore{db: db, cipher: cipher}, nil
}

// ListPlaylists returns every playlist with its track count and links, newest first.
func (s *PlaylistStore) ListPlaylists(ctx context.Context) ([]*Playlist, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.name, p.description, p.request, p.created_at, p.updated_at,
			(SELECT COUNT(*) FROM playlist_tracks t WHERE t.playlist_id = p.id)
		FROM playlists p
		ORDER BY p.created_at DESC, p.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*Playlist{}
	byID := map[int]*Playlist{}
	for rows.Next() {
		p := &Playlist{Links: []PlaylistLink{}}
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Request, &p.CreatedAt, &p.UpdatedAt, &p.TrackCount); err != nil {
			return nil, err
		}
		out = append(out, p)
		byID[p.ID] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	links, err := s.listLinks(ctx, `SELECT `+linkColumns+` FROM playlist_links ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for _, l := range links {
		if p := byID[l.PlaylistID]; p != nil {
			p.Links = append(p.Links, l)
		}
	}
	return out, nil
}

// GetPlaylist returns a playlist with its tracks and links.
func (s *PlaylistStore) GetPlaylist(ctx context.Context, id int) (*Playlist, error) {
	p := &Playlist{Tracks: []PlaylistTrack{}}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, request, created_at, updated_at FROM playlists WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &p.Description, &p.Request, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPlaylistNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT artist, title, album FROM playlist_tracks WHERE playlist_id = ? ORDER BY position, id
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t PlaylistTrack
		if err := rows.Scan(&t.Artist, &t.Title, &t.Album); err != nil {
			return nil, err
		}
		p.Tracks = append(p.Tracks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	p.TrackCount = len(p.Tracks)

	p.Links, err = s.listLinks(ctx, `SELECT `+linkColumns+` FROM playlist_links WHERE playlist_id = ? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// CreatePlaylist stores a playlist with its tracks and returns it.
func (s *PlaylistStore) CreatePlaylist(ctx context.Context, p *Playlist) (*Playlist, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var id int
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO playlists (name, description, request) VALUES (?, ?, ?) RETURNING id
	`, p.Name, p.Description, p.Request).Scan(&id); err != nil {
		return nil, err
	}
	if err := insertTracks(ctx, tx, id, p.Tracks); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetPlaylist(ctx, id)
}

// UpdatePlaylist replaces the name, description and tracks.
func (s *PlaylistStore) UpdatePlaylist(ctx context.Context, p *Playlist) (*Playlist, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE playlists SET name = ?, description = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, p.Name, p.Description, p.ID)
	if err != nil {
		return nil, err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return nil, ErrPlaylistNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM playlist_tracks WHERE playlist_id = ?`, p.ID); err != nil {
		return nil, err
	}
	if err := insertTracks(ctx, tx, p.ID, p.Tracks); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetPlaylist(ctx, p.ID)
}

func insertTracks(ctx context.Context, tx dbinterface.TxQuerier, playlistID int, tracks []PlaylistTrack) error {
	for i, t := range tracks {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO playlist_tracks (playlist_id, position, artist, title, album) VALUES (?, ?, ?, ?, ?)
		`, playlistID, i, t.Artist, t.Title, t.Album); err != nil {
			return err
		}
	}
	return nil
}

// DeletePlaylist deletes a playlist with its tracks and links. The copies on
// the services stay.
func (s *PlaylistStore) DeletePlaylist(ctx context.Context, id int) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM playlists WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrPlaylistNotFound
	}
	return nil
}

// Links

const linkColumns = `id, playlist_id, service, server_id, account_id, account_name, remote_id, remote_name, matched, missing, message, last_synced_at`

func (s *PlaylistStore) listLinks(ctx context.Context, query string, args ...any) ([]PlaylistLink, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlaylistLink{}
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
	}
	return out, rows.Err()
}

func scanLink(row interface{ Scan(...any) error }) (*PlaylistLink, error) {
	var l PlaylistLink
	var serverID sql.NullInt64
	var synced sql.NullTime
	if err := row.Scan(&l.ID, &l.PlaylistID, &l.Service, &serverID, &l.AccountID, &l.AccountName, &l.RemoteID, &l.RemoteName,
		&l.Matched, &l.Missing, &l.Message, &synced); err != nil {
		return nil, err
	}
	if serverID.Valid {
		id := int(serverID.Int64)
		l.ServerID = &id
	}
	if synced.Valid {
		l.LastSyncedAt = &synced.Time
	}
	return &l, nil
}

func (s *PlaylistStore) GetLink(ctx context.Context, playlistID, linkID int) (*PlaylistLink, error) {
	l, err := scanLink(s.db.QueryRowContext(ctx, `SELECT `+linkColumns+` FROM playlist_links WHERE id = ? AND playlist_id = ?`, linkID, playlistID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPlaylistLinkNotFound
	}
	return l, err
}

func (s *PlaylistStore) CreateLink(ctx context.Context, l *PlaylistLink) (*PlaylistLink, error) {
	var serverID any
	if l.ServerID != nil {
		serverID = *l.ServerID
	}
	var id int
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO playlist_links (playlist_id, service, server_id, account_id, account_name, remote_id, remote_name)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`, l.PlaylistID, l.Service, serverID, l.AccountID, l.AccountName, l.RemoteID, l.RemoteName).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetLink(ctx, l.PlaylistID, id)
}

// SaveSyncResult stores the remote playlist and the outcome of a sync.
func (s *PlaylistStore) SaveSyncResult(ctx context.Context, l *PlaylistLink) (*PlaylistLink, error) {
	// Bind UTC: CURRENT_TIMESTAMP would be the Postgres session's local time, and the column has no zone.
	_, err := s.db.ExecContext(ctx, `
		UPDATE playlist_links
		SET remote_id = ?, remote_name = ?, matched = ?, missing = ?, message = ?, last_synced_at = ?
		WHERE id = ?
	`, l.RemoteID, l.RemoteName, l.Matched, l.Missing, l.Message, time.Now().UTC(), l.ID)
	if err != nil {
		return nil, err
	}
	return s.GetLink(ctx, l.PlaylistID, l.ID)
}

func (s *PlaylistStore) DeleteLink(ctx context.Context, playlistID, linkID int) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM playlist_links WHERE id = ? AND playlist_id = ?`, linkID, playlistID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrPlaylistLinkNotFound
	}
	return nil
}

// Service sign-ins

// GetServiceSettings returns nil when the service is not set up. Settings
// that no longer decrypt (the session secret changed) read as not set up.
func (s *PlaylistStore) GetServiceSettings(ctx context.Context, service PlaylistService) (*PlaylistServiceSettings, error) {
	var encrypted string
	settings := &PlaylistServiceSettings{Service: service}
	err := s.db.QueryRowContext(ctx, `
		SELECT settings_encrypted, account_name, updated_at FROM playlist_services WHERE service = ?
	`, service).Scan(&encrypted, &settings.AccountName, &settings.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	plain, err := s.cipher.Decrypt(encrypted, nil)
	if err != nil {
		return nil, nil
	}
	settings.Settings = plain
	return settings, nil
}

func (s *PlaylistStore) SaveServiceSettings(ctx context.Context, settings *PlaylistServiceSettings) error {
	encrypted, err := s.cipher.Encrypt(settings.Settings, nil)
	if err != nil {
		return fmt.Errorf("encrypt settings: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO playlist_services (service, settings_encrypted, account_name, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (service) DO UPDATE SET
			settings_encrypted = excluded.settings_encrypted,
			account_name = excluded.account_name,
			updated_at = CURRENT_TIMESTAMP
	`, settings.Service, encrypted, settings.AccountName)
	return err
}

func (s *PlaylistStore) DeleteServiceSettings(ctx context.Context, service PlaylistService) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM playlist_services WHERE service = ?`, service)
	return err
}
