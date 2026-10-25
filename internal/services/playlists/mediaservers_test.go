// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/models"
)

// fakeJellyfin keeps a music library and playlists in memory and answers
// the Jellyfin endpoints qui uses.
type fakeJellyfin struct {
	*httptest.Server
	mu        sync.Mutex
	songs     []jellyfinAudio
	playlists map[string][]string // playlist ID -> item IDs, in order
	nextEntry int
}

func newFakeJellyfin(t *testing.T, songs ...jellyfinAudio) *fakeJellyfin {
	t.Helper()
	f := &fakeJellyfin{songs: songs, playlists: map[string][]string{}}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeJellyfin) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != `MediaBrowser Token="key"` {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	q := r.URL.Query()
	write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	switch {
	case r.URL.Path == "/Items" && q.Get("IncludeItemTypes") == "Audio":
		write(map[string]any{"Items": f.songs})
	case r.URL.Path == "/Items" && q.Get("IncludeItemTypes") == "Playlist":
		var items []jellyfinAudio
		for id, entries := range f.playlists {
			items = append(items, jellyfinAudio{ID: id, Name: "List " + id, MediaType: "Audio", ChildCount: len(entries)})
		}
		write(map[string]any{"Items": items})
	case r.URL.Path == "/Playlists" && r.Method == http.MethodPost:
		var body struct {
			Name string
			Ids  []string
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := fmt.Sprintf("pl%d", len(f.playlists)+1)
		f.playlists[id] = append([]string{}, body.Ids...)
		write(map[string]string{"Id": id})
	case strings.HasPrefix(r.URL.Path, "/Playlists/") && strings.HasSuffix(r.URL.Path, "/Items"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/Playlists/"), "/Items")
		switch r.Method {
		case http.MethodGet:
			var items []jellyfinAudio
			for i, songID := range f.playlists[id] {
				for _, s := range f.songs {
					if s.ID == songID {
						s.PlaylistItemID = fmt.Sprintf("%s-%d", songID, i)
						items = append(items, s)
					}
				}
			}
			write(map[string]any{"Items": items})
		case http.MethodPost:
			f.playlists[id] = append(f.playlists[id], strings.Split(q.Get("ids"), ",")...)
		case http.MethodDelete:
			drop := map[string]bool{}
			for _, e := range strings.Split(q.Get("entryIds"), ",") {
				drop[e] = true
			}
			var kept []string
			for i, songID := range f.playlists[id] {
				if !drop[fmt.Sprintf("%s-%d", songID, i)] {
					kept = append(kept, songID)
				}
			}
			f.playlists[id] = kept
		}
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeJellyfin) list(id string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.playlists[id]...)
}

func song(id, artist, title string) jellyfinAudio {
	return jellyfinAudio{ID: id, Name: title, Artists: []string{artist}, Album: "Album " + id}
}

func TestJellyfinTarget(t *testing.T) {
	f := newFakeJellyfin(t, song("s1", "Nora Vale", "Grey Windows"), song("s2", "The Lanterns", "Puddles (Remastered)"))
	jf := &jellyfin{srv: &models.MediaServer{BaseURL: f.URL, Token: "key"}, userID: "u1", http: f.Client()}
	ctx := t.Context()

	ids, err := jf.match(ctx, []models.PlaylistTrack{
		{Artist: "Nora Vale", Title: "Grey Windows"},
		{Artist: "Lanterns", Title: "Puddles"},
		{Artist: "Nobody", Title: "Unknown"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"s1", "s2", ""}, ids)

	id, err := jf.create(ctx, "Mix", "", []string{"s1", "s2"})
	require.NoError(t, err)
	items, err := jf.items(ctx, id)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "s1-0", items[0].EntryID)

	require.NoError(t, jf.edit(ctx, id, []string{"s1"}, items[:1]))
	require.Equal(t, []string{"s2", "s1"}, f.list(id))

	lists, err := jf.playlists(ctx)
	require.NoError(t, err)
	require.Equal(t, []RemotePlaylist{{ID: id, Name: "List " + id, Count: 2}}, lists)
}

func TestPlexTarget(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "friend-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.RequestURI())
		mu.Unlock()
		switch r.URL.Path {
		case "/identity":
			_, _ = w.Write([]byte(`{"MediaContainer":{"machineIdentifier":"mid"}}`))
		case "/library/sections":
			_, _ = w.Write([]byte(`{"MediaContainer":{"Directory":[{"key":"3","type":"artist"},{"key":"1","type":"movie"}]}}`))
		case "/library/sections/3/all":
			_, _ = w.Write([]byte(`{"MediaContainer":{"Metadata":[
				{"ratingKey":"31","title":"Grey Windows","grandparentTitle":"Nora Vale","parentTitle":"Weather"},
				{"ratingKey":"32","title":"Puddles","grandparentTitle":"Various Artists","originalTitle":"The Lanterns"}]}}`))
		case "/playlists":
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"900"}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"MediaContainer":{"Metadata":[
				{"ratingKey":"900","title":"Mix","leafCount":2,"smart":false},
				{"ratingKey":"901","title":"Recently added","smart":true}]}}`))
		case "/playlists/900/items":
			_, _ = w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"31","playlistItemID":7001,"title":"Grey Windows","grandparentTitle":"Nora Vale"}]}}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(server.Close)

	px := &plex{srv: &models.MediaServer{BaseURL: server.URL}, token: "friend-token", http: server.Client()}
	ctx := t.Context()

	ids, err := px.match(ctx, []models.PlaylistTrack{{Artist: "Nora Vale", Title: "Grey Windows"}, {Artist: "The Lanterns", Title: "Puddles"}})
	require.NoError(t, err)
	require.Equal(t, []string{"31", "32"}, ids)

	id, err := px.create(ctx, "Mix", "", ids)
	require.NoError(t, err)
	require.Equal(t, "900", id)

	lists, err := px.playlists(ctx)
	require.NoError(t, err)
	require.Equal(t, []RemotePlaylist{{ID: "900", Name: "Mix", Count: 2}}, lists, "smart playlists are left out")

	items, err := px.items(ctx, "900")
	require.NoError(t, err)
	require.Equal(t, []RemoteItem{{TrackID: "31", EntryID: "7001", Artist: "Nora Vale", Title: "Grey Windows"}}, items)

	require.NoError(t, px.edit(ctx, "900", []string{"32"}, items))
	mu.Lock()
	defer mu.Unlock()
	require.Contains(t, seen, "POST /playlists?smart=0&title=Mix&type=audio&uri=server%3A%2F%2Fmid%2Fcom.plexapp.plugins.library%2Flibrary%2Fmetadata%2F31%2C32")
	require.Contains(t, seen, "DELETE /playlists/900/items/7001")
	require.Contains(t, seen, "PUT /playlists/900/items?uri=server%3A%2F%2Fmid%2Fcom.plexapp.plugins.library%2Flibrary%2Fmetadata%2F32")
}
