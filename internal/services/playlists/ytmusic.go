// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"context"
	"crypto/sha1" //nolint:gosec // YouTube's SAPISIDHASH is defined as SHA-1.
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gjhuerte/qui/internal/models"
)

// YouTube Music has no public API. qui talks to the private one the web app
// uses, signed in with the cookie of the user's browser, as the ytmusicapi
// Python library does. Google can change it at any time.

const (
	ytMusicBase   = "https://music.youtube.com"
	ytMusicOrigin = "https://music.youtube.com"
	// The public key of the web app, sent by ytmusicapi with browser sign-ins.
	ytMusicKey = "AIzaSyC9XL3ZjWddXya6X74dJoCTL-WEYFDNX30"
	// songsParams is the search filter "Songs".
	ytSongsParams = "EgWKAQIIAWoMEA4QChADEAQQCRAF"
	ytUserAgent   = "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0"
)

// ytHeaders are the request headers kept from the browser.
type ytHeaders map[string]string

// keptHeaders are the browser headers qui sends; the rest are dropped.
var keptHeaders = map[string]bool{"cookie": true, "x-goog-authuser": true, "user-agent": true, "x-goog-visitor-id": true}

var curlHeader = regexp.MustCompile(`(?:-H|--header)\s+(?:'([^']*)'|"([^"]*)")`)

// ParseYTMusicHeaders reads request headers copied from the browser: "Copy
// request headers" from Firefox or Chrome, the raw "name:" / value lines
// Chrome shows, or "Copy as cURL".
func ParseYTMusicHeaders(raw string) (ytHeaders, error) {
	headers := ytHeaders{}
	put := func(name, value string) {
		name = strings.ToLower(strings.TrimSpace(name))
		if keptHeaders[name] {
			headers[name] = strings.TrimSpace(value)
		}
	}

	if strings.HasPrefix(strings.TrimSpace(raw), "curl ") {
		for _, m := range curlHeader.FindAllStringSubmatch(raw, -1) {
			h := m[1] + m[2]
			if name, value, ok := strings.Cut(h, ":"); ok {
				put(name, value)
			}
		}
		// curl sends cookies with -b.
		if m := regexp.MustCompile(`(?:-b|--cookie)\s+(?:'([^']*)'|"([^"]*)")`).FindStringSubmatch(raw); m != nil {
			put("cookie", m[1]+m[2])
		}
	} else {
		pending := ""
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}
			if name, value, ok := strings.Cut(line, ": "); ok && !strings.Contains(name, " ") {
				put(name, value)
				pending = ""
				continue
			}
			if strings.HasSuffix(line, ":") && !strings.Contains(line, " ") {
				pending = strings.TrimSuffix(line, ":")
				continue
			}
			if pending != "" {
				put(pending, line)
				pending = ""
			}
		}
	}

	if headers["cookie"] == "" {
		return nil, fmt.Errorf("%w: no cookie found. Copy the headers of a request to music.youtube.com while you are signed in", ErrInvalid)
	}
	if sapisid(headers["cookie"]) == "" {
		return nil, fmt.Errorf("%w: the cookie has no __Secure-3PAPISID. Copy the headers of a POST request to music.youtube.com/youtubei/v1/browse while you are signed in", ErrInvalid)
	}
	if headers["x-goog-authuser"] == "" {
		headers["x-goog-authuser"] = "0"
	}
	return headers, nil
}

func sapisid(cookie string) string {
	for _, part := range strings.Split(cookie, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && (name == "__Secure-3PAPISID" || name == "SAPISID") {
			return strings.Trim(value, `"`)
		}
	}
	return ""
}

// sapisidHash is the Authorization YouTube expects from a signed-in browser.
func sapisidHash(sid string, now time.Time) string {
	ts := strconv.FormatInt(now.Unix(), 10)
	sum := sha1.Sum([]byte(ts + " " + sid + " " + ytMusicOrigin)) //nolint:gosec // defined by YouTube
	return "SAPISIDHASH " + ts + "_" + hex.EncodeToString(sum[:])
}

// SaveYTMusic checks pasted browser headers by asking for the account name,
// then stores them.
func (s *Service) SaveYTMusic(ctx context.Context, raw string) (string, error) {
	headers, err := ParseYTMusicHeaders(raw)
	if err != nil {
		return "", err
	}
	yt := &ytMusic{base: s.ytBase, headers: headers, http: s.http}
	name, err := yt.accountName(ctx)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(headers)
	if err != nil {
		return "", err
	}
	return name, s.store.SaveServiceSettings(ctx, &models.PlaylistServiceSettings{
		Service: models.PlaylistYTMusic, Settings: string(data), AccountName: name,
	})
}

func (s *Service) ytMusicTarget(ctx context.Context) (target, error) {
	stored, err := s.store.GetServiceSettings(ctx, models.PlaylistYTMusic)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, fmt.Errorf("%w: connect YouTube Music on the Playlist services page first", ErrInvalid)
	}
	headers := ytHeaders{}
	if err := json.Unmarshal([]byte(stored.Settings), &headers); err != nil {
		return nil, err
	}
	return &ytMusic{base: s.ytBase, headers: headers, http: s.http}, nil
}

type ytMusic struct {
	base    string
	headers ytHeaders
	http    *http.Client
}

var errYTSignedOut = errors.New("YouTube Music signed qui out: paste new browser headers on the Playlist services page")

func (yt *ytMusic) call(ctx context.Context, endpoint string, body map[string]any) (any, error) {
	body["context"] = map[string]any{
		"client": map[string]any{
			"clientName":    "WEB_REMIX",
			"clientVersion": "1." + time.Now().UTC().Format("20060102") + ".01.00",
			"hl":            "en",
		},
		"user": map[string]any{},
	}
	header := http.Header{}
	for k, v := range yt.headers {
		header.Set(k, v)
	}
	if header.Get("User-Agent") == "" {
		header.Set("User-Agent", ytUserAgent)
	}
	header.Set("Origin", ytMusicOrigin)
	header.Set("X-Origin", ytMusicOrigin)
	header.Set("Authorization", sapisidHash(sapisid(yt.headers["cookie"]), time.Now()))

	var out any
	err := send(ctx, yt.http, request{
		method: http.MethodPost,
		url:    yt.base + "/youtubei/v1/" + endpoint + "?alt=json&key=" + ytMusicKey,
		header: header,
		body:   body,
	}, &out)
	var he *httpError
	if errors.As(err, &he) && (he.Status == http.StatusUnauthorized || he.Status == http.StatusForbidden) {
		return nil, errYTSignedOut
	}
	return out, err
}

// The answers are deep trees of renderers whose layout YouTube changes
// often, so qui searches them for the few keys it needs instead of
// following fixed paths.

// walk calls fn for every object in the tree, depth first, in a stable order.
func walk(node any, fn func(map[string]any)) {
	switch v := node.(type) {
	case map[string]any:
		fn(v)
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walk(v[k], fn)
		}
	case []any:
		for _, item := range v {
			walk(item, fn)
		}
	}
}

// renderers returns every object stored under key, in document order.
func renderers(node any, key string) []map[string]any {
	var out []map[string]any
	walk(node, func(m map[string]any) {
		if r, ok := m[key].(map[string]any); ok {
			out = append(out, r)
		}
	})
	return out
}

// findString returns the first string stored under key anywhere in node.
func findString(node any, key string) string {
	found := ""
	walk(node, func(m map[string]any) {
		if found == "" {
			if s, ok := m[key].(string); ok {
				found = s
			}
		}
	})
	return found
}

// path follows keys and array indexes; it returns nil when one is missing.
func path(node any, steps ...any) any {
	for _, step := range steps {
		switch k := step.(type) {
		case string:
			m, ok := node.(map[string]any)
			if !ok {
				return nil
			}
			node = m[k]
		case int:
			a, ok := node.([]any)
			if !ok || k >= len(a) {
				return nil
			}
			node = a[k]
		}
	}
	return node
}

func runsText(runs any) string {
	var b strings.Builder
	for _, r := range asSlice(runs) {
		if t, ok := path(r, "text").(string); ok {
			b.WriteString(t)
		}
	}
	return b.String()
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

// listItem reads a song out of a musicResponsiveListItemRenderer: the title
// from the first column, the artists and album from the links to their
// pages in the other columns.
func listItem(r map[string]any) (videoID, setVideoID string, song Song) {
	columns := asSlice(r["flexColumns"])
	if len(columns) == 0 {
		return "", "", Song{}
	}
	song.Title = runsText(path(columns[0], "musicResponsiveListItemFlexColumnRenderer", "text", "runs"))
	var firstText string
	for _, col := range columns[1:] {
		for _, run := range asSlice(path(col, "musicResponsiveListItemFlexColumnRenderer", "text", "runs")) {
			text, _ := path(run, "text").(string)
			if firstText == "" && strings.TrimSpace(text) != "" {
				firstText = text
			}
			pageType, _ := path(run, "navigationEndpoint", "browseEndpoint", "browseEndpointContextSupportedConfigs",
				"browseEndpointContextMusicConfig", "pageType").(string)
			switch pageType {
			case "MUSIC_PAGE_TYPE_ARTIST", "MUSIC_PAGE_TYPE_USER_CHANNEL", "MUSIC_PAGE_TYPE_UNKNOWN":
				song.Artists = append(song.Artists, text)
			case "MUSIC_PAGE_TYPE_ALBUM":
				song.Album = text
			}
		}
	}
	if len(song.Artists) == 0 && firstText != "" {
		song.Artists = []string{firstText}
	}
	videoID, _ = path(r, "playlistItemData", "videoId").(string)
	if videoID == "" {
		videoID = findString(r, "videoId")
	}
	setVideoID, _ = path(r, "playlistItemData", "playlistSetVideoId").(string)
	if setVideoID == "" {
		setVideoID = findString(r, "setVideoId")
	}
	song.ID = videoID
	return videoID, setVideoID, song
}

func (yt *ytMusic) accountName(ctx context.Context) (string, error) {
	resp, err := yt.call(ctx, "account/account_menu", map[string]any{})
	if err != nil {
		return "", err
	}
	for _, header := range renderers(resp, "activeAccountHeaderRenderer") {
		if name := runsText(path(header, "accountName", "runs")); name != "" {
			return name, nil
		}
	}
	return "", errors.New("YouTube Music did not name the account: the headers may be from a signed-out page")
}

func (yt *ytMusic) search(ctx context.Context, q string) ([]Song, error) {
	resp, err := yt.call(ctx, "search", map[string]any{"query": q, "params": ytSongsParams})
	if err != nil {
		return nil, err
	}
	var out []Song
	for _, r := range renderers(resp, "musicResponsiveListItemRenderer") {
		if id, _, song := listItem(r); id != "" {
			out = append(out, song)
		}
	}
	return out, nil
}

func (yt *ytMusic) match(ctx context.Context, tracks []models.PlaylistTrack) ([]string, error) {
	ids := make([]string, len(tracks))
	for i, t := range tracks {
		songs, err := yt.search(ctx, strings.TrimSpace(t.Artist+" "+t.Title))
		if err != nil {
			return nil, err
		}
		if s, ok := best(t, songs); ok {
			ids[i] = s.ID
		}
	}
	return ids, nil
}

func (yt *ytMusic) playlists(ctx context.Context) ([]RemotePlaylist, error) {
	resp, err := yt.call(ctx, "browse", map[string]any{"browseId": "FEmusic_liked_playlists"})
	if err != nil {
		return nil, err
	}
	out := []RemotePlaylist{}
	for _, r := range renderers(resp, "musicTwoRowItemRenderer") {
		browseID, _ := path(r, "navigationEndpoint", "browseEndpoint", "browseId").(string)
		id := strings.TrimPrefix(browseID, "VL")
		// Liked Music and Episodes for Later are not editable lists.
		if !strings.HasPrefix(browseID, "VL") || id == "LM" || id == "SE" {
			continue
		}
		out = append(out, RemotePlaylist{ID: id, Name: runsText(path(r, "title", "runs"))})
	}
	return out, nil
}

// continuation returns the token for the next page of a playlist, or "".
func continuation(resp any) string {
	for _, r := range renderers(resp, "continuationItemRenderer") {
		if token, _ := path(r, "continuationEndpoint", "continuationCommand", "token").(string); token != "" {
			return token
		}
	}
	return ""
}

func (yt *ytMusic) items(ctx context.Context, playlistID string) ([]RemoteItem, error) {
	resp, err := yt.call(ctx, "browse", map[string]any{"browseId": "VL" + playlistID})
	if err != nil {
		return nil, err
	}
	out := []RemoteItem{}
	for page := 0; page < 100; page++ {
		for _, r := range renderers(resp, "musicResponsiveListItemRenderer") {
			id, setID, song := listItem(r)
			if id == "" {
				continue
			}
			artist := ""
			if len(song.Artists) > 0 {
				artist = song.Artists[0]
			}
			out = append(out, RemoteItem{TrackID: id, EntryID: setID, Artist: artist, Title: song.Title, Album: song.Album})
		}
		token := continuation(resp)
		if token == "" {
			break
		}
		if resp, err = yt.call(ctx, "browse", map[string]any{"continuation": token}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (yt *ytMusic) create(ctx context.Context, name, description string, trackIDs []string) (string, error) {
	resp, err := yt.call(ctx, "playlist/create", map[string]any{
		"title": name, "description": description, "privacyStatus": "PRIVATE", "videoIds": trackIDs,
	})
	if err != nil {
		return "", err
	}
	id, _ := path(resp, "playlistId").(string)
	if id == "" {
		return "", errors.New("YouTube Music did not create the playlist")
	}
	return id, nil
}

func (yt *ytMusic) edit(ctx context.Context, playlistID string, add []string, remove []RemoteItem) error {
	var actions []any
	for _, item := range remove {
		actions = append(actions, map[string]any{"action": "ACTION_REMOVE_VIDEO", "setVideoId": item.EntryID, "removedVideoId": item.TrackID})
	}
	for _, id := range add {
		actions = append(actions, map[string]any{"action": "ACTION_ADD_VIDEO", "addedVideoId": id})
	}
	for _, batch := range chunks(actions, 100) {
		resp, err := yt.call(ctx, "browse/edit_playlist", map[string]any{"playlistId": playlistID, "actions": batch})
		if err != nil {
			return err
		}
		if status, _ := path(resp, "status").(string); !strings.Contains(status, "SUCCEEDED") {
			return fmt.Errorf("YouTube Music did not change the playlist (status %q)", status)
		}
	}
	return nil
}
