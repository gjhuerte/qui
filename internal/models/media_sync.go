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

// MediaServerType is a media server whose watched state qui can sync.
type MediaServerType string

const (
	MediaServerJellyfin MediaServerType = "jellyfin"
	MediaServerPlex     MediaServerType = "plex"
)

// MediaServerTypes lists the supported types in display order.
var MediaServerTypes = []MediaServerType{MediaServerJellyfin, MediaServerPlex}

func (t MediaServerType) Valid() bool {
	for _, known := range MediaServerTypes {
		if t == known {
			return true
		}
	}
	return false
}

// SyncDirection says which way watched state flows between server A and B.
type SyncDirection string

const (
	SyncBoth SyncDirection = "both"
	SyncAToB SyncDirection = "a_to_b"
	SyncBToA SyncDirection = "b_to_a"
)

func (d SyncDirection) Valid() bool {
	return d == SyncBoth || d == SyncAToB || d == SyncBToA
}

// ToB and ToA report whether marks are written to that side.
func (d SyncDirection) ToB() bool { return d == SyncBoth || d == SyncAToB }
func (d SyncDirection) ToA() bool { return d == SyncBoth || d == SyncBToA }

// Media sync run states and sources.
const (
	SyncRunRunning = "running"
	SyncRunSuccess = "success"
	// SyncRunPartial: some users or items failed, the rest synced.
	SyncRunPartial = "partial"
	SyncRunFailed  = "failed"

	SyncSourceSchedule = "schedule"
	SyncSourceManual   = "manual"
)

var (
	ErrMediaServerNotFound = errors.New("media server not found")
	ErrSyncPairNotFound    = errors.New("sync pair not found")
	ErrSyncRunNotFound     = errors.New("sync run not found")
)

// MediaServer is one Jellyfin or Plex server. Token is the Jellyfin API key or
// the Plex token of the server owner; it is decrypted and never sent out.
type MediaServer struct {
	ID        int             `json:"id"`
	Type      MediaServerType `json:"type"`
	Name      string          `json:"name"`
	BaseURL   string          `json:"baseUrl"`
	Token     string          `json:"-"`
	Enabled   bool            `json:"enabled"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// SyncPair links two servers. IntervalMinutes 0 means manual runs only.
type SyncPair struct {
	ID              int           `json:"id"`
	ServerAID       int           `json:"serverAId"`
	ServerBID       int           `json:"serverBId"`
	Direction       SyncDirection `json:"direction"`
	Enabled         bool          `json:"enabled"`
	IntervalMinutes int           `json:"intervalMinutes"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
}

// SyncUserMap says that account UserAID on server A is account UserBID on
// server B. The names are kept so the log reads well after a user is renamed
// or removed.
type SyncUserMap struct {
	UserAID   string `json:"userAId"`
	UserAName string `json:"userAName"`
	UserBID   string `json:"userBId"`
	UserBName string `json:"userBName"`
}

// SyncRun is one sync of a pair and its totals.
type SyncRun struct {
	ID         int64         `json:"id"`
	PairID     int           `json:"pairId"`
	Source     string        `json:"source"`
	Status     string        `json:"status"`
	Users      int           `json:"users"`
	MarkedA    int           `json:"markedA"`
	MarkedB    int           `json:"markedB"`
	Failed     int           `json:"failed"`
	NotFound   int           `json:"notFound"`
	Message    string        `json:"message"`
	StartedAt  time.Time     `json:"startedAt"`
	FinishedAt *time.Time    `json:"finishedAt,omitempty"`
	Items      []SyncRunItem `json:"items,omitempty"`
}

// SyncRunItem is one title marked watched, or one that failed to mark.
type SyncRunItem struct {
	UserName string `json:"userName"`
	// Target is the side that was written: "a" or "b".
	Target string `json:"target"`
	Title  string `json:"title"`
	Error  string `json:"error,omitempty"`
}

type MediaSyncStore struct {
	db     dbinterface.Querier
	cipher *CredentialCipher
}

func NewMediaSyncStore(db dbinterface.Querier, encryptionKey []byte, opts ...CredentialCipherOption) (*MediaSyncStore, error) {
	cipher, err := NewCredentialCipher(encryptionKey, opts...)
	if err != nil {
		return nil, err
	}
	return &MediaSyncStore{db: db, cipher: cipher}, nil
}

// Servers

const mediaServerColumns = `id, type, name, base_url, token_encrypted, enabled, created_at, updated_at`

func (s *MediaSyncStore) scanServer(row interface{ Scan(...any) error }) (*MediaServer, error) {
	var srv MediaServer
	var encrypted string
	var enabled int
	if err := row.Scan(&srv.ID, &srv.Type, &srv.Name, &srv.BaseURL, &encrypted, &enabled, &srv.CreatedAt, &srv.UpdatedAt); err != nil {
		return nil, err
	}
	srv.Enabled = SQLiteIntToBool(enabled)
	if token, err := s.cipher.Decrypt(encrypted, nil); err == nil {
		srv.Token = token
	}
	return &srv, nil
}

func (s *MediaSyncStore) ListServers(ctx context.Context) ([]*MediaServer, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+mediaServerColumns+` FROM media_servers ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*MediaServer{}
	for rows.Next() {
		srv, err := s.scanServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, srv)
	}
	return out, rows.Err()
}

func (s *MediaSyncStore) GetServer(ctx context.Context, id int) (*MediaServer, error) {
	srv, err := s.scanServer(s.db.QueryRowContext(ctx, `SELECT `+mediaServerColumns+` FROM media_servers WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMediaServerNotFound
	}
	return srv, err
}

func (s *MediaSyncStore) CreateServer(ctx context.Context, srv *MediaServer) (*MediaServer, error) {
	encrypted, err := s.cipher.Encrypt(srv.Token, nil)
	if err != nil {
		return nil, fmt.Errorf("encrypt token: %w", err)
	}
	var id int
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO media_servers (type, name, base_url, token_encrypted, enabled)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id
	`, srv.Type, srv.Name, srv.BaseURL, encrypted, BoolToSQLite(srv.Enabled)).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetServer(ctx, id)
}

// UpdateServer replaces every field, including the token.
func (s *MediaSyncStore) UpdateServer(ctx context.Context, srv *MediaServer) (*MediaServer, error) {
	encrypted, err := s.cipher.Encrypt(srv.Token, nil)
	if err != nil {
		return nil, fmt.Errorf("encrypt token: %w", err)
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE media_servers
		SET type = ?, name = ?, base_url = ?, token_encrypted = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, srv.Type, srv.Name, srv.BaseURL, encrypted, BoolToSQLite(srv.Enabled), srv.ID)
	if err != nil {
		return nil, err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return nil, ErrMediaServerNotFound
	}
	return s.GetServer(ctx, srv.ID)
}

// DeleteServer also deletes the pairs that use the server, with their maps and runs.
func (s *MediaSyncStore) DeleteServer(ctx context.Context, id int) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM media_servers WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrMediaServerNotFound
	}
	return nil
}

// Pairs

const syncPairColumns = `id, server_a_id, server_b_id, direction, enabled, interval_minutes, created_at, updated_at`

func scanPair(row interface{ Scan(...any) error }) (*SyncPair, error) {
	var p SyncPair
	var enabled int
	if err := row.Scan(&p.ID, &p.ServerAID, &p.ServerBID, &p.Direction, &enabled, &p.IntervalMinutes, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	p.Enabled = SQLiteIntToBool(enabled)
	return &p, nil
}

func (s *MediaSyncStore) ListPairs(ctx context.Context) ([]*SyncPair, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+syncPairColumns+` FROM media_sync_pairs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*SyncPair{}
	for rows.Next() {
		p, err := scanPair(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *MediaSyncStore) GetPair(ctx context.Context, id int) (*SyncPair, error) {
	p, err := scanPair(s.db.QueryRowContext(ctx, `SELECT `+syncPairColumns+` FROM media_sync_pairs WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSyncPairNotFound
	}
	return p, err
}

func (s *MediaSyncStore) CreatePair(ctx context.Context, p *SyncPair) (*SyncPair, error) {
	var id int
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO media_sync_pairs (server_a_id, server_b_id, direction, enabled, interval_minutes)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id
	`, p.ServerAID, p.ServerBID, p.Direction, BoolToSQLite(p.Enabled), p.IntervalMinutes).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetPair(ctx, id)
}

// UpdatePair changes the direction and schedule. The servers of a pair are
// fixed, because the user maps belong to them.
func (s *MediaSyncStore) UpdatePair(ctx context.Context, p *SyncPair) (*SyncPair, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE media_sync_pairs
		SET direction = ?, enabled = ?, interval_minutes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, p.Direction, BoolToSQLite(p.Enabled), p.IntervalMinutes, p.ID)
	if err != nil {
		return nil, err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return nil, ErrSyncPairNotFound
	}
	return s.GetPair(ctx, p.ID)
}

func (s *MediaSyncStore) DeletePair(ctx context.Context, id int) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM media_sync_pairs WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrSyncPairNotFound
	}
	return nil
}

// User maps

func (s *MediaSyncStore) ListUserMaps(ctx context.Context, pairID int) ([]SyncUserMap, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_a_id, user_a_name, user_b_id, user_b_name
		FROM media_sync_user_maps
		WHERE pair_id = ?
		ORDER BY user_a_name, user_b_name
	`, pairID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SyncUserMap{}
	for rows.Next() {
		var m SyncUserMap
		if err := rows.Scan(&m.UserAID, &m.UserAName, &m.UserBID, &m.UserBName); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ReplaceUserMaps sets the full list of user maps of a pair.
func (s *MediaSyncStore) ReplaceUserMaps(ctx context.Context, pairID int, maps []SyncUserMap) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM media_sync_user_maps WHERE pair_id = ?`, pairID); err != nil {
		return err
	}
	for _, m := range maps {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO media_sync_user_maps (pair_id, user_a_id, user_a_name, user_b_id, user_b_name)
			VALUES (?, ?, ?, ?, ?)
		`, pairID, m.UserAID, m.UserAName, m.UserBID, m.UserBName); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Runs

const syncRunColumns = `id, pair_id, source, status, users, marked_a, marked_b, failed, not_found, message, started_at, finished_at`

func scanRun(row interface{ Scan(...any) error }) (*SyncRun, error) {
	var r SyncRun
	var finished sql.NullTime
	if err := row.Scan(&r.ID, &r.PairID, &r.Source, &r.Status, &r.Users, &r.MarkedA, &r.MarkedB, &r.Failed, &r.NotFound, &r.Message, &r.StartedAt, &finished); err != nil {
		return nil, err
	}
	if finished.Valid {
		r.FinishedAt = &finished.Time
	}
	return &r, nil
}

// StartRun records a run in the running state.
func (s *MediaSyncStore) StartRun(ctx context.Context, pairID int, source string) (*SyncRun, error) {
	// Bind UTC: CURRENT_TIMESTAMP would be the Postgres session's local time, and the column has no zone.
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO media_sync_runs (pair_id, source, status, started_at)
		VALUES (?, ?, ?, ?)
		RETURNING id
	`, pairID, source, SyncRunRunning, time.Now().UTC()).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetRun(ctx, id)
}

// FinishRun stores the totals and the items of a finished run.
func (s *MediaSyncStore) FinishRun(ctx context.Context, run *SyncRun) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE media_sync_runs
		SET status = ?, users = ?, marked_a = ?, marked_b = ?, failed = ?, not_found = ?, message = ?, finished_at = ?
		WHERE id = ?
	`, run.Status, run.Users, run.MarkedA, run.MarkedB, run.Failed, run.NotFound, run.Message, time.Now().UTC(), run.ID); err != nil {
		return err
	}
	for _, item := range run.Items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO media_sync_run_items (run_id, user_name, target, title, error)
			VALUES (?, ?, ?, ?, ?)
		`, run.ID, item.UserName, item.Target, item.Title, item.Error); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetRun returns a run with its items.
func (s *MediaSyncStore) GetRun(ctx context.Context, id int64) (*SyncRun, error) {
	run, err := scanRun(s.db.QueryRowContext(ctx, `SELECT `+syncRunColumns+` FROM media_sync_runs WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSyncRunNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT user_name, target, title, error
		FROM media_sync_run_items
		WHERE run_id = ?
		ORDER BY id
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item SyncRunItem
		if err := rows.Scan(&item.UserName, &item.Target, &item.Title, &item.Error); err != nil {
			return nil, err
		}
		run.Items = append(run.Items, item)
	}
	return run, rows.Err()
}

// ListRuns returns the newest runs first, without items. pairID 0 lists every pair.
func (s *MediaSyncStore) ListRuns(ctx context.Context, pairID, limit int) ([]*SyncRun, error) {
	query := `SELECT ` + syncRunColumns + ` FROM media_sync_runs`
	args := []any{}
	if pairID > 0 {
		query += ` WHERE pair_id = ?`
		args = append(args, pairID)
	}
	query += ` ORDER BY started_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*SyncRun{}
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// LastRunStarted returns when the newest run of a pair started, or nil.
func (s *MediaSyncStore) LastRunStarted(ctx context.Context, pairID int) (*time.Time, error) {
	var started time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT started_at FROM media_sync_runs WHERE pair_id = ? ORDER BY started_at DESC, id DESC LIMIT 1
	`, pairID).Scan(&started)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &started, nil
}

// FailInterruptedRuns marks runs left running by a restart as failed.
func (s *MediaSyncStore) FailInterruptedRuns(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE media_sync_runs
		SET status = ?, message = ?, finished_at = ?
		WHERE status = ?
	`, SyncRunFailed, "interrupted by a restart", time.Now().UTC(), SyncRunRunning)
	return err
}

// PruneRuns deletes runs that started before the cutoff, with their items.
func (s *MediaSyncStore) PruneRuns(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM media_sync_runs WHERE started_at < ?`, before.UTC())
	return err
}
