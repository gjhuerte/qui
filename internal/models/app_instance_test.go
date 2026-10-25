// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/models"
)

func TestAppInstanceStore(t *testing.T) {
	for name, open := range serviceStoreDatabases(t) {
		t.Run(name, func(t *testing.T) {
			store, err := models.NewAppInstanceStore(open(t), testEncryptionKey)
			require.NoError(t, err)
			ctx := t.Context()

			list, err := store.List(ctx)
			require.NoError(t, err)
			require.Empty(t, list)

			radarr, err := store.Create(ctx, &models.AppInstance{Type: models.AppTypeRadarr, Name: "Movies", BaseURL: "http://h:7878", APIKey: "r", Enabled: true})
			require.NoError(t, err)
			_, err = store.Create(ctx, &models.AppInstance{Type: models.AppTypeBazarr, Name: "Subs", BaseURL: "http://h:6767", APIKey: "b", Enabled: false})
			require.NoError(t, err)

			radarr.Name, radarr.APIKey = "Movies 4K", "r2"
			updated, err := store.Update(ctx, radarr)
			require.NoError(t, err)
			require.Equal(t, "Movies 4K", updated.Name)
			require.Equal(t, "r2", updated.APIKey)

			list, err = store.List(ctx)
			require.NoError(t, err)
			require.Len(t, list, 2)
			require.Equal(t, models.AppTypeBazarr, list[0].Type)
			require.False(t, list[0].Enabled)

			require.NoError(t, store.Delete(ctx, radarr.ID))
			require.ErrorIs(t, store.Delete(ctx, radarr.ID), models.ErrAppInstanceNotFound)
			_, err = store.Get(ctx, radarr.ID)
			require.ErrorIs(t, err, models.ErrAppInstanceNotFound)
		})
	}
}
