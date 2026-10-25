// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/models"
	"github.com/gjhuerte/qui/internal/testutil/testdb"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

func testStores(t *testing.T) (*models.PlaylistStore, *models.MediaSyncStore) {
	t.Helper()
	db := testdb.NewMigratedSQLite(t, "playlists")
	playlists, err := models.NewPlaylistStore(db, testKey)
	require.NoError(t, err)
	servers, err := models.NewMediaSyncStore(db, testKey)
	require.NoError(t, err)
	return playlists, servers
}
