// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/models"
)

func TestMediaSyncStore(t *testing.T) {
	for name, open := range serviceStoreDatabases(t) {
		t.Run(name, func(t *testing.T) {
			store, err := models.NewMediaSyncStore(open(t), testEncryptionKey)
			require.NoError(t, err)
			ctx := t.Context()

			jf, err := store.CreateServer(ctx, &models.MediaServer{Type: models.MediaServerJellyfin, Name: "Jellyfin", BaseURL: "http://h:8096", Token: "jf-key", Enabled: true})
			require.NoError(t, err)
			px, err := store.CreateServer(ctx, &models.MediaServer{Type: models.MediaServerPlex, Name: "Plex", BaseURL: "http://h:32400", Token: "plex-token", Enabled: true})
			require.NoError(t, err)

			px.Token = "plex-token-2"
			px, err = store.UpdateServer(ctx, px)
			require.NoError(t, err)
			require.Equal(t, "plex-token-2", px.Token)

			pair, err := store.CreatePair(ctx, &models.SyncPair{ServerAID: jf.ID, ServerBID: px.ID, Direction: models.SyncBoth, Enabled: true, IntervalMinutes: 60})
			require.NoError(t, err)
			pair.Direction, pair.IntervalMinutes = models.SyncAToB, 30
			pair, err = store.UpdatePair(ctx, pair)
			require.NoError(t, err)
			require.Equal(t, models.SyncAToB, pair.Direction)
			require.Equal(t, 30, pair.IntervalMinutes)

			maps := []models.SyncUserMap{{UserAID: "a1", UserAName: "Ann", UserBID: "b1", UserBName: "ann_plex"}}
			require.NoError(t, store.ReplaceUserMaps(ctx, pair.ID, maps))
			require.NoError(t, store.ReplaceUserMaps(ctx, pair.ID, append(maps, models.SyncUserMap{UserAID: "a2", UserAName: "Bob", UserBID: "b2", UserBName: "bob"})))
			got, err := store.ListUserMaps(ctx, pair.ID)
			require.NoError(t, err)
			require.Len(t, got, 2)
			require.Equal(t, "Ann", got[0].UserAName)

			last, err := store.LastRunStarted(ctx, pair.ID)
			require.NoError(t, err)
			require.Nil(t, last)

			run, err := store.StartRun(ctx, pair.ID, models.SyncSourceManual)
			require.NoError(t, err)
			require.Equal(t, models.SyncRunRunning, run.Status)
			require.Nil(t, run.FinishedAt)

			run.Status, run.Users, run.MarkedB = models.SyncRunSuccess, 2, 1
			run.Items = []models.SyncRunItem{{UserName: "ann_plex", Target: "b", Title: "Example Movie (1999)"}}
			require.NoError(t, store.FinishRun(ctx, run))

			stored, err := store.GetRun(ctx, run.ID)
			require.NoError(t, err)
			require.Equal(t, models.SyncRunSuccess, stored.Status)
			require.Equal(t, 1, stored.MarkedB)
			require.NotNil(t, stored.FinishedAt)
			require.Equal(t, run.Items, stored.Items)

			last, err = store.LastRunStarted(ctx, pair.ID)
			require.NoError(t, err)
			require.NotNil(t, last)

			interrupted, err := store.StartRun(ctx, pair.ID, models.SyncSourceSchedule)
			require.NoError(t, err)
			require.NoError(t, store.FailInterruptedRuns(ctx))
			interrupted, err = store.GetRun(ctx, interrupted.ID)
			require.NoError(t, err)
			require.Equal(t, models.SyncRunFailed, interrupted.Status)

			runs, err := store.ListRuns(ctx, 0, 10)
			require.NoError(t, err)
			require.Len(t, runs, 2)
			require.Equal(t, interrupted.ID, runs[0].ID)

			require.NoError(t, store.PruneRuns(ctx, time.Now().Add(time.Hour)))
			runs, err = store.ListRuns(ctx, pair.ID, 10)
			require.NoError(t, err)
			require.Empty(t, runs)

			// Deleting a server deletes its pairs, their maps and runs.
			_, err = store.StartRun(ctx, pair.ID, models.SyncSourceManual)
			require.NoError(t, err)
			require.NoError(t, store.DeleteServer(ctx, px.ID))
			_, err = store.GetPair(ctx, pair.ID)
			require.ErrorIs(t, err, models.ErrSyncPairNotFound)
			runs, err = store.ListRuns(ctx, 0, 10)
			require.NoError(t, err)
			require.Empty(t, runs)
			require.ErrorIs(t, store.DeleteServer(ctx, px.ID), models.ErrMediaServerNotFound)
		})
	}
}
