// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/models"
)

func TestPlaylistStore(t *testing.T) {
	for name, open := range serviceStoreDatabases(t) {
		t.Run(name, func(t *testing.T) {
			db := open(t)
			store, err := models.NewPlaylistStore(db, testEncryptionKey)
			require.NoError(t, err)
			servers, err := models.NewMediaSyncStore(db, testEncryptionKey)
			require.NoError(t, err)
			ctx := t.Context()

			p, err := store.CreatePlaylist(ctx, &models.Playlist{Name: "Rainy", Request: "slow songs", Tracks: []models.PlaylistTrack{
				{Artist: "Nora Vale", Title: "Grey Windows", Album: "Weather"}, {Artist: "The Lanterns", Title: "Puddles"},
			}})
			require.NoError(t, err)
			require.Equal(t, 2, p.TrackCount)
			require.Equal(t, "Puddles", p.Tracks[1].Title)

			p, err = store.UpdatePlaylist(ctx, &models.Playlist{ID: p.ID, Name: "Rainy day", Tracks: []models.PlaylistTrack{{Artist: "Kite Club", Title: "Summer Static"}}})
			require.NoError(t, err)
			require.Equal(t, "Rainy day", p.Name)
			require.Equal(t, []models.PlaylistTrack{{Artist: "Kite Club", Title: "Summer Static"}}, p.Tracks)

			srv, err := servers.CreateServer(ctx, &models.MediaServer{Type: models.MediaServerPlex, Name: "Plex", BaseURL: "http://h:32400", Token: "t", Enabled: true})
			require.NoError(t, err)
			plexLink, err := store.CreateLink(ctx, &models.PlaylistLink{PlaylistID: p.ID, Service: models.PlaylistPlex, ServerID: &srv.ID, AccountID: "1", AccountName: "gab"})
			require.NoError(t, err)
			spotifyLink, err := store.CreateLink(ctx, &models.PlaylistLink{PlaylistID: p.ID, Service: models.PlaylistSpotify, RemoteID: "abc"})
			require.NoError(t, err)
			require.Nil(t, spotifyLink.ServerID)

			plexLink.RemoteID, plexLink.Matched, plexLink.Missing, plexLink.Message = "900", 1, 0, ""
			saved, err := store.SaveSyncResult(ctx, plexLink)
			require.NoError(t, err)
			require.Equal(t, "900", saved.RemoteID)
			require.NotNil(t, saved.LastSyncedAt)

			list, err := store.ListPlaylists(ctx)
			require.NoError(t, err)
			require.Len(t, list, 1)
			require.Equal(t, 1, list[0].TrackCount)
			require.Len(t, list[0].Links, 2)

			// Deleting the Plex server deletes its link only.
			require.NoError(t, servers.DeleteServer(ctx, srv.ID))
			p, err = store.GetPlaylist(ctx, p.ID)
			require.NoError(t, err)
			require.Len(t, p.Links, 1)
			require.Equal(t, models.PlaylistSpotify, p.Links[0].Service)

			settings, err := store.GetServiceSettings(ctx, models.PlaylistSpotify)
			require.NoError(t, err)
			require.Nil(t, settings)
			require.NoError(t, store.SaveServiceSettings(ctx, &models.PlaylistServiceSettings{Service: models.PlaylistSpotify, Settings: `{"clientId":"c"}`, AccountName: "Gab"}))
			require.NoError(t, store.SaveServiceSettings(ctx, &models.PlaylistServiceSettings{Service: models.PlaylistSpotify, Settings: `{"clientId":"d"}`, AccountName: "Gab"}))
			settings, err = store.GetServiceSettings(ctx, models.PlaylistSpotify)
			require.NoError(t, err)
			require.Equal(t, `{"clientId":"d"}`, settings.Settings)
			require.NoError(t, store.DeleteServiceSettings(ctx, models.PlaylistSpotify))

			require.NoError(t, store.DeletePlaylist(ctx, p.ID))
			require.ErrorIs(t, store.DeletePlaylist(ctx, p.ID), models.ErrPlaylistNotFound)
			_, err = store.GetLink(ctx, p.ID, spotifyLink.ID)
			require.ErrorIs(t, err, models.ErrPlaylistLinkNotFound)
		})
	}
}
