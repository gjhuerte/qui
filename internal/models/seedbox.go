// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gjhuerte/qui/internal/dbinterface"
)

// Seedbox schedule limits and defaults.
const (
	SeedboxDefaultInterval = 600
	SeedboxMinInterval     = 60
	SeedboxMaxInterval     = 86400
)

// SeedboxSettings is the single settings row of the MyAnonamouse dynamic seedbox
// updater. MamID is the decrypted cookie and never leaves the server.
type SeedboxSettings struct {
	MamID           string
	Enabled         bool
	IntervalSeconds int
	UpdatedAt       time.Time
}

// SeedboxContact is one request to MAM and its outcome.
type SeedboxContact struct {
	ID         int64     `json:"id"`
	Source     string    `json:"source"` // schedule | manual | cookie
	Reached    bool      `json:"-"`
	Success    *bool     `json:"-"`
	HTTPStatus *int      `json:"httpStatus,omitempty"`
	Message    string    `json:"message,omitempty"`
	IP         string    `json:"ip,omitempty"`
	ASN        *int      `json:"asn,omitempty"`
	ASOrg      string    `json:"asOrg,omitempty"`
	State      string    `json:"state"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Seedbox contact states.
const (
	SeedboxStateUpdated     = "updated"
	SeedboxStateUnchanged   = "unchanged"
	SeedboxStateWaiting     = "waiting"
	SeedboxStateThrottled   = "throttled"
	SeedboxStateRejected    = "rejected"
	SeedboxStateUnreachable = "unreachable"
)

// ComputeState names the outcome: updated, unchanged, throttled, rejected,
// unreachable, or waiting (no mam_id set yet).
func (c *SeedboxContact) ComputeState() string {
	switch {
	case !c.Reached:
		return SeedboxStateUnreachable
	case c.Success == nil:
		return SeedboxStateWaiting
	case *c.Success && strings.Contains(strings.ToLower(c.Message), "no change"):
		return SeedboxStateUnchanged
	case *c.Success:
		return SeedboxStateUpdated
	case c.HTTPStatus != nil && *c.HTTPStatus == 429:
		return SeedboxStateThrottled
	default:
		return SeedboxStateRejected
	}
}

// ClampSeedboxInterval keeps an interval inside the allowed range.
func ClampSeedboxInterval(seconds int) int {
	return max(SeedboxMinInterval, min(SeedboxMaxInterval, seconds))
}

type SeedboxStore struct {
	db     dbinterface.Querier
	cipher *CredentialCipher
}

func NewSeedboxStore(db dbinterface.Querier, encryptionKey []byte, opts ...CredentialCipherOption) (*SeedboxStore, error) {
	cipher, err := NewCredentialCipher(encryptionKey, opts...)
	if err != nil {
		return nil, err
	}
	return &SeedboxStore{db: db, cipher: cipher}, nil
}

// GetSettings returns the settings, with defaults when nothing was saved yet.
func (s *SeedboxStore) GetSettings(ctx context.Context) (*SeedboxSettings, error) {
	var encrypted sql.NullString
	var enabled, interval int
	var updatedAt time.Time

	err := s.db.QueryRowContext(ctx, `
		SELECT mam_id_encrypted, enabled, interval_seconds, updated_at
		FROM seedbox_settings
		WHERE id = 1
	`).Scan(&encrypted, &enabled, &interval, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return &SeedboxSettings{Enabled: true, IntervalSeconds: SeedboxDefaultInterval}, nil
	}
	if err != nil {
		return nil, err
	}

	settings := &SeedboxSettings{
		Enabled:         SQLiteIntToBool(enabled),
		IntervalSeconds: ClampSeedboxInterval(interval),
		UpdatedAt:       updatedAt,
	}
	if encrypted.Valid && encrypted.String != "" {
		mamID, err := s.cipher.Decrypt(encrypted.String, nil)
		if err != nil {
			// The session secret changed: the stored cookie is unreadable.
			return settings, nil
		}
		settings.MamID = mamID
	}
	return settings, nil
}

// SetMamID stores the cookie, or clears it when mamID is empty.
func (s *SeedboxStore) SetMamID(ctx context.Context, mamID string) error {
	var value any
	if mamID != "" {
		encrypted, err := s.cipher.Encrypt(mamID, nil)
		if err != nil {
			return fmt.Errorf("encrypt mam_id: %w", err)
		}
		value = encrypted
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO seedbox_settings (id, mam_id_encrypted, updated_at)
		VALUES (1, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET mam_id_encrypted = excluded.mam_id_encrypted, updated_at = CURRENT_TIMESTAMP
	`, value)
	return err
}

// UpdateSchedule stores whether automatic updates run and how often.
func (s *SeedboxStore) UpdateSchedule(ctx context.Context, enabled bool, intervalSeconds int) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO seedbox_settings (id, enabled, interval_seconds, updated_at)
		VALUES (1, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET enabled = excluded.enabled, interval_seconds = excluded.interval_seconds, updated_at = CURRENT_TIMESTAMP
	`, BoolToSQLite(enabled), ClampSeedboxInterval(intervalSeconds))
	return err
}

// AddContact records one request to MAM.
func (s *SeedboxStore) AddContact(ctx context.Context, c *SeedboxContact) (*SeedboxContact, error) {
	var success any
	if c.Success != nil {
		success = BoolToSQLite(*c.Success)
	}

	err := s.db.QueryRowContext(ctx, `
		INSERT INTO seedbox_contacts (source, reached, success, http_status, message, ip, asn, as_org)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, created_at
	`, c.Source, BoolToSQLite(c.Reached), success, c.HTTPStatus, nullIfEmpty(c.Message), nullIfEmpty(c.IP), c.ASN, nullIfEmpty(c.ASOrg)).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.State = c.ComputeState()
	return c, nil
}

// ListContacts returns the newest contacts first.
func (s *SeedboxStore) ListContacts(ctx context.Context, limit int) ([]SeedboxContact, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, source, reached, success, http_status, message, ip, asn, as_org, created_at
		FROM seedbox_contacts
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contacts := []SeedboxContact{}
	for rows.Next() {
		var c SeedboxContact
		var reached int
		var success, httpStatus, asn sql.NullInt64
		var message, ip, asOrg sql.NullString
		if err := rows.Scan(&c.ID, &c.Source, &reached, &success, &httpStatus, &message, &ip, &asn, &asOrg, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.Reached = SQLiteIntToBool(reached)
		if success.Valid {
			v := success.Int64 != 0
			c.Success = &v
		}
		if httpStatus.Valid {
			v := int(httpStatus.Int64)
			c.HTTPStatus = &v
		}
		if asn.Valid {
			v := int(asn.Int64)
			c.ASN = &v
		}
		c.Message = message.String
		c.IP = ip.String
		c.ASOrg = asOrg.String
		c.State = c.ComputeState()
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

// PruneContacts deletes contacts older than before.
func (s *SeedboxStore) PruneContacts(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM seedbox_contacts WHERE created_at < ?`, before.UTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
