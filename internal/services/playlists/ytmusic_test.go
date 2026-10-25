// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/models"
)

const testCookie = "PREF=a=b; __Secure-3PAPISID=sid123/abc; SID=xyz"

func TestParseYTMusicHeaders(t *testing.T) {
	want := ytHeaders{"cookie": testCookie, "x-goog-authuser": "1", "user-agent": "Mozilla/5.0 Test"}
	for name, raw := range map[string]string{
		"firefox": "POST /youtubei/v1/browse?prettyPrint=false HTTP/2\nHost: music.youtube.com\nUser-Agent: Mozilla/5.0 Test\n" +
			"X-Goog-AuthUser: 1\nCookie: " + testCookie + "\nContent-Type: application/json\n",
		"chrome raw lines": ":authority:\nmusic.youtube.com\ncookie:\n" + testCookie + "\nuser-agent:\nMozilla/5.0 Test\nx-goog-authuser:\n1\n",
		"curl": "curl 'https://music.youtube.com/youtubei/v1/browse' -H 'user-agent: Mozilla/5.0 Test' " +
			"-H 'x-goog-authuser: 1' -b '" + testCookie + "' --data-raw '{}'",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ParseYTMusicHeaders(raw)
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}

	_, err := ParseYTMusicHeaders("Cookie: SID=xyz\n")
	require.ErrorIs(t, err, ErrInvalid, "a cookie without __Secure-3PAPISID cannot sign requests")
}

func TestSapisidHash(t *testing.T) {
	// sha1("1700000000 sid123/abc https://music.youtube.com")
	require.Equal(t, "SAPISIDHASH 1700000000_3e010c92a6372b2430298e06fa40ded0b595fd5f",
		sapisidHash("sid123/abc", time.Unix(1700000000, 0)))
}

// ytItem builds a musicResponsiveListItemRenderer as YouTube Music sends it.
func ytItem(videoID, setVideoID, title, artist, album string) map[string]any {
	link := func(text, pageType string) map[string]any {
		return map[string]any{"text": text, "navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{
			"browseEndpointContextSupportedConfigs": map[string]any{"browseEndpointContextMusicConfig": map[string]any{"pageType": pageType}},
		}}}
	}
	col := func(runs ...any) map[string]any {
		return map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": runs}}}
	}
	data := map[string]any{"videoId": videoID}
	if setVideoID != "" {
		data["playlistSetVideoId"] = setVideoID
	}
	return map[string]any{"musicResponsiveListItemRenderer": map[string]any{
		"playlistItemData": data,
		"flexColumns": []any{
			col(map[string]any{"text": title}),
			col(link(artist, "MUSIC_PAGE_TYPE_ARTIST"), map[string]any{"text": " • "}, link(album, "MUSIC_PAGE_TYPE_ALBUM"), map[string]any{"text": " • 3:21"}),
		},
	}}
}

func TestYTMusicTarget(t *testing.T) {
	var mu sync.Mutex
	var edits []any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "SAPISIDHASH ") || r.Header.Get("Cookie") != testCookie {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch strings.TrimPrefix(r.URL.Path, "/youtubei/v1/") {
		case "account/account_menu":
			write(map[string]any{"actions": []any{map[string]any{"openPopupAction": map[string]any{"popup": map[string]any{"multiPageMenuRenderer": map[string]any{
				"header": map[string]any{"activeAccountHeaderRenderer": map[string]any{"accountName": map[string]any{"runs": []any{map[string]any{"text": "Gab"}}}}}}}}}}})
		case "search":
			write(map[string]any{"contents": []any{
				ytItem("cover1", "", "Grey Windows", "Cover Band", "Covers"),
				ytItem("gw1", "", "Grey Windows", "Nora Vale", "Weather"),
			}})
		case "browse":
			switch {
			case body["browseId"] == "FEmusic_liked_playlists":
				two := func(id, title string) map[string]any {
					return map[string]any{"musicTwoRowItemRenderer": map[string]any{
						"title":              map[string]any{"runs": []any{map[string]any{"text": title}}},
						"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": id}},
					}}
				}
				write(map[string]any{"items": []any{two("", "New playlist"), two("VLLM", "Liked Music"), two("VLPL1", "Road trip")}})
			case body["browseId"] == "VLPL1":
				write(map[string]any{"contents": []any{
					ytItem("gw1", "set1", "Grey Windows", "Nora Vale", "Weather"),
					map[string]any{"continuationItemRenderer": map[string]any{"continuationEndpoint": map[string]any{"continuationCommand": map[string]any{"token": "next"}}}},
				}})
			case body["continuation"] == "next":
				write(map[string]any{"onResponseReceivedActions": []any{map[string]any{"appendContinuationItemsAction": map[string]any{
					"continuationItems": []any{ytItem("pd1", "set2", "Puddles", "The Lanterns", "Wet")}}}}})
			}
		case "playlist/create":
			write(map[string]any{"playlistId": "PLnew"})
		case "browse/edit_playlist":
			mu.Lock()
			edits = append(edits, body["actions"])
			mu.Unlock()
			write(map[string]any{"status": "STATUS_SUCCEEDED"})
		}
	}))
	t.Cleanup(server.Close)

	store, _ := testStores(t)
	svc := NewService(store, nil)
	svc.ytBase = server.URL
	ctx := t.Context()

	name, err := svc.SaveYTMusic(ctx, "Cookie: "+testCookie+"\nX-Goog-AuthUser: 0\n")
	require.NoError(t, err)
	require.Equal(t, "Gab", name)

	tg, err := svc.target(ctx, Account{Service: models.PlaylistYTMusic})
	require.NoError(t, err)

	ids, err := tg.match(ctx, []models.PlaylistTrack{{Artist: "Nora Vale", Title: "Grey Windows"}})
	require.NoError(t, err)
	require.Equal(t, []string{"gw1"}, ids)

	lists, err := tg.playlists(ctx)
	require.NoError(t, err)
	require.Equal(t, []RemotePlaylist{{ID: "PL1", Name: "Road trip"}}, lists)

	items, err := tg.items(ctx, "PL1")
	require.NoError(t, err)
	require.Equal(t, []RemoteItem{
		{TrackID: "gw1", EntryID: "set1", Artist: "Nora Vale", Title: "Grey Windows", Album: "Weather"},
		{TrackID: "pd1", EntryID: "set2", Artist: "The Lanterns", Title: "Puddles", Album: "Wet"},
	}, items, "the second page comes from the continuation")

	id, err := tg.create(ctx, "Mix", "", ids)
	require.NoError(t, err)
	require.Equal(t, "PLnew", id)

	require.NoError(t, tg.edit(ctx, "PL1", []string{"x1"}, items[:1]))
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []any{[]any{
		map[string]any{"action": "ACTION_REMOVE_VIDEO", "setVideoId": "set1", "removedVideoId": "gw1"},
		map[string]any{"action": "ACTION_ADD_VIDEO", "addedVideoId": "x1"},
	}}, edits)
}
