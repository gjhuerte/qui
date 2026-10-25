// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package mediasync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/gjhuerte/qui/internal/models"
)

const defaultPlexTV = "https://plex.tv"

// plex talks to a Plex Media Server with the token of its owner. Each other
// account reads and writes its watched state with its own token, which
// plex.tv hands to the owner in the list of shared servers.
type plex struct {
	srv  *models.MediaServer
	http *http.Client
	// plexTV is the plex.tv base URL; tests point it at a fake.
	plexTV string

	// tokens maps a user ID to its token; loaded on first use.
	tokens map[string]string
	people []User
}

func plexHeader(token string) http.Header {
	h := http.Header{}
	h.Set("X-Plex-Token", token)
	h.Set("X-Plex-Client-Identifier", "qui-media-sync")
	h.Set("X-Plex-Product", "qui")
	return h
}

func (p *plex) call(ctx context.Context, token, method, path string, query url.Values, out any) error {
	target := strings.TrimRight(p.srv.BaseURL, "/") + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	return do(ctx, p.http, method, target, plexHeader(token), out, false)
}

// plexInt reads a number Plex sends either as a number or as a string.
type plexInt int

func (n *plexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	*n = plexInt(v)
	return nil
}

type plexMetadata struct {
	RatingKey            json.Number `json:"ratingKey"`
	Type                 string      `json:"type"`
	Title                string      `json:"title"`
	GrandparentTitle     string      `json:"grandparentTitle"`
	GrandparentRatingKey json.Number `json:"grandparentRatingKey"`
	ParentIndex          plexInt     `json:"parentIndex"`
	Index                plexInt     `json:"index"`
	Year                 plexInt     `json:"year"`
	ViewCount            plexInt     `json:"viewCount"`
	Duration             int64       `json:"duration"`
	ViewOffset           int64       `json:"viewOffset"`
	GUID                 []struct {
		ID string `json:"id"`
	} `json:"Guid"`
	User *struct {
		Title string `json:"title"`
	} `json:"User"`
	Player *struct {
		Title   string `json:"title"`
		Product string `json:"product"`
		State   string `json:"state"`
	} `json:"Player"`
	TranscodeSession *struct {
		VideoDecision string `json:"videoDecision"`
	} `json:"TranscodeSession"`
}

type plexContainer struct {
	MediaContainer struct {
		FriendlyName      string         `json:"friendlyName"`
		Version           string         `json:"version"`
		Platform          string         `json:"platform"`
		MachineIdentifier string         `json:"machineIdentifier"`
		TotalSize         int            `json:"totalSize"`
		Size              int            `json:"size"`
		Metadata          []plexMetadata `json:"Metadata"`
		Directory         []struct {
			Key   string `json:"key"`
			Title string `json:"title"`
			Type  string `json:"type"`
		} `json:"Directory"`
	} `json:"MediaContainer"`
}

func (p *plex) status(ctx context.Context) (*ServerStatus, error) {
	var root plexContainer
	if err := p.call(ctx, p.srv.Token, http.MethodGet, "/", nil, &root); err != nil {
		return nil, err
	}
	mc := root.MediaContainer
	st := &ServerStatus{Online: true, ServerName: mc.FriendlyName, Version: mc.Version, Platform: mc.Platform}

	var sections plexContainer
	if err := p.call(ctx, p.srv.Token, http.MethodGet, "/library/sections", nil, &sections); err == nil {
		for _, d := range sections.MediaContainer.Directory {
			lib := Library{Name: d.Title, Type: d.Type}
			if n, err := p.count(ctx, d.Key, ""); err == nil {
				lib.Items = &n
				switch d.Type {
				case "movie":
					st.Movies += n
				case "show":
					st.Series += n
					if eps, err := p.count(ctx, d.Key, "4"); err == nil {
						st.Episodes += eps
					}
				}
			}
			st.Libraries = append(st.Libraries, lib)
		}
	}

	var sessions plexContainer
	if err := p.call(ctx, p.srv.Token, http.MethodGet, "/status/sessions", nil, &sessions); err == nil {
		for _, m := range sessions.MediaContainer.Metadata {
			sess := Session{Title: m.Title, State: "playing"}
			if m.Type == "episode" {
				sess.Title = Item{Kind: kindEpisode, Series: m.GrandparentTitle, Season: int(m.ParentIndex), Episode: int(m.Index), Title: m.Title}.DisplayTitle()
			}
			if m.User != nil {
				sess.User = m.User.Title
			}
			if m.Player != nil {
				sess.Client = strings.TrimSpace(m.Player.Product + " · " + m.Player.Title)
				if m.Player.State == "paused" {
					sess.State = "paused"
				}
			}
			sess.Transcoding = m.TranscodeSession != nil && m.TranscodeSession.VideoDecision == "transcode"
			if m.Duration > 0 {
				sess.Progress = float64(m.ViewOffset) / float64(m.Duration)
			}
			st.Sessions = append(st.Sessions, sess)
		}
	}
	return st, nil
}

// count asks for zero items of a section, which returns only the total.
func (p *plex) count(ctx context.Context, section, itemType string) (int, error) {
	query := url.Values{"X-Plex-Container-Start": {"0"}, "X-Plex-Container-Size": {"0"}}
	if itemType != "" {
		query.Set("type", itemType)
	}
	var resp plexContainer
	if err := p.call(ctx, p.srv.Token, http.MethodGet, "/library/sections/"+url.PathEscape(section)+"/all", query, &resp); err != nil {
		return 0, err
	}
	return resp.MediaContainer.TotalSize, nil
}

type plexSharedServers struct {
	Servers []struct {
		UserID      string `xml:"userID,attr"`
		Username    string `xml:"username,attr"`
		AccessToken string `xml:"accessToken,attr"`
	} `xml:"SharedServer"`
}

// loadUsers fills in the owner and every account the server is shared with.
func (p *plex) loadUsers(ctx context.Context) error {
	if p.tokens != nil {
		return nil
	}
	var owner struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Title    string `json:"title"`
	}
	if err := do(ctx, p.http, http.MethodGet, p.plexTV+"/api/v2/user", plexHeader(p.srv.Token), &owner, false); err != nil {
		return fmt.Errorf("plex.tv: %w", err)
	}
	var identity plexContainer
	if err := p.call(ctx, p.srv.Token, http.MethodGet, "/identity", nil, &identity); err != nil {
		return err
	}
	var shared plexSharedServers
	target := p.plexTV + "/api/servers/" + url.PathEscape(identity.MediaContainer.MachineIdentifier) + "/shared_servers"
	if err := do(ctx, p.http, http.MethodGet, target, plexHeader(p.srv.Token), &shared, true); err != nil {
		return fmt.Errorf("plex.tv: %w", err)
	}

	ownerName := owner.Title
	if ownerName == "" {
		ownerName = owner.Username
	}
	ownerID := strconv.Itoa(owner.ID)
	tokens := map[string]string{ownerID: p.srv.Token}
	people := []User{{ID: ownerID, Name: ownerName, Admin: true}}
	for _, s := range shared.Servers {
		if s.UserID == "" || s.AccessToken == "" || tokens[s.UserID] != "" {
			continue
		}
		tokens[s.UserID] = s.AccessToken
		people = append(people, User{ID: s.UserID, Name: s.Username})
	}
	sort.SliceStable(people[1:], func(a, b int) bool {
		return strings.ToLower(people[a+1].Name) < strings.ToLower(people[b+1].Name)
	})
	p.tokens, p.people = tokens, people
	return nil
}

func (p *plex) users(ctx context.Context) ([]User, error) {
	if err := p.loadUsers(ctx); err != nil {
		return nil, err
	}
	return p.people, nil
}

func (p *plex) token(ctx context.Context, userID string) (string, error) {
	if err := p.loadUsers(ctx); err != nil {
		return "", err
	}
	token := p.tokens[userID]
	if token == "" {
		return "", fmt.Errorf("the Plex account %s no longer has access to this server", userID)
	}
	return token, nil
}

func (p *plex) library(ctx context.Context, userID string) ([]Item, error) {
	token, err := p.token(ctx, userID)
	if err != nil {
		return nil, err
	}
	var sections plexContainer
	if err := p.call(ctx, token, http.MethodGet, "/library/sections", nil, &sections); err != nil {
		return nil, err
	}

	var out []Item
	for _, d := range sections.MediaContainer.Directory {
		section := "/library/sections/" + url.PathEscape(d.Key) + "/all"
		switch d.Type {
		case "movie":
			var movies plexContainer
			if err := p.call(ctx, token, http.MethodGet, section, url.Values{"type": {"1"}, "includeGuids": {"1"}}, &movies); err != nil {
				return nil, err
			}
			for _, m := range movies.MediaContainer.Metadata {
				out = append(out, Item{ID: m.RatingKey.String(), Kind: kindMovie, Title: m.Title, Year: int(m.Year), IDs: plexGUIDs(m), Watched: m.ViewCount > 0})
			}
		case "show":
			var shows, episodes plexContainer
			if err := p.call(ctx, token, http.MethodGet, section, url.Values{"type": {"2"}, "includeGuids": {"1"}}, &shows); err != nil {
				return nil, err
			}
			if err := p.call(ctx, token, http.MethodGet, section, url.Values{"type": {"4"}, "includeGuids": {"1"}}, &episodes); err != nil {
				return nil, err
			}
			showIDs := make(map[string][]string, len(shows.MediaContainer.Metadata))
			for _, s := range shows.MediaContainer.Metadata {
				showIDs[s.RatingKey.String()] = plexGUIDs(s)
			}
			for _, e := range episodes.MediaContainer.Metadata {
				out = append(out, Item{
					ID: e.RatingKey.String(), Kind: kindEpisode, Title: e.Title, IDs: plexGUIDs(e),
					Series: e.GrandparentTitle, SeriesIDs: showIDs[e.GrandparentRatingKey.String()],
					Season: int(e.ParentIndex), Episode: int(e.Index), Watched: e.ViewCount > 0,
				})
			}
		}
	}
	return out, nil
}

// plexGUIDs turns "imdb://tt0133093" into "imdb:tt0133093".
func plexGUIDs(m plexMetadata) []string {
	out := make([]string, 0, len(m.GUID))
	for _, g := range m.GUID {
		provider, value, ok := strings.Cut(g.ID, "://")
		if !ok {
			continue
		}
		if id := providerID(provider, value); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func (p *plex) markWatched(ctx context.Context, userID string, item Item) error {
	token, err := p.token(ctx, userID)
	if err != nil {
		return err
	}
	query := url.Values{"identifier": {"com.plexapp.plugins.library"}, "key": {item.ID}}
	return p.call(ctx, token, http.MethodGet, "/:/scrobble", query, nil)
}

// PlexUserToken returns the token that acts as one account on a Plex server:
// the owner's token for the owner, or the account's own token from plex.tv.
func PlexUserToken(ctx context.Context, srv *models.MediaServer, userID string) (string, error) {
	p := &plex{srv: srv, http: &http.Client{Timeout: requestTimeout}, plexTV: defaultPlexTV}
	return p.token(ctx, userID)
}
