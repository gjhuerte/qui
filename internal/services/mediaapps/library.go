// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package mediaapps

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gjhuerte/qui/internal/models"
)

// Library views merge the lists of every enabled instance of one type, so the
// page can show all movies, series or wanted subtitles in one table. Each item
// names its instance and links to its page in the app.

// InstanceRef identifies the instance an item came from.
type InstanceRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// InstanceError is an instance whose list could not be loaded.
type InstanceError struct {
	InstanceRef
	Error string `json:"error"`
}

// Library is the merged list plus the instances it was built from.
type Library[T any] struct {
	Items     []T             `json:"items"`
	Instances []InstanceRef   `json:"instances"`
	Errors    []InstanceError `json:"errors"`
	LoadedAt  time.Time       `json:"loadedAt"`
}

type Movie struct {
	Instance    InstanceRef `json:"instance"`
	ID          int         `json:"id"`
	Title       string      `json:"title"`
	Year        int         `json:"year,omitempty"`
	Status      string      `json:"status,omitempty"` // announced | inCinemas | released | tba | deleted
	Monitored   bool        `json:"monitored"`
	HasFile     bool        `json:"hasFile"`
	IsAvailable bool        `json:"isAvailable"`
	Quality     string      `json:"quality,omitempty"`
	SizeOnDisk  int64       `json:"sizeOnDisk"`
	Added       string      `json:"added,omitempty"`
	Release     string      `json:"release,omitempty"` // digital, physical or cinema release date
	ImdbID      string      `json:"imdbId,omitempty"`
	Link        string      `json:"link"`
}

type Series struct {
	Instance      InstanceRef `json:"instance"`
	ID            int         `json:"id"`
	Title         string      `json:"title"`
	Year          int         `json:"year,omitempty"`
	Status        string      `json:"status,omitempty"` // continuing | ended | upcoming | deleted
	Monitored     bool        `json:"monitored"`
	Network       string      `json:"network,omitempty"`
	Seasons       int         `json:"seasons"`
	EpisodeFiles  int         `json:"episodeFiles"`
	Episodes      int         `json:"episodes"` // monitored, aired episodes
	TotalEpisodes int         `json:"totalEpisodes"`
	Missing       int         `json:"missing"`
	SizeOnDisk    int64       `json:"sizeOnDisk"`
	NextAiring    string      `json:"nextAiring,omitempty"`
	Link          string      `json:"link"`
}

type WantedSubtitle struct {
	Instance  InstanceRef `json:"instance"`
	Kind      string      `json:"kind"` // episode | movie
	Title     string      `json:"title"`
	Episode   string      `json:"episode,omitempty"` // 1x02 · Episode title
	Languages []string    `json:"languages"`
	Link      string      `json:"link"`
}

const (
	libraryCacheTTL = time.Minute
	// Bazarr pages its wanted lists; this covers any realistic backlog.
	wantedLimit = 5000
)

type libraryCache struct {
	mu      sync.Mutex
	entries map[string]libraryEntry
}

type libraryEntry struct {
	value any
	at    time.Time
}

func (c *libraryCache) get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Since(e.at) > libraryCacheTTL {
		return nil, false
	}
	return e.value, true
}

func (c *libraryCache) put(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]libraryEntry{}
	}
	c.entries[key] = libraryEntry{value: value, at: time.Now()}
}

func (c *libraryCache) clear() {
	c.mu.Lock()
	c.entries = nil
	c.mu.Unlock()
}

// loadLibrary fetches one list per enabled instance of appType in parallel and
// merges them. A failing instance becomes an entry in Errors.
func loadLibrary[T any](ctx context.Context, s *Service, appType models.AppType, refresh bool,
	fetchOne func(ctx context.Context, c *client, ref InstanceRef) ([]T, error), sortKey func(T) string,
) (*Library[T], error) {
	key := string(appType)
	if !refresh {
		if cached, ok := s.library.get(key); ok {
			return cached.(*Library[T]), nil
		}
	}

	instances, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*requestTimeout)
	defer cancel()

	lib := &Library[T]{Items: []T{}, Instances: []InstanceRef{}, Errors: []InstanceError{}}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, inst := range instances {
		if !inst.Enabled || inst.Type != appType {
			continue
		}
		ref := InstanceRef{ID: inst.ID, Name: inst.Name}
		lib.Instances = append(lib.Instances, ref)
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Full libraries can take a while on a slow server.
			c := &client{inst: inst, http: s.libraryHTTP}
			items, err := fetchOne(ctx, c, ref)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				lib.Errors = append(lib.Errors, InstanceError{InstanceRef: ref, Error: err.Error()})
				return
			}
			lib.Items = append(lib.Items, items...)
		}()
	}
	wg.Wait()

	sort.SliceStable(lib.Items, func(i, j int) bool {
		return strings.ToLower(sortKey(lib.Items[i])) < strings.ToLower(sortKey(lib.Items[j]))
	})
	lib.LoadedAt = time.Now()
	s.library.put(key, lib)
	return lib, nil
}

func appLink(base string, parts ...string) string {
	return strings.TrimRight(base, "/") + "/" + strings.Join(parts, "/")
}

// Movies lists every movie of every enabled Radarr instance.
func (s *Service) Movies(ctx context.Context, refresh bool) (*Library[Movie], error) {
	return loadLibrary(ctx, s, models.AppTypeRadarr, refresh, fetchMovies, func(m Movie) string { return m.Title })
}

func fetchMovies(ctx context.Context, c *client, ref InstanceRef) ([]Movie, error) {
	var movies []struct {
		ID              int    `json:"id"`
		Title           string `json:"title"`
		Year            int    `json:"year"`
		TitleSlug       string `json:"titleSlug"`
		TmdbID          int    `json:"tmdbId"`
		ImdbID          string `json:"imdbId"`
		Status          string `json:"status"`
		Monitored       bool   `json:"monitored"`
		HasFile         bool   `json:"hasFile"`
		IsAvailable     bool   `json:"isAvailable"`
		SizeOnDisk      int64  `json:"sizeOnDisk"`
		Added           string `json:"added"`
		InCinemas       string `json:"inCinemas"`
		DigitalRelease  string `json:"digitalRelease"`
		PhysicalRelease string `json:"physicalRelease"`
		MovieFile       *struct {
			Quality struct {
				Quality struct {
					Name string `json:"name"`
				} `json:"quality"`
			} `json:"quality"`
		} `json:"movieFile"`
	}
	if err := c.get(ctx, "/api/v3/movie", nil, &movies); err != nil {
		return nil, err
	}

	out := make([]Movie, 0, len(movies))
	for _, m := range movies {
		slug := m.TitleSlug
		if slug == "" {
			slug = strconv.Itoa(m.TmdbID)
		}
		movie := Movie{
			Instance:    ref,
			ID:          m.ID,
			Title:       m.Title,
			Year:        m.Year,
			Status:      m.Status,
			Monitored:   m.Monitored,
			HasFile:     m.HasFile,
			IsAvailable: m.IsAvailable,
			SizeOnDisk:  m.SizeOnDisk,
			Added:       m.Added,
			Release:     firstNonEmpty(m.DigitalRelease, m.PhysicalRelease, m.InCinemas),
			ImdbID:      m.ImdbID,
			Link:        appLink(c.inst.BaseURL, "movie", url.PathEscape(slug)),
		}
		if m.MovieFile != nil {
			movie.Quality = m.MovieFile.Quality.Quality.Name
		}
		out = append(out, movie)
	}
	return out, nil
}

// Series lists every series of every enabled Sonarr instance.
func (s *Service) Series(ctx context.Context, refresh bool) (*Library[Series], error) {
	return loadLibrary(ctx, s, models.AppTypeSonarr, refresh, fetchSeries, func(se Series) string { return se.Title })
}

func fetchSeries(ctx context.Context, c *client, ref InstanceRef) ([]Series, error) {
	var series []struct {
		ID         int    `json:"id"`
		Title      string `json:"title"`
		Year       int    `json:"year"`
		TitleSlug  string `json:"titleSlug"`
		Status     string `json:"status"`
		Monitored  bool   `json:"monitored"`
		Network    string `json:"network"`
		NextAiring string `json:"nextAiring"`
		Statistics struct {
			SeasonCount       int   `json:"seasonCount"`
			EpisodeFileCount  int   `json:"episodeFileCount"`
			EpisodeCount      int   `json:"episodeCount"`
			TotalEpisodeCount int   `json:"totalEpisodeCount"`
			SizeOnDisk        int64 `json:"sizeOnDisk"`
		} `json:"statistics"`
	}
	if err := c.get(ctx, "/api/v3/series", nil, &series); err != nil {
		return nil, err
	}

	out := make([]Series, 0, len(series))
	for _, se := range series {
		st := se.Statistics
		out = append(out, Series{
			Instance:      ref,
			ID:            se.ID,
			Title:         se.Title,
			Year:          se.Year,
			Status:        se.Status,
			Monitored:     se.Monitored,
			Network:       se.Network,
			Seasons:       st.SeasonCount,
			EpisodeFiles:  st.EpisodeFileCount,
			Episodes:      st.EpisodeCount,
			TotalEpisodes: st.TotalEpisodeCount,
			Missing:       max(0, st.EpisodeCount-st.EpisodeFileCount),
			SizeOnDisk:    st.SizeOnDisk,
			NextAiring:    se.NextAiring,
			Link:          appLink(c.inst.BaseURL, "series", url.PathEscape(se.TitleSlug)),
		})
	}
	return out, nil
}

// WantedSubtitles lists the episodes and movies that miss subtitles in every
// enabled Bazarr instance.
func (s *Service) WantedSubtitles(ctx context.Context, refresh bool) (*Library[WantedSubtitle], error) {
	return loadLibrary(ctx, s, models.AppTypeBazarr, refresh, fetchWanted, func(w WantedSubtitle) string { return w.Title + " " + w.Episode })
}

type bazarrLanguage struct {
	Name   string `json:"name"`
	Code2  string `json:"code2"`
	Forced bool   `json:"forced"`
	HI     bool   `json:"hi"`
}

func languageNames(langs []bazarrLanguage) []string {
	out := make([]string, 0, len(langs))
	for _, l := range langs {
		name := firstNonEmpty(l.Name, l.Code2)
		switch {
		case l.Forced:
			name += " (forced)"
		case l.HI:
			name += " (HI)"
		}
		out = append(out, name)
	}
	return out
}

func fetchWanted(ctx context.Context, c *client, ref InstanceRef) ([]WantedSubtitle, error) {
	page := url.Values{"start": {"0"}, "length": {strconv.Itoa(wantedLimit)}}

	var episodes struct {
		Data []struct {
			SeriesTitle      string           `json:"seriesTitle"`
			EpisodeNumber    string           `json:"episode_number"`
			EpisodeTitle     string           `json:"episodeTitle"`
			SonarrSeriesID   int              `json:"sonarrSeriesId"`
			MissingSubtitles []bazarrLanguage `json:"missing_subtitles"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/api/episodes/wanted", page, &episodes); err != nil {
		return nil, err
	}
	var movies struct {
		Data []struct {
			Title            string           `json:"title"`
			RadarrID         int              `json:"radarrId"`
			MissingSubtitles []bazarrLanguage `json:"missing_subtitles"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/api/movies/wanted", page, &movies); err != nil {
		return nil, err
	}

	out := make([]WantedSubtitle, 0, len(episodes.Data)+len(movies.Data))
	for _, e := range episodes.Data {
		episode := e.EpisodeNumber
		if e.EpisodeTitle != "" {
			episode = strings.TrimSpace(fmt.Sprintf("%s · %s", e.EpisodeNumber, e.EpisodeTitle))
		}
		out = append(out, WantedSubtitle{
			Instance:  ref,
			Kind:      "episode",
			Title:     e.SeriesTitle,
			Episode:   episode,
			Languages: languageNames(e.MissingSubtitles),
			Link:      appLink(c.inst.BaseURL, "series", strconv.Itoa(e.SonarrSeriesID)),
		})
	}
	for _, m := range movies.Data {
		out = append(out, WantedSubtitle{
			Instance:  ref,
			Kind:      "movie",
			Title:     m.Title,
			Languages: languageNames(m.MissingSubtitles),
			Link:      appLink(c.inst.BaseURL, "movies", strconv.Itoa(m.RadarrID)),
		})
	}
	return out, nil
}
