// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package mediasync

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/models"
	"github.com/gjhuerte/qui/internal/testutil/testdb"
)

func testStore(t *testing.T) *models.MediaSyncStore {
	t.Helper()
	store, err := models.NewMediaSyncStore(testdb.NewMigratedSQLite(t, "mediasync"), []byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)
	return store
}
