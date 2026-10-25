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

// Gluetun control server auth types.
const (
	GluetunAuthNone   = "none"
	GluetunAuthAPIKey = "apikey"
	GluetunAuthBasic  = "basic"
)

// GluetunSettings says how qui reaches gluetun. Secret is the decrypted API key
// or password and never leaves the server.
type GluetunSettings struct {
	ControlURL    string
	AuthType      string
	Username      string
	Secret        string
	DockerHost    string
	ContainerName string
	UpdatedAt     time.Time
}

// DefaultGluetunSettings fit qui running with network_mode: "service:gluetun"
// and the Docker socket mounted.
func DefaultGluetunSettings() *GluetunSettings {
	return &GluetunSettings{
		ControlURL:    "http://127.0.0.1:8000",
		AuthType:      GluetunAuthNone,
		DockerHost:    "unix:///var/run/docker.sock",
		ContainerName: "gluetun",
	}
}

type GluetunSettingsStore struct {
	db     dbinterface.Querier
	cipher *CredentialCipher
}

func NewGluetunSettingsStore(db dbinterface.Querier, encryptionKey []byte, opts ...CredentialCipherOption) (*GluetunSettingsStore, error) {
	cipher, err := NewCredentialCipher(encryptionKey, opts...)
	if err != nil {
		return nil, err
	}
	return &GluetunSettingsStore{db: db, cipher: cipher}, nil
}

func (s *GluetunSettingsStore) Get(ctx context.Context) (*GluetunSettings, error) {
	settings := DefaultGluetunSettings()
	var secret sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT control_url, auth_type, username, secret_encrypted, docker_host, container_name, updated_at
		FROM gluetun_settings
		WHERE id = 1
	`).Scan(&settings.ControlURL, &settings.AuthType, &settings.Username, &secret, &settings.DockerHost, &settings.ContainerName, &settings.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, nil
	}
	if err != nil {
		return nil, err
	}

	if secret.Valid && secret.String != "" {
		if plain, err := s.cipher.Decrypt(secret.String, nil); err == nil {
			settings.Secret = plain
		}
	}
	return settings, nil
}

// Save replaces the settings, including the secret (empty clears it).
func (s *GluetunSettingsStore) Save(ctx context.Context, settings *GluetunSettings) error {
	var secret any
	if settings.Secret != "" {
		encrypted, err := s.cipher.Encrypt(settings.Secret, nil)
		if err != nil {
			return fmt.Errorf("encrypt gluetun secret: %w", err)
		}
		secret = encrypted
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO gluetun_settings (id, control_url, auth_type, username, secret_encrypted, docker_host, container_name, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET
			control_url = excluded.control_url,
			auth_type = excluded.auth_type,
			username = excluded.username,
			secret_encrypted = excluded.secret_encrypted,
			docker_host = excluded.docker_host,
			container_name = excluded.container_name,
			updated_at = CURRENT_TIMESTAMP
	`, settings.ControlURL, settings.AuthType, settings.Username, secret, settings.DockerHost, settings.ContainerName)
	return err
}
