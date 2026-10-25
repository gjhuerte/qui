// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package mediasync

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/models"
)

// fakeServer answers the routes given and records every request it saw.
type fakeServer struct {
	*httptest.Server
	mu   sync.Mutex
	seen []string
}

func newFakeServer(t *testing.T, auth func(r *http.Request) bool, routes map[string]string) *fakeServer {
	t.Helper()
	f := &fakeServer{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.mu.Lock()
		f.seen = append(f.seen, r.Method+" "+r.URL.RequestURI())
		f.mu.Unlock()
		key := r.Method + " " + r.URL.Path
		if t := r.URL.Query().Get("type"); t != "" {
			key += "?type=" + t
		}
		body, ok := routes[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeServer) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.seen...)
}

func TestJellyfinLibraryAndMark(t *testing.T) {
	server := newFakeServer(t, func(r *http.Request) bool {
		return r.Header.Get("Authorization") == `MediaBrowser Token="key"`
	}, map[string]string{
		"GET /Users": `[{"Id":"u1","Name":"Ann","Policy":{"IsAdministrator":true}},{"Id":"u2","Name":"Old","Policy":{"IsDisabled":true}}]`,
		"GET /Items": `{"Items":[
			{"Id":"m1","Name":"Example Movie","Type":"Movie","ProductionYear":1999,"ProviderIds":{"Imdb":"tt1"},"UserData":{"Played":true}},
			{"Id":"s1","Name":"Show","Type":"Series","ProviderIds":{"Tvdb":"100"}},
			{"Id":"e1","Name":"Pilot","Type":"Episode","SeriesId":"s1","SeriesName":"Show","ParentIndexNumber":1,"IndexNumber":1,"ProviderIds":{},"UserData":{"Played":false}}
		]}`,
		"POST /UserPlayedItems/e1": `{}`,
	})
	jf := &jellyfin{srv: &models.MediaServer{BaseURL: server.URL, Token: "key"}, http: server.Client()}
	ctx := t.Context()

	users, err := jf.users(ctx)
	require.NoError(t, err)
	require.Equal(t, []User{{ID: "u1", Name: "Ann", Admin: true}}, users)

	items, err := jf.library(ctx, "u1")
	require.NoError(t, err)
	require.Equal(t, []Item{
		{ID: "m1", Kind: kindMovie, Title: "Example Movie", Year: 1999, IDs: []string{"imdb:tt1"}, Watched: true},
		{ID: "e1", Kind: kindEpisode, Title: "Pilot", IDs: []string{}, Series: "Show", SeriesIDs: []string{"tvdb:100"}, Season: 1, Episode: 1},
	}, items)

	require.NoError(t, jf.markWatched(ctx, "u1", items[1]))
	require.Contains(t, server.requests(), "POST /UserPlayedItems/e1?userId=u1")

	bad := &jellyfin{srv: &models.MediaServer{BaseURL: server.URL, Token: "wrong"}, http: server.Client()}
	_, err = bad.users(ctx)
	require.ErrorContains(t, err, "check the token")
}

func TestPlexUsersLibraryAndMark(t *testing.T) {
	tokens := map[string]bool{"owner-token": true, "friend-token": true}
	auth := func(r *http.Request) bool { return tokens[r.Header.Get("X-Plex-Token")] }

	plexTV := newFakeServer(t, auth, map[string]string{
		"GET /api/v2/user": `{"id":1,"username":"owner","title":"Owner"}`,
		"GET /api/servers/machine-1/shared_servers": `<MediaContainer>
			<SharedServer userID="7" username="friend" accessToken="friend-token"/>
			<SharedServer userID="8" username="pending" accessToken=""/>
		</MediaContainer>`,
	})
	pms := newFakeServer(t, auth, map[string]string{
		"GET /identity":         `{"MediaContainer":{"machineIdentifier":"machine-1"}}`,
		"GET /library/sections": `{"MediaContainer":{"Directory":[{"key":"1","title":"Movies","type":"movie"},{"key":"2","title":"TV","type":"show"}]}}`,
		"GET /library/sections/1/all?type=1": `{"MediaContainer":{"Metadata":[
			{"ratingKey":"10","title":"Example Movie","year":1999,"viewCount":2,"Guid":[{"id":"imdb://tt1"}]}
		]}}`,
		"GET /library/sections/2/all?type=2": `{"MediaContainer":{"Metadata":[{"ratingKey":"20","title":"Show","Guid":[{"id":"tvdb://100"}]}]}}`,
		"GET /library/sections/2/all?type=4": `{"MediaContainer":{"Metadata":[
			{"ratingKey":"21","title":"Pilot","grandparentTitle":"Show","grandparentRatingKey":"20","parentIndex":1,"index":1}
		]}}`,
		"GET /:/scrobble": `{}`,
	})

	px := &plex{srv: &models.MediaServer{BaseURL: pms.URL, Token: "owner-token"}, http: pms.Client(), plexTV: plexTV.URL}
	ctx := t.Context()

	users, err := px.users(ctx)
	require.NoError(t, err)
	require.Equal(t, []User{{ID: "1", Name: "Owner", Admin: true}, {ID: "7", Name: "friend"}}, users)

	items, err := px.library(ctx, "7")
	require.NoError(t, err)
	require.Equal(t, []Item{
		{ID: "10", Kind: kindMovie, Title: "Example Movie", Year: 1999, IDs: []string{"imdb:tt1"}, Watched: true},
		{ID: "21", Kind: kindEpisode, Title: "Pilot", IDs: []string{}, Series: "Show", SeriesIDs: []string{"tvdb:100"}, Season: 1, Episode: 1},
	}, items)

	require.NoError(t, px.markWatched(ctx, "7", items[1]))
	require.Contains(t, pms.requests(), "GET /:/scrobble?identifier=com.plexapp.plugins.library&key=21")

	_, err = px.library(ctx, "8")
	require.ErrorContains(t, err, "no longer has access")
}
