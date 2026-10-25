// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gjhuerte/qui/internal/models"
)

// SpotifyRedirectURI is the address Spotify sends the browser to after the
// sign-in. Nothing listens there: the user copies the address from the
// browser and pastes it into qui. A loopback address is the only http
// address Spotify accepts, and it works wherever qui runs.
const SpotifyRedirectURI = "http://127.0.0.1:8888/callback"

const spotifyScopes = "playlist-read-private playlist-modify-private playlist-modify-public"

type spotifyURLs struct {
	accounts string
	api      string
}

var defaultSpotifyURLs = spotifyURLs{accounts: "https://accounts.spotify.com", api: "https://api.spotify.com/v1"}

type spotifySettings struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	RefreshToken string `json:"refreshToken,omitempty"`
}

func (s *Service) loadSpotify(ctx context.Context) (*spotifySettings, *models.PlaylistServiceSettings, error) {
	stored, err := s.store.GetServiceSettings(ctx, models.PlaylistSpotify)
	if err != nil {
		return nil, nil, err
	}
	settings := &spotifySettings{}
	if stored != nil {
		if err := json.Unmarshal([]byte(stored.Settings), settings); err != nil {
			return nil, nil, err
		}
	}
	return settings, stored, nil
}

func (s *Service) saveSpotify(ctx context.Context, settings *spotifySettings, accountName string) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return s.store.SaveServiceSettings(ctx, &models.PlaylistServiceSettings{
		Service: models.PlaylistSpotify, Settings: string(data), AccountName: accountName,
	})
}

// SaveSpotifyApp stores the client ID and secret of the user's Spotify app.
// An empty secret keeps the stored one. A new client ID signs out.
func (s *Service) SaveSpotifyApp(ctx context.Context, clientID, clientSecret string) error {
	clientID, clientSecret = strings.TrimSpace(clientID), strings.TrimSpace(clientSecret)
	if clientID == "" {
		return fmt.Errorf("%w: the client ID is required", ErrInvalid)
	}
	settings, stored, err := s.loadSpotify(ctx)
	if err != nil {
		return err
	}
	accountName := ""
	if stored != nil {
		accountName = stored.AccountName
	}
	if settings.ClientID != clientID {
		settings.RefreshToken, accountName = "", ""
	}
	settings.ClientID = clientID
	if clientSecret != "" {
		settings.ClientSecret = clientSecret
	}
	if settings.ClientSecret == "" {
		return fmt.Errorf("%w: the client secret is required", ErrInvalid)
	}
	return s.saveSpotify(ctx, settings, accountName)
}

// SpotifyAuthorizeURL starts a sign-in and returns the Spotify page to open.
func (s *Service) SpotifyAuthorizeURL(ctx context.Context) (string, error) {
	settings, _, err := s.loadSpotify(ctx)
	if err != nil {
		return "", err
	}
	if settings.ClientID == "" {
		return "", fmt.Errorf("%w: save the client ID and secret first", ErrInvalid)
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	state := hex.EncodeToString(buf)

	s.mu.Lock()
	for st, at := range s.spotifyStates {
		if time.Since(at) > time.Hour {
			delete(s.spotifyStates, st)
		}
	}
	s.spotifyStates[state] = time.Now()
	s.mu.Unlock()

	query := url.Values{
		"client_id":     {settings.ClientID},
		"response_type": {"code"},
		"redirect_uri":  {SpotifyRedirectURI},
		"scope":         {spotifyScopes},
		"state":         {state},
	}
	return s.spotify.accounts + "/authorize?" + query.Encode(), nil
}

type spotifyToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (s *Service) spotifyTokenRequest(ctx context.Context, settings *spotifySettings, form url.Values) (*spotifyToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.spotify.accounts+"/api/token", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(settings.ClientID, settings.ClientSecret)
	var token spotifyToken
	err = send(ctx, s.http, request{method: http.MethodPost, url: req.URL.String(), header: req.Header, form: form}, &token)
	return &token, err
}

// ConnectSpotify finishes a sign-in with the address the browser landed on,
// and returns the Spotify account name.
func (s *Service) ConnectSpotify(ctx context.Context, redirected string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(redirected))
	if err != nil {
		return "", fmt.Errorf("%w: paste the whole address from the browser", ErrInvalid)
	}
	query := parsed.Query()
	if e := query.Get("error"); e != "" {
		return "", fmt.Errorf("%w: Spotify said %q", ErrInvalid, e)
	}
	code, state := query.Get("code"), query.Get("state")
	if code == "" {
		return "", fmt.Errorf("%w: the address has no code; paste the whole address from the browser", ErrInvalid)
	}
	s.mu.Lock()
	_, known := s.spotifyStates[state]
	delete(s.spotifyStates, state)
	s.mu.Unlock()
	if !known {
		return "", fmt.Errorf("%w: this sign-in was not started here or is older than an hour; click Sign in again", ErrInvalid)
	}

	settings, _, err := s.loadSpotify(ctx)
	if err != nil {
		return "", err
	}
	token, err := s.spotifyTokenRequest(ctx, settings, url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {SpotifyRedirectURI},
	})
	if err != nil {
		return "", fmt.Errorf("spotify: %w", err)
	}
	settings.RefreshToken = token.RefreshToken

	sp := &spotify{api: s.spotify.api, token: token.AccessToken, http: s.http}
	me, err := sp.me(ctx)
	if err != nil {
		return "", fmt.Errorf("spotify: %w", err)
	}
	name := me.DisplayName
	if name == "" {
		name = me.ID
	}
	return name, s.saveSpotify(ctx, settings, name)
}

func (s *Service) spotifyTarget(ctx context.Context) (target, error) {
	settings, stored, err := s.loadSpotify(ctx)
	if err != nil {
		return nil, err
	}
	if settings.RefreshToken == "" {
		return nil, fmt.Errorf("%w: sign in to Spotify on the Playlist services page first", ErrInvalid)
	}
	token, err := s.spotifyTokenRequest(ctx, settings, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {settings.RefreshToken},
	})
	if err != nil {
		return nil, fmt.Errorf("spotify: %w", err)
	}
	// Spotify may rotate the refresh token.
	if token.RefreshToken != "" && token.RefreshToken != settings.RefreshToken {
		settings.RefreshToken = token.RefreshToken
		if err := s.saveSpotify(ctx, settings, stored.AccountName); err != nil {
			return nil, err
		}
	}
	return &spotify{api: s.spotify.api, token: token.AccessToken, http: s.http}, nil
}

// spotify uses the Web API as changed in February 2026 for apps in
// Development Mode: /me/playlists and /playlists/{id}/items.
type spotify struct {
	api   string
	token string
	http  *http.Client
	meID  string
}

func (sp *spotify) call(ctx context.Context, method, target string, body, out any) error {
	if !strings.HasPrefix(target, "http") {
		target = sp.api + target
	}
	header := http.Header{}
	header.Set("Authorization", "Bearer "+sp.token)
	return send(ctx, sp.http, request{method: method, url: target, header: header, body: body}, out)
}

type spotifyUser struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

func (sp *spotify) me(ctx context.Context) (*spotifyUser, error) {
	var me spotifyUser
	if err := sp.call(ctx, http.MethodGet, "/me", nil, &me); err != nil {
		return nil, err
	}
	sp.meID = me.ID
	return &me, nil
}

type spotifyTrack struct {
	URI     string `json:"uri"`
	Name    string `json:"name"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		Name string `json:"name"`
	} `json:"album"`
}

func (t spotifyTrack) song() Song {
	s := Song{ID: t.URI, Title: t.Name, Album: t.Album.Name}
	for _, a := range t.Artists {
		s.Artists = append(s.Artists, a.Name)
	}
	return s
}

func (sp *spotify) search(ctx context.Context, q string) ([]Song, error) {
	var resp struct {
		Tracks struct {
			Items []spotifyTrack `json:"items"`
		} `json:"tracks"`
	}
	query := url.Values{"q": {q}, "type": {"track"}, "limit": {"10"}}
	if err := sp.call(ctx, http.MethodGet, "/search?"+query.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	out := make([]Song, 0, len(resp.Tracks.Items))
	for _, t := range resp.Tracks.Items {
		out = append(out, t.song())
	}
	return out, nil
}

// match searches each track by field first, then as plain words, which finds
// titles that the field search misses over punctuation.
func (sp *spotify) match(ctx context.Context, tracks []models.PlaylistTrack) ([]string, error) {
	ids := make([]string, len(tracks))
	for i, t := range tracks {
		queries := []string{fmt.Sprintf("track:%s artist:%s", t.Title, t.Artist), t.Artist + " " + t.Title}
		if t.Artist == "" {
			queries = []string{t.Title}
		}
		for _, q := range queries {
			songs, err := sp.search(ctx, q)
			if err != nil {
				return nil, err
			}
			if s, ok := best(t, songs); ok {
				ids[i] = s.ID
				break
			}
		}
	}
	return ids, nil
}

// playlists lists the playlists the user owns: Development Mode apps can
// only read and edit the items of those.
func (sp *spotify) playlists(ctx context.Context) ([]RemotePlaylist, error) {
	if sp.meID == "" {
		if _, err := sp.me(ctx); err != nil {
			return nil, err
		}
	}
	out := []RemotePlaylist{}
	next := "/me/playlists?limit=50"
	for next != "" {
		var resp struct {
			Items []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Owner struct {
					ID string `json:"id"`
				} `json:"owner"`
				// "items" since February 2026; "tracks" before.
				Items  *struct{ Total int } `json:"items"`
				Tracks *struct{ Total int } `json:"tracks"`
			} `json:"items"`
			Next string `json:"next"`
		}
		if err := sp.call(ctx, http.MethodGet, next, nil, &resp); err != nil {
			return nil, err
		}
		for _, p := range resp.Items {
			if p.Owner.ID != sp.meID {
				continue
			}
			count := 0
			if p.Items != nil {
				count = p.Items.Total
			} else if p.Tracks != nil {
				count = p.Tracks.Total
			}
			out = append(out, RemotePlaylist{ID: p.ID, Name: p.Name, Count: count})
		}
		next = resp.Next
	}
	return out, nil
}

func (sp *spotify) items(ctx context.Context, playlistID string) ([]RemoteItem, error) {
	out := []RemoteItem{}
	next := "/playlists/" + url.PathEscape(playlistID) + "/items?limit=50"
	for next != "" {
		var resp struct {
			Items []struct {
				// "item" since February 2026; "track" before.
				Item  *spotifyTrack `json:"item"`
				Track *spotifyTrack `json:"track"`
			} `json:"items"`
			Next string `json:"next"`
		}
		if err := sp.call(ctx, http.MethodGet, next, nil, &resp); err != nil {
			return nil, err
		}
		for _, entry := range resp.Items {
			t := entry.Item
			if t == nil {
				t = entry.Track
			}
			if t == nil || t.URI == "" {
				continue
			}
			s := t.song()
			artist := ""
			if len(s.Artists) > 0 {
				artist = s.Artists[0]
			}
			out = append(out, RemoteItem{TrackID: t.URI, EntryID: t.URI, Artist: artist, Title: s.Title, Album: s.Album})
		}
		next = resp.Next
	}
	return out, nil
}

func (sp *spotify) create(ctx context.Context, name, description string, trackIDs []string) (string, error) {
	var resp struct {
		ID string `json:"id"`
	}
	body := map[string]any{"name": name, "description": description, "public": false}
	if err := sp.call(ctx, http.MethodPost, "/me/playlists", body, &resp); err != nil {
		return "", err
	}
	return resp.ID, sp.edit(ctx, resp.ID, trackIDs, nil)
}

// edit removes by track URI, which removes every copy of the track: Spotify
// has no entry IDs.
func (sp *spotify) edit(ctx context.Context, playlistID string, add []string, remove []RemoteItem) error {
	path := "/playlists/" + url.PathEscape(playlistID) + "/items"
	for _, batch := range chunks(remove, 100) {
		items := make([]map[string]string, len(batch))
		for i, item := range batch {
			items[i] = map[string]string{"uri": item.TrackID}
		}
		if err := sp.call(ctx, http.MethodDelete, path, map[string]any{"items": items}, nil); err != nil {
			return err
		}
	}
	for _, batch := range chunks(add, 100) {
		if err := sp.call(ctx, http.MethodPost, path, map[string]any{"uris": batch}, nil); err != nil {
			return err
		}
	}
	return nil
}
