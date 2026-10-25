// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/gjhuerte/qui/internal/models"
)

// Jellyfin and Plex hold the music themselves, so a track matches when the
// account's library has it: qui loads the whole music library once and looks
// each track up locally.

// jellyfin acts as one user with the server's API key.
type jellyfin struct {
	srv    *models.MediaServer
	userID string
	http   *http.Client
	songs  []Song
}

func (j *jellyfin) call(ctx context.Context, method, path string, query url.Values, body, out any) error {
	target := strings.TrimRight(j.srv.BaseURL, "/") + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	header := http.Header{}
	header.Set("Authorization", `MediaBrowser Token="`+j.srv.Token+`"`)
	return send(ctx, j.http, request{method: method, url: target, header: header, body: body}, out)
}

type jellyfinAudio struct {
	ID             string `json:"Id"`
	PlaylistItemID string `json:"PlaylistItemId"`
	Name           string
	Album          string
	AlbumArtist    string
	Artists        []string
	MediaType      string
	ChildCount     int
}

func (a jellyfinAudio) song() Song {
	artists := append([]string{}, a.Artists...)
	if a.AlbumArtist != "" {
		artists = append(artists, a.AlbumArtist)
	}
	return Song{ID: a.ID, Title: a.Name, Artists: artists, Album: a.Album}
}

func (j *jellyfin) library(ctx context.Context) ([]Song, error) {
	if j.songs != nil {
		return j.songs, nil
	}
	var resp struct{ Items []jellyfinAudio }
	query := url.Values{
		"userId":                 {j.userID},
		"Recursive":              {"true"},
		"IncludeItemTypes":       {"Audio"},
		"EnableImages":           {"false"},
		"EnableUserData":         {"false"},
		"EnableTotalRecordCount": {"false"},
	}
	if err := j.call(ctx, http.MethodGet, "/Items", query, nil, &resp); err != nil {
		return nil, err
	}
	j.songs = make([]Song, 0, len(resp.Items))
	for _, a := range resp.Items {
		j.songs = append(j.songs, a.song())
	}
	return j.songs, nil
}

func (j *jellyfin) match(ctx context.Context, tracks []models.PlaylistTrack) ([]string, error) {
	songs, err := j.library(ctx)
	if err != nil {
		return nil, err
	}
	idx := newLibraryIndex(songs)
	ids := make([]string, len(tracks))
	for i, t := range tracks {
		ids[i] = idx.find(t)
	}
	return ids, nil
}

func (j *jellyfin) playlists(ctx context.Context) ([]RemotePlaylist, error) {
	var resp struct{ Items []jellyfinAudio }
	query := url.Values{
		"userId":           {j.userID},
		"Recursive":        {"true"},
		"IncludeItemTypes": {"Playlist"},
		"Fields":           {"ChildCount"},
		"EnableImages":     {"false"},
	}
	if err := j.call(ctx, http.MethodGet, "/Items", query, nil, &resp); err != nil {
		return nil, err
	}
	out := []RemotePlaylist{}
	for _, p := range resp.Items {
		if p.MediaType != "" && p.MediaType != "Audio" {
			continue
		}
		out = append(out, RemotePlaylist{ID: p.ID, Name: p.Name, Count: p.ChildCount})
	}
	return out, nil
}

func (j *jellyfin) items(ctx context.Context, playlistID string) ([]RemoteItem, error) {
	var resp struct{ Items []jellyfinAudio }
	query := url.Values{"userId": {j.userID}, "EnableImages": {"false"}, "EnableUserData": {"false"}}
	if err := j.call(ctx, http.MethodGet, "/Playlists/"+url.PathEscape(playlistID)+"/Items", query, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]RemoteItem, 0, len(resp.Items))
	for _, a := range resp.Items {
		artist := ""
		if len(a.Artists) > 0 {
			artist = a.Artists[0]
		}
		out = append(out, RemoteItem{TrackID: a.ID, EntryID: a.PlaylistItemID, Artist: artist, Title: a.Name, Album: a.Album})
	}
	return out, nil
}

func (j *jellyfin) create(ctx context.Context, name, _ string, trackIDs []string) (string, error) {
	var resp struct {
		ID string `json:"Id"`
	}
	body := map[string]any{"Name": name, "Ids": trackIDs, "UserId": j.userID, "MediaType": "Audio", "IsPublic": false}
	if err := j.call(ctx, http.MethodPost, "/Playlists", nil, body, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (j *jellyfin) edit(ctx context.Context, playlistID string, add []string, remove []RemoteItem) error {
	path := "/Playlists/" + url.PathEscape(playlistID) + "/Items"
	for _, batch := range chunks(remove, 100) {
		entries := make([]string, len(batch))
		for i, item := range batch {
			entries[i] = item.EntryID
		}
		if err := j.call(ctx, http.MethodDelete, path, url.Values{"entryIds": {strings.Join(entries, ",")}}, nil, nil); err != nil {
			return err
		}
	}
	for _, batch := range chunks(add, 100) {
		if err := j.call(ctx, http.MethodPost, path, url.Values{"ids": {strings.Join(batch, ",")}, "userId": {j.userID}}, nil, nil); err != nil {
			return err
		}
	}
	return nil
}

// plex acts as one account with that account's token.
type plex struct {
	srv       *models.MediaServer
	token     string
	http      *http.Client
	songs     []Song
	machineID string
}

func (p *plex) call(ctx context.Context, method, path string, query url.Values, out any) error {
	target := strings.TrimRight(p.srv.BaseURL, "/") + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	header := http.Header{}
	header.Set("X-Plex-Token", p.token)
	header.Set("X-Plex-Client-Identifier", "qui-playlists")
	header.Set("X-Plex-Product", "qui")
	return send(ctx, p.http, request{method: method, url: target, header: header}, out)
}

type plexTrack struct {
	RatingKey        json.Number `json:"ratingKey"`
	PlaylistItemID   json.Number `json:"playlistItemID"`
	Title            string      `json:"title"`
	GrandparentTitle string      `json:"grandparentTitle"`
	OriginalTitle    string      `json:"originalTitle"`
	ParentTitle      string      `json:"parentTitle"`
	LeafCount        int         `json:"leafCount"`
	Smart            any         `json:"smart"`
}

// artist is the track artist: originalTitle when it differs from the album
// artist, as on compilations.
func (t plexTrack) artist() string {
	if t.OriginalTitle != "" {
		return t.OriginalTitle
	}
	return t.GrandparentTitle
}

type plexMedia struct {
	MediaContainer struct {
		MachineIdentifier string      `json:"machineIdentifier"`
		Metadata          []plexTrack `json:"Metadata"`
		Directory         []struct {
			Key  string `json:"key"`
			Type string `json:"type"`
		} `json:"Directory"`
	} `json:"MediaContainer"`
}

func (p *plex) library(ctx context.Context) ([]Song, error) {
	if p.songs != nil {
		return p.songs, nil
	}
	var sections plexMedia
	if err := p.call(ctx, http.MethodGet, "/library/sections", nil, &sections); err != nil {
		return nil, err
	}
	p.songs = []Song{}
	for _, d := range sections.MediaContainer.Directory {
		if d.Type != "artist" {
			continue
		}
		var tracks plexMedia
		if err := p.call(ctx, http.MethodGet, "/library/sections/"+url.PathEscape(d.Key)+"/all", url.Values{"type": {"10"}}, &tracks); err != nil {
			return nil, err
		}
		for _, t := range tracks.MediaContainer.Metadata {
			artists := []string{t.artist()}
			if t.OriginalTitle != "" && t.GrandparentTitle != "" {
				artists = append(artists, t.GrandparentTitle)
			}
			p.songs = append(p.songs, Song{ID: t.RatingKey.String(), Title: t.Title, Artists: artists, Album: t.ParentTitle})
		}
	}
	return p.songs, nil
}

func (p *plex) match(ctx context.Context, tracks []models.PlaylistTrack) ([]string, error) {
	songs, err := p.library(ctx)
	if err != nil {
		return nil, err
	}
	idx := newLibraryIndex(songs)
	ids := make([]string, len(tracks))
	for i, t := range tracks {
		ids[i] = idx.find(t)
	}
	return ids, nil
}

func (p *plex) playlists(ctx context.Context) ([]RemotePlaylist, error) {
	var resp plexMedia
	if err := p.call(ctx, http.MethodGet, "/playlists", url.Values{"playlistType": {"audio"}}, &resp); err != nil {
		return nil, err
	}
	out := []RemotePlaylist{}
	for _, m := range resp.MediaContainer.Metadata {
		// Smart playlists follow a filter; their items cannot be edited.
		if smart, _ := m.Smart.(bool); smart || m.Smart == float64(1) || m.Smart == "1" {
			continue
		}
		out = append(out, RemotePlaylist{ID: m.RatingKey.String(), Name: m.Title, Count: m.LeafCount})
	}
	return out, nil
}

func (p *plex) items(ctx context.Context, playlistID string) ([]RemoteItem, error) {
	var resp plexMedia
	if err := p.call(ctx, http.MethodGet, "/playlists/"+url.PathEscape(playlistID)+"/items", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]RemoteItem, 0, len(resp.MediaContainer.Metadata))
	for _, t := range resp.MediaContainer.Metadata {
		out = append(out, RemoteItem{TrackID: t.RatingKey.String(), EntryID: t.PlaylistItemID.String(), Artist: t.artist(), Title: t.Title, Album: t.ParentTitle})
	}
	return out, nil
}

// uri names library items for the playlist endpoints.
func (p *plex) uri(ctx context.Context, ids []string) (string, error) {
	if p.machineID == "" {
		var identity plexMedia
		if err := p.call(ctx, http.MethodGet, "/identity", nil, &identity); err != nil {
			return "", err
		}
		p.machineID = identity.MediaContainer.MachineIdentifier
	}
	return "server://" + p.machineID + "/com.plexapp.plugins.library/library/metadata/" + strings.Join(ids, ","), nil
}

// create makes the playlist with the first batch of tracks (Plex refuses an
// empty one) and adds the rest.
func (p *plex) create(ctx context.Context, name, _ string, trackIDs []string) (string, error) {
	batches := chunks(trackIDs, 100)
	uri, err := p.uri(ctx, batches[0])
	if err != nil {
		return "", err
	}
	var resp plexMedia
	query := url.Values{"type": {"audio"}, "title": {name}, "smart": {"0"}, "uri": {uri}}
	if err := p.call(ctx, http.MethodPost, "/playlists", query, &resp); err != nil {
		return "", err
	}
	if len(resp.MediaContainer.Metadata) == 0 {
		return "", &httpError{Status: http.StatusBadGateway, Body: "Plex did not return the new playlist"}
	}
	id := resp.MediaContainer.Metadata[0].RatingKey.String()
	var rest []string
	for _, b := range batches[1:] {
		rest = append(rest, b...)
	}
	return id, p.edit(ctx, id, rest, nil)
}

func (p *plex) edit(ctx context.Context, playlistID string, add []string, remove []RemoteItem) error {
	path := "/playlists/" + url.PathEscape(playlistID) + "/items"
	for _, item := range remove {
		if err := p.call(ctx, http.MethodDelete, path+"/"+url.PathEscape(item.EntryID), nil, nil); err != nil {
			return err
		}
	}
	for _, batch := range chunks(add, 100) {
		uri, err := p.uri(ctx, batch)
		if err != nil {
			return err
		}
		if err := p.call(ctx, http.MethodPut, path, url.Values{"uri": {uri}}, nil); err != nil {
			return err
		}
	}
	return nil
}
