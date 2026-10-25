// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/models"
)

// fakeSpotify answers the accounts and Web API endpoints qui uses.
type fakeSpotify struct {
	*httptest.Server
	mu       sync.Mutex
	requests []string
	bodies   map[string]string
}

func newFakeSpotify(t *testing.T) *fakeSpotify {
	t.Helper()
	f := &fakeSpotify{bodies: map[string]string{}}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		f.bodies[r.Method+" "+r.URL.Path] = string(body)
		f.mu.Unlock()

		if r.URL.Path == "/api/token" {
			id, secret, _ := r.BasicAuth()
			form, _ := url.ParseQuery(string(body))
			switch {
			case id != "cid" || secret != "secret":
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"Invalid client"}`))
			case form.Get("grant_type") == "authorization_code" && form.Get("code") == "the-code":
				_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"rt"}`))
			case form.Get("grant_type") == "refresh_token" && form.Get("refresh_token") == "rt":
				_, _ = w.Write([]byte(`{"access_token":"at"}`))
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
			return
		}
		if r.Header.Get("Authorization") != "Bearer at" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v1/me":
			_, _ = w.Write([]byte(`{"id":"me1","display_name":"Gab"}`))
		case "/v1/search":
			q := r.URL.Query().Get("q")
			if strings.Contains(q, "Grey Windows") {
				_, _ = w.Write([]byte(`{"tracks":{"items":[
					{"uri":"spotify:track:cover","name":"Grey Windows","artists":[{"name":"Cover Band"}]},
					{"uri":"spotify:track:gw","name":"Grey Windows","artists":[{"name":"Nora Vale"}],"album":{"name":"Weather"}}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"tracks":{"items":[]}}`))
		case "/v1/me/playlists":
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{"id":"newpl"}`))
				return
			}
			_, _ = w.Write([]byte(`{"items":[
				{"id":"mine","name":"Mine","owner":{"id":"me1"},"items":{"total":3}},
				{"id":"theirs","name":"Followed","owner":{"id":"other"},"items":{"total":9}}],"next":null}`))
		case "/v1/playlists/mine/items":
			if r.Method != http.MethodGet {
				_, _ = w.Write([]byte(`{"snapshot_id":"x"}`))
				return
			}
			_, _ = w.Write([]byte(`{"items":[{"item":{"uri":"spotify:track:gw","name":"Grey Windows","artists":[{"name":"Nora Vale"}],"album":{"name":"Weather"}}}],"next":null}`))
		default:
			_, _ = w.Write([]byte(`{"snapshot_id":"x"}`))
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func TestSpotifySignInAndTarget(t *testing.T) {
	f := newFakeSpotify(t)
	store, _ := testStores(t)
	svc := NewService(store, nil)
	svc.spotify = spotifyURLs{accounts: f.URL, api: f.URL + "/v1"}
	ctx := t.Context()

	_, err := svc.SpotifyAuthorizeURL(ctx)
	require.ErrorIs(t, err, ErrInvalid, "no app saved yet")
	require.NoError(t, svc.SaveSpotifyApp(ctx, "cid", "secret"))

	authURL, err := svc.SpotifyAuthorizeURL(ctx)
	require.NoError(t, err)
	parsed, err := url.Parse(authURL)
	require.NoError(t, err)
	require.Equal(t, SpotifyRedirectURI, parsed.Query().Get("redirect_uri"))
	state := parsed.Query().Get("state")

	_, err = svc.ConnectSpotify(ctx, SpotifyRedirectURI+"?code=the-code&state=forged")
	require.ErrorIs(t, err, ErrInvalid)
	name, err := svc.ConnectSpotify(ctx, SpotifyRedirectURI+"?code=the-code&state="+state)
	require.NoError(t, err)
	require.Equal(t, "Gab", name)

	status, err := svc.Services(ctx)
	require.NoError(t, err)
	require.True(t, status.Spotify.Connected)
	require.Equal(t, "Gab", status.Spotify.AccountName)

	tg, err := svc.target(ctx, Account{Service: models.PlaylistSpotify})
	require.NoError(t, err)

	ids, err := tg.match(ctx, []models.PlaylistTrack{{Artist: "Nora Vale", Title: "Grey Windows"}, {Artist: "Nobody", Title: "Nothing"}})
	require.NoError(t, err)
	require.Equal(t, []string{"spotify:track:gw", ""}, ids, "the cover by another artist is skipped")

	lists, err := tg.playlists(ctx)
	require.NoError(t, err)
	require.Equal(t, []RemotePlaylist{{ID: "mine", Name: "Mine", Count: 3}}, lists, "only owned playlists")

	items, err := tg.items(ctx, "mine")
	require.NoError(t, err)
	require.Equal(t, []RemoteItem{{TrackID: "spotify:track:gw", EntryID: "spotify:track:gw", Artist: "Nora Vale", Title: "Grey Windows", Album: "Weather"}}, items)

	require.NoError(t, tg.edit(ctx, "mine", []string{"spotify:track:x"}, items))
	f.mu.Lock()
	var del, add map[string]any
	require.NoError(t, json.Unmarshal([]byte(f.bodies["DELETE /v1/playlists/mine/items"]), &del))
	require.NoError(t, json.Unmarshal([]byte(f.bodies["POST /v1/playlists/mine/items"]), &add))
	f.mu.Unlock()
	require.Equal(t, map[string]any{"items": []any{map[string]any{"uri": "spotify:track:gw"}}}, del)
	require.Equal(t, map[string]any{"uris": []any{"spotify:track:x"}}, add)

	id, err := tg.create(ctx, "Rainy", "Slow songs", []string{"spotify:track:gw"})
	require.NoError(t, err)
	require.Equal(t, "newpl", id)

	require.NoError(t, svc.SaveSpotifyApp(ctx, "other-client", ""))
	status, err = svc.Services(ctx)
	require.NoError(t, err)
	require.False(t, status.Spotify.Connected, "a new client ID signs out")
}
