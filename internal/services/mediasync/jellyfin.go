// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package mediasync

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/gjhuerte/qui/internal/models"
)

// jellyfin talks to the Jellyfin API with an API key from Dashboard > API
// Keys. An API key acts as an administrator, so it can read and write the
// played state of every user.
type jellyfin struct {
	srv  *models.MediaServer
	http *http.Client
}

func (j *jellyfin) call(ctx context.Context, method, path string, query url.Values, out any) error {
	target := strings.TrimRight(j.srv.BaseURL, "/") + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	header := http.Header{}
	header.Set("Authorization", `MediaBrowser Token="`+j.srv.Token+`"`)
	return do(ctx, j.http, method, target, header, out, false)
}

func (j *jellyfin) status(ctx context.Context) (*ServerStatus, error) {
	var info struct {
		ServerName      string
		Version         string
		OperatingSystem string
	}
	if err := j.call(ctx, http.MethodGet, "/System/Info", nil, &info); err != nil {
		return nil, err
	}
	st := &ServerStatus{Online: true, ServerName: info.ServerName, Version: info.Version, Platform: info.OperatingSystem}

	var counts struct {
		MovieCount   int
		SeriesCount  int
		EpisodeCount int
	}
	if err := j.call(ctx, http.MethodGet, "/Items/Counts", nil, &counts); err == nil {
		st.Movies, st.Series, st.Episodes = counts.MovieCount, counts.SeriesCount, counts.EpisodeCount
	}

	var folders []struct {
		Name           string
		CollectionType string
	}
	if err := j.call(ctx, http.MethodGet, "/Library/VirtualFolders", nil, &folders); err == nil {
		for _, f := range folders {
			st.Libraries = append(st.Libraries, Library{Name: f.Name, Type: f.CollectionType})
		}
	}

	var users []jellyfinUser
	if err := j.call(ctx, http.MethodGet, "/Users", nil, &users); err == nil {
		st.Users = len(users)
	}

	var sessions []struct {
		UserName       string
		Client         string
		DeviceName     string
		NowPlayingItem *struct {
			Name              string
			Type              string
			SeriesName        string
			ParentIndexNumber int
			IndexNumber       int
			RunTimeTicks      int64
		}
		PlayState struct {
			PositionTicks int64
			IsPaused      bool
			PlayMethod    string
		}
	}
	if err := j.call(ctx, http.MethodGet, "/Sessions", url.Values{"activeWithinSeconds": {"960"}}, &sessions); err == nil {
		for _, s := range sessions {
			np := s.NowPlayingItem
			if np == nil {
				continue
			}
			sess := Session{
				User:        s.UserName,
				Title:       np.Name,
				Client:      strings.TrimSpace(s.Client + " · " + s.DeviceName),
				State:       "playing",
				Transcoding: s.PlayState.PlayMethod == "Transcode",
			}
			if np.Type == "Episode" {
				sess.Title = Item{Kind: kindEpisode, Series: np.SeriesName, Season: np.ParentIndexNumber, Episode: np.IndexNumber, Title: np.Name}.DisplayTitle()
			}
			if s.PlayState.IsPaused {
				sess.State = "paused"
			}
			if np.RunTimeTicks > 0 {
				sess.Progress = float64(s.PlayState.PositionTicks) / float64(np.RunTimeTicks)
			}
			st.Sessions = append(st.Sessions, sess)
		}
	}
	return st, nil
}

type jellyfinUser struct {
	ID     string `json:"Id"`
	Name   string
	Policy struct {
		IsAdministrator bool
		IsDisabled      bool
	}
}

func (j *jellyfin) users(ctx context.Context) ([]User, error) {
	var users []jellyfinUser
	if err := j.call(ctx, http.MethodGet, "/Users", nil, &users); err != nil {
		return nil, err
	}
	out := make([]User, 0, len(users))
	for _, u := range users {
		if u.Policy.IsDisabled {
			continue
		}
		out = append(out, User{ID: u.ID, Name: u.Name, Admin: u.Policy.IsAdministrator})
	}
	sort.Slice(out, func(a, b int) bool { return strings.ToLower(out[a].Name) < strings.ToLower(out[b].Name) })
	return out, nil
}

func (j *jellyfin) library(ctx context.Context, userID string) ([]Item, error) {
	var resp struct {
		Items []struct {
			ID                string `json:"Id"`
			Name              string
			Type              string
			ProductionYear    int
			ProviderIds       map[string]string
			SeriesID          string `json:"SeriesId"`
			SeriesName        string
			ParentIndexNumber int
			IndexNumber       int
			UserData          struct {
				Played bool
			}
		}
	}
	query := url.Values{
		"userId":                 {userID},
		"Recursive":              {"true"},
		"IncludeItemTypes":       {"Movie,Episode,Series"},
		"Fields":                 {"ProviderIds"},
		"EnableImages":           {"false"},
		"EnableTotalRecordCount": {"false"},
	}
	if err := j.call(ctx, http.MethodGet, "/Items", query, &resp); err != nil {
		return nil, err
	}

	seriesIDs := map[string][]string{}
	for _, it := range resp.Items {
		if it.Type == "Series" {
			seriesIDs[it.ID] = jellyfinProviderIDs(it.ProviderIds)
		}
	}

	out := make([]Item, 0, len(resp.Items))
	for _, it := range resp.Items {
		item := Item{ID: it.ID, Title: it.Name, Year: it.ProductionYear, IDs: jellyfinProviderIDs(it.ProviderIds), Watched: it.UserData.Played}
		switch it.Type {
		case "Movie":
			item.Kind = kindMovie
		case "Episode":
			item.Kind = kindEpisode
			item.Series, item.SeriesIDs = it.SeriesName, seriesIDs[it.SeriesID]
			item.Season, item.Episode = it.ParentIndexNumber, it.IndexNumber
		default:
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func jellyfinProviderIDs(ids map[string]string) []string {
	out := make([]string, 0, len(ids))
	for provider, value := range ids {
		if id := providerID(provider, value); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func (j *jellyfin) markWatched(ctx context.Context, userID string, item Item) error {
	return j.call(ctx, http.MethodPost, "/UserPlayedItems/"+url.PathEscape(item.ID), url.Values{"userId": {userID}}, nil)
}
