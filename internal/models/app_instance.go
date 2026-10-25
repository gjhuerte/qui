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

// AppType is a media app the Services page can track.
type AppType string

const (
	AppTypeSonarr   AppType = "sonarr"
	AppTypeRadarr   AppType = "radarr"
	AppTypeProwlarr AppType = "prowlarr"
	AppTypeBazarr   AppType = "bazarr"
	AppTypeTautulli AppType = "tautulli"
)

// AppTypes lists the supported types in display order.
var AppTypes = []AppType{AppTypeSonarr, AppTypeRadarr, AppTypeProwlarr, AppTypeBazarr, AppTypeTautulli}

// Valid reports whether t is a supported app type.
func (t AppType) Valid() bool {
	for _, known := range AppTypes {
		if t == known {
			return true
		}
	}
	return false
}

var ErrAppInstanceNotFound = errors.New("app instance not found")

// AppInstance is one Sonarr, Radarr, Prowlarr, Bazarr or Tautulli server.
// APIKey is decrypted and never sent to the browser.
type AppInstance struct {
	ID        int       `json:"id"`
	Type      AppType   `json:"type"`
	Name      string    `json:"name"`
	BaseURL   string    `json:"baseUrl"`
	APIKey    string    `json:"-"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AppInstanceStore struct {
	db     dbinterface.Querier
	cipher *CredentialCipher
}

func NewAppInstanceStore(db dbinterface.Querier, encryptionKey []byte, opts ...CredentialCipherOption) (*AppInstanceStore, error) {
	cipher, err := NewCredentialCipher(encryptionKey, opts...)
	if err != nil {
		return nil, err
	}
	return &AppInstanceStore{db: db, cipher: cipher}, nil
}

const appInstanceColumns = `id, type, name, base_url, api_key_encrypted, enabled, created_at, updated_at`

func (s *AppInstanceStore) scan(row interface{ Scan(...any) error }) (*AppInstance, error) {
	var inst AppInstance
	var encrypted string
	var enabled int
	if err := row.Scan(&inst.ID, &inst.Type, &inst.Name, &inst.BaseURL, &encrypted, &enabled, &inst.CreatedAt, &inst.UpdatedAt); err != nil {
		return nil, err
	}
	inst.Enabled = SQLiteIntToBool(enabled)
	if key, err := s.cipher.Decrypt(encrypted, nil); err == nil {
		inst.APIKey = key
	}
	return &inst, nil
}

// List returns all instances ordered by type and name.
func (s *AppInstanceStore) List(ctx context.Context) ([]*AppInstance, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+appInstanceColumns+` FROM app_instances ORDER BY type, name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*AppInstance{}
	for rows.Next() {
		inst, err := s.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inst)
	}
	return out, rows.Err()
}

func (s *AppInstanceStore) Get(ctx context.Context, id int) (*AppInstance, error) {
	inst, err := s.scan(s.db.QueryRowContext(ctx, `SELECT `+appInstanceColumns+` FROM app_instances WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAppInstanceNotFound
	}
	return inst, err
}

// Create stores a new instance and returns it with its ID.
func (s *AppInstanceStore) Create(ctx context.Context, inst *AppInstance) (*AppInstance, error) {
	encrypted, err := s.cipher.Encrypt(inst.APIKey, nil)
	if err != nil {
		return nil, fmt.Errorf("encrypt api key: %w", err)
	}
	var id int
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO app_instances (type, name, base_url, api_key_encrypted, enabled)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id
	`, inst.Type, inst.Name, inst.BaseURL, encrypted, BoolToSQLite(inst.Enabled)).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// Update replaces every field, including the API key.
func (s *AppInstanceStore) Update(ctx context.Context, inst *AppInstance) (*AppInstance, error) {
	encrypted, err := s.cipher.Encrypt(inst.APIKey, nil)
	if err != nil {
		return nil, fmt.Errorf("encrypt api key: %w", err)
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE app_instances
		SET type = ?, name = ?, base_url = ?, api_key_encrypted = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, inst.Type, inst.Name, inst.BaseURL, encrypted, BoolToSQLite(inst.Enabled), inst.ID)
	if err != nil {
		return nil, err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return nil, ErrAppInstanceNotFound
	}
	return s.Get(ctx, inst.ID)
}

func (s *AppInstanceStore) Delete(ctx context.Context, id int) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM app_instances WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrAppInstanceNotFound
	}
	return nil
}
