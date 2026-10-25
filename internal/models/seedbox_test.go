// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/database"
	"github.com/gjhuerte/qui/internal/models"
	"github.com/gjhuerte/qui/internal/testutil/testdb"
)

var testEncryptionKey = []byte("0123456789abcdef0123456789abcdef")

func serviceStoreDatabases(t *testing.T) map[string]func(t *testing.T) *database.DB {
	t.Helper()
	return map[string]func(t *testing.T) *database.DB{
		"sqlite":   func(t *testing.T) *database.DB { return testdb.NewMigratedSQLite(t, "seedbox-gluetun") },
		"postgres": func(t *testing.T) *database.DB { return testdb.NewMigratedPostgres(t, "seedbox-gluetun") },
	}
}

func TestSeedboxStore(t *testing.T) {
	for name, open := range serviceStoreDatabases(t) {
		t.Run(name, func(t *testing.T) {
			store, err := models.NewSeedboxStore(open(t), testEncryptionKey)
			require.NoError(t, err)
			ctx := t.Context()

			settings, err := store.GetSettings(ctx)
			require.NoError(t, err)
			require.Equal(t, &models.SeedboxSettings{Enabled: true, IntervalSeconds: models.SeedboxDefaultInterval}, settings)

			require.NoError(t, store.SetMamID(ctx, "cookie-value"))
			require.NoError(t, store.UpdateSchedule(ctx, false, 30))
			settings, err = store.GetSettings(ctx)
			require.NoError(t, err)
			require.Equal(t, "cookie-value", settings.MamID)
			require.False(t, settings.Enabled)
			require.Equal(t, models.SeedboxMinInterval, settings.IntervalSeconds)

			require.NoError(t, store.SetMamID(ctx, ""))
			settings, err = store.GetSettings(ctx)
			require.NoError(t, err)
			require.Empty(t, settings.MamID)
			require.False(t, settings.Enabled, "clearing the cookie keeps the schedule")

			yes, status, asn := true, 200, 12345
			_, err = store.AddContact(ctx, &models.SeedboxContact{Source: "manual", Reached: true, Success: &yes, HTTPStatus: &status, Message: "No change", IP: "1.2.3.4", ASN: &asn, ASOrg: "VPN"})
			require.NoError(t, err)
			_, err = store.AddContact(ctx, &models.SeedboxContact{Source: "schedule", Reached: false, Message: "timeout"})
			require.NoError(t, err)

			contacts, err := store.ListContacts(ctx, 10)
			require.NoError(t, err)
			require.Len(t, contacts, 2)
			require.Equal(t, models.SeedboxStateUnreachable, contacts[0].State)
			require.Equal(t, models.SeedboxStateUnchanged, contacts[1].State)
			require.Equal(t, 12345, *contacts[1].ASN)
			require.Nil(t, contacts[0].Success)

			deleted, err := store.PruneContacts(ctx, time.Now().Add(time.Hour))
			require.NoError(t, err)
			require.EqualValues(t, 2, deleted)
		})
	}
}

func TestGluetunSettingsStore(t *testing.T) {
	for name, open := range serviceStoreDatabases(t) {
		t.Run(name, func(t *testing.T) {
			store, err := models.NewGluetunSettingsStore(open(t), testEncryptionKey)
			require.NoError(t, err)
			ctx := t.Context()

			settings, err := store.Get(ctx)
			require.NoError(t, err)
			require.Equal(t, models.DefaultGluetunSettings(), settings)

			saved := &models.GluetunSettings{ControlURL: "http://gluetun:8000", AuthType: models.GluetunAuthAPIKey, Secret: "key", DockerHost: "", ContainerName: "vpn"}
			require.NoError(t, store.Save(ctx, saved))
			settings, err = store.Get(ctx)
			require.NoError(t, err)
			settings.UpdatedAt = time.Time{}
			require.Equal(t, saved, settings)
		})
	}
}
