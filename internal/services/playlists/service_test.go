// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/models"
)

const aiAnswer = "```json\n" + `{"name": "Rainy Sunday", "description": "Slow songs", "tracks": [
	{"artist": "Nora Vale", "title": "Grey Windows"},
	{"artist": "The Lanterns", "title": "Puddles"},
	{"artist": "Nobody", "title": "Not In The Library"}]}` + "\n```"

func setupJellyfin(t *testing.T) (*Service, *fakeJellyfin, *models.Playlist, *models.PlaylistLink) {
	t.Helper()
	f := newFakeJellyfin(t,
		song("s1", "Nora Vale", "Grey Windows"),
		song("s2", "The Lanterns", "Puddles"),
		song("s3", "Kite Club", "Summer Static"),
	)
	store, servers := testStores(t)
	ctx := t.Context()
	srv, err := servers.CreateServer(ctx, &models.MediaServer{Type: models.MediaServerJellyfin, Name: "Home", BaseURL: f.URL, Token: "key", Enabled: true})
	require.NoError(t, err)

	svc := NewService(store, servers)
	p, err := svc.Import(ctx, ImportInput{Text: aiAnswer, Request: "rainy day songs"})
	require.NoError(t, err)
	require.Equal(t, "Rainy Sunday", p.Name)
	require.Len(t, p.Tracks, 3)

	link, err := svc.CreateLink(ctx, p.ID, LinkInput{Account: Account{Service: models.PlaylistJellyfin, ServerID: &srv.ID, AccountID: "u1"}, AccountName: "Ann"})
	require.NoError(t, err)
	return svc, f, p, link
}

func TestSyncCreatesThenUpdates(t *testing.T) {
	svc, f, p, link := setupJellyfin(t)
	ctx := t.Context()

	res, err := svc.Sync(ctx, p.ID, link.ID)
	require.NoError(t, err)
	require.Equal(t, "pl1", res.Link.RemoteID)
	require.Equal(t, 2, res.Link.Matched)
	require.Equal(t, 1, res.Link.Missing)
	require.Equal(t, "Not found: Nobody - Not In The Library", res.Link.Message)
	require.Equal(t, []models.PlaylistTrack{{Artist: "Nobody", Title: "Not In The Library"}}, res.Missing)
	require.Equal(t, []string{"s1", "s2"}, f.list("pl1"))

	// Edit the playlist in qui: drop Puddles, add Summer Static.
	_, err = svc.Update(ctx, p.ID, UpdateInput{Name: p.Name, Tracks: []models.PlaylistTrack{
		{Artist: "Nora Vale", Title: "Grey Windows"}, {Artist: "Kite Club", Title: "Summer Static"},
	}})
	require.NoError(t, err)
	_, err = svc.Sync(ctx, p.ID, link.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"s1", "s3"}, f.list("pl1"))
}

func TestCompareAndApply(t *testing.T) {
	svc, f, p, link := setupJellyfin(t)
	ctx := t.Context()

	_, err := svc.Compare(ctx, p.ID, link.ID)
	require.ErrorIs(t, err, ErrInvalid, "nothing to compare before the first sync")

	_, err = svc.Sync(ctx, p.ID, link.ID)
	require.NoError(t, err)
	// Someone adds Summer Static to the Jellyfin playlist.
	f.mu.Lock()
	f.playlists["pl1"] = append(f.playlists["pl1"], "s3")
	f.mu.Unlock()

	rows, err := svc.Compare(ctx, p.ID, link.ID)
	require.NoError(t, err)
	type side struct {
		title         string
		local, remote bool
	}
	var got []side
	for _, r := range rows {
		got = append(got, side{r.Title, r.Local, r.Remote})
	}
	require.Equal(t, []side{
		{"Grey Windows", true, true},
		{"Puddles", true, true},
		{"Not In The Library", true, false},
		{"Summer Static", false, true},
	}, got)

	// Keep Summer Static on both sides, drop Puddles from both.
	for i := range rows {
		rows[i].Keep = rows[i].Title != "Puddles"
	}
	updated, err := svc.Apply(ctx, p.ID, link.ID, rows)
	require.NoError(t, err)
	require.Equal(t, []string{"s1", "s3"}, f.list("pl1"))
	var titles []string
	for _, tr := range updated.Tracks {
		titles = append(titles, tr.Title)
	}
	require.Equal(t, []string{"Grey Windows", "Not In The Library", "Summer Static"}, titles)
	require.Equal(t, 2, updated.Links[0].Matched)
	require.Equal(t, 1, updated.Links[0].Missing)
}

func TestLinkExistingAndRemoteList(t *testing.T) {
	svc, f, p, link := setupJellyfin(t)
	ctx := t.Context()
	_, err := svc.Sync(ctx, p.ID, link.ID)
	require.NoError(t, err)

	lists, err := svc.RemotePlaylists(ctx, Account{Service: models.PlaylistJellyfin, ServerID: link.ServerID, AccountID: "u1"})
	require.NoError(t, err)
	require.Equal(t, []RemotePlaylist{{ID: "pl1", Name: "List pl1", Count: 2}}, lists)

	other, err := svc.Import(ctx, ImportInput{Text: "Kite Club - Summer Static"})
	require.NoError(t, err)
	existing, err := svc.CreateLink(ctx, other.ID, LinkInput{
		Account: Account{Service: models.PlaylistJellyfin, ServerID: link.ServerID, AccountID: "u1"}, RemoteID: "pl1", RemoteName: "List pl1",
	})
	require.NoError(t, err)
	rows, err := svc.Compare(ctx, other.ID, existing.ID)
	require.NoError(t, err)
	require.Len(t, rows, 3, "Summer Static from qui, Grey Windows and Puddles from Jellyfin")
	require.Equal(t, []string{"s1", "s2"}, f.list("pl1"), "compare changes nothing")

	text, err := svc.ExportLibrary(ctx, Account{Service: models.PlaylistJellyfin, ServerID: link.ServerID, AccountID: "u1"})
	require.NoError(t, err)
	require.Contains(t, text, "# Music library of Home: 3 tracks by 3 artists")
}

func TestLinkValidation(t *testing.T) {
	svc, _, p, _ := setupJellyfin(t)
	ctx := t.Context()

	_, err := svc.CreateLink(ctx, p.ID, LinkInput{Account: Account{Service: "napster"}})
	require.ErrorIs(t, err, ErrInvalid)
	_, err = svc.CreateLink(ctx, p.ID, LinkInput{Account: Account{Service: models.PlaylistPlex}})
	require.ErrorIs(t, err, ErrInvalid, "Plex needs a server and an account")
	_, err = svc.CreateLink(ctx, p.ID, LinkInput{Account: Account{Service: models.PlaylistSpotify}})
	require.ErrorIs(t, err, ErrInvalid, "Spotify is not connected")
}
