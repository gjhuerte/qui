// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package mediasync

import (
	"fmt"
	"strings"
)

// Item kinds.
const (
	kindMovie   = "movie"
	kindEpisode = "episode"
)

// Item is a movie or an episode on one server, as one user sees it.
type Item struct {
	// ID is the server's own item ID (Jellyfin item ID, Plex ratingKey).
	ID    string
	Kind  string
	Title string
	Year  int
	// IDs and SeriesIDs are normalised provider IDs such as "imdb:tt0133093".
	IDs       []string
	Series    string
	SeriesIDs []string
	Season    int
	Episode   int
	Watched   bool
}

// DisplayTitle is the title written to the sync log.
func (it Item) DisplayTitle() string {
	if it.Kind == kindEpisode {
		title := fmt.Sprintf("%s S%02dE%02d", it.Series, it.Season, it.Episode)
		if it.Title != "" {
			title += " · " + it.Title
		}
		return title
	}
	if it.Year > 0 {
		return fmt.Sprintf("%s (%d)", it.Title, it.Year)
	}
	return it.Title
}

// providerID normalises one external ID. Only IMDb, TMDB and TVDB IDs are
// shared by Jellyfin and Plex; other providers return "".
func providerID(provider, value string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	switch provider {
	case "imdb", "tmdb", "tvdb":
		return provider + ":" + strings.ToLower(value)
	default:
		return ""
	}
}

// matchKeys are the keys under which an item can be found on the other
// server. An episode matches on its own provider IDs, or on its series
// provider ID plus season and episode number, because many episodes carry
// no IDs of their own.
func matchKeys(it Item) []string {
	keys := make([]string, 0, len(it.IDs)+len(it.SeriesIDs))
	switch it.Kind {
	case kindMovie:
		for _, id := range it.IDs {
			keys = append(keys, "m|"+id)
		}
	case kindEpisode:
		for _, id := range it.IDs {
			keys = append(keys, "e|"+id)
		}
		if it.Episode > 0 {
			for _, id := range it.SeriesIDs {
				keys = append(keys, fmt.Sprintf("s|%s|%d|%d", id, it.Season, it.Episode))
			}
		}
	}
	return keys
}

// mark is one item to mark watched on the target server.
type mark struct {
	Source Item
	Target Item
}

// plan lists the items watched in src that exist unwatched in dst. notFound
// counts watched items that dst does not have, or has without matching IDs.
func plan(src, dst []Item) (marks []mark, notFound int) {
	index := make(map[string]int, len(dst)*2)
	for i, it := range dst {
		for _, key := range matchKeys(it) {
			if _, taken := index[key]; !taken {
				index[key] = i
			}
		}
	}

	planned := make(map[string]bool)
	for _, it := range src {
		if !it.Watched {
			continue
		}
		target, found := -1, false
		for _, key := range matchKeys(it) {
			if i, ok := index[key]; ok {
				target, found = i, true
				break
			}
		}
		if !found {
			notFound++
			continue
		}
		t := dst[target]
		if t.Watched || planned[t.ID] {
			continue
		}
		planned[t.ID] = true
		marks = append(marks, mark{Source: it, Target: t})
	}
	return marks, notFound
}
