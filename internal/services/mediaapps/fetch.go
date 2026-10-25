// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package mediaapps

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gjhuerte/qui/internal/models"
)

// Status is what the dashboard shows for one instance. Fields that do not
// apply to the instance's type stay empty.
type Status struct {
	Instance  *models.AppInstance `json:"instance"`
	Online    bool                `json:"online"`
	Error     string              `json:"error,omitempty"`
	Version   string              `json:"version,omitempty"`
	CheckedAt time.Time           `json:"checkedAt"`

	Health   []HealthItem `json:"health"`
	Queue    *Queue       `json:"queue,omitempty"`
	// QueueIssues are downloads Sonarr or Radarr flag with a warning or error.
	QueueIssues []QueueItem `json:"queueIssues,omitempty"`
	Missing  *int         `json:"missing,omitempty"`
	Disks    []Disk       `json:"disks,omitempty"`
	Upcoming []Upcoming   `json:"upcoming,omitempty"`

	Indexers  *Indexers  `json:"indexers,omitempty"`
	Subtitles *Subtitles `json:"subtitles,omitempty"`
	Streams   *Streams   `json:"streams,omitempty"`

	// Warnings are parts of the status that could not be loaded.
	Warnings []string `json:"warnings,omitempty"`
}

type HealthItem struct {
	Level   string `json:"level"` // ok | notice | warning | error
	Source  string `json:"source,omitempty"`
	Message string `json:"message"`
}

type Queue struct {
	Total int         `json:"total"`
	Items []QueueItem `json:"items"`
}

type QueueItem struct {
	Title    string  `json:"title"`
	Status   string  `json:"status,omitempty"`
	Progress float64 `json:"progress"` // 0..1
	TimeLeft string  `json:"timeLeft,omitempty"`
	Client   string  `json:"client,omitempty"`
	Warning  bool    `json:"warning,omitempty"`
	// Messages explain a warning, for example "No files found are eligible for import".
	Messages []string `json:"messages,omitempty"`
}

type Disk struct {
	Path  string `json:"path"`
	Free  int64  `json:"free"`
	Total int64  `json:"total"`
}

type Upcoming struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	Date     string `json:"date"`
	HasFile  bool   `json:"hasFile"`
}

type Indexers struct {
	Total   int      `json:"total"`
	Enabled int      `json:"enabled"`
	Failing []string `json:"failing"`
	Queries int      `json:"queries"`
	Grabs   int      `json:"grabs"`
}

type Subtitles struct {
	WantedEpisodes int `json:"wantedEpisodes"`
	WantedMovies   int `json:"wantedMovies"`
}

type Streams struct {
	Count     int       `json:"count"`
	Bandwidth int       `json:"bandwidth"` // kbps
	Sessions  []Session `json:"sessions"`
}

type Session struct {
	User     string  `json:"user"`
	Title    string  `json:"title"`
	State    string  `json:"state"`
	Progress float64 `json:"progress"` // 0..1
	Player   string  `json:"player,omitempty"`
	Decision string  `json:"decision,omitempty"` // direct play | copy | transcode
	Quality  string  `json:"quality,omitempty"`
}

const (
	queueItemsShown    = 10
	queueItemsFetched  = 200
	upcomingDays       = 7
	upcomingItemsShown = 10
)

// fetch loads the status of one instance. The version call decides whether the
// app is online; the other calls only add warnings when they fail.
func fetch(ctx context.Context, c *client) *Status {
	st := &Status{Instance: c.inst, Health: []HealthItem{}, CheckedAt: time.Now()}

	var err error
	switch c.inst.Type {
	case models.AppTypeSonarr, models.AppTypeRadarr:
		err = fetchArr(ctx, c, st)
	case models.AppTypeProwlarr:
		err = fetchProwlarr(ctx, c, st)
	case models.AppTypeBazarr:
		err = fetchBazarr(ctx, c, st)
	case models.AppTypeTautulli:
		err = fetchTautulli(ctx, c, st)
	default:
		err = fmt.Errorf("unknown app type %q", c.inst.Type)
	}
	if err != nil {
		st.Online = false
		st.Error = err.Error()
		if errors.Is(err, errNotFound) {
			st.Error = "not found: check the URL, including any URL base such as /sonarr"
		}
	}
	return st
}

// parallel runs the optional parts of a status concurrently and records each
// failure as a warning.
type parallel struct {
	wg sync.WaitGroup
	mu sync.Mutex
	st *Status
}

func (p *parallel) run(what string, fn func() error) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		if err := fn(); err != nil {
			p.mu.Lock()
			p.st.Warnings = append(p.st.Warnings, what+": "+err.Error())
			p.mu.Unlock()
		}
	}()
}

func (p *parallel) wait() {
	p.wg.Wait()
	sort.Strings(p.st.Warnings)
}

// --- Sonarr and Radarr (API v3) ---

type arrHealth struct {
	Source  string `json:"source"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

func fetchHealth(ctx context.Context, c *client, path string, st *Status, mu *sync.Mutex) error {
	var health []arrHealth
	if err := c.get(ctx, path, nil, &health); err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	for _, h := range health {
		st.Health = append(st.Health, HealthItem{Level: strings.ToLower(h.Type), Source: h.Source, Message: h.Message})
	}
	return nil
}

func fetchArr(ctx context.Context, c *client, st *Status) error {
	var status struct {
		Version string `json:"version"`
	}
	if err := c.get(ctx, "/api/v3/system/status", nil, &status); err != nil {
		return err
	}
	st.Online, st.Version = true, status.Version

	p := &parallel{st: st}
	p.run("health", func() error { return fetchHealth(ctx, c, "/api/v3/health", st, &p.mu) })
	p.run("queue", func() error { return fetchArrQueue(ctx, c, st, &p.mu) })
	p.run("missing", func() error { return fetchArrMissing(ctx, c, st, &p.mu) })
	p.run("disk space", func() error { return fetchArrDisks(ctx, c, st, &p.mu) })
	p.run("calendar", func() error { return fetchArrCalendar(ctx, c, st, &p.mu) })
	p.wait()
	return nil
}

func fetchArrQueue(ctx context.Context, c *client, st *Status, mu *sync.Mutex) error {
	var page struct {
		TotalRecords int `json:"totalRecords"`
		Records      []struct {
			Title                 string  `json:"title"`
			Status                string  `json:"status"`
			TrackedDownloadStatus string  `json:"trackedDownloadStatus"`
			TrackedDownloadState  string  `json:"trackedDownloadState"`
			Size                  float64 `json:"size"`
			SizeLeft              float64 `json:"sizeleft"`
			TimeLeft              string  `json:"timeleft"`
			DownloadClient        string  `json:"downloadClient"`
			ErrorMessage          string  `json:"errorMessage"`
			StatusMessages        []struct {
				Title    string   `json:"title"`
				Messages []string `json:"messages"`
			} `json:"statusMessages"`
		} `json:"records"`
	}
	// One page large enough to find every flagged download; the card shows the first few.
	query := url.Values{"page": {"1"}, "pageSize": {strconv.Itoa(queueItemsFetched)}, "sortKey": {"timeleft"}, "sortDirection": {"ascending"}}
	if err := c.get(ctx, "/api/v3/queue", query, &page); err != nil {
		return err
	}

	q := &Queue{Total: page.TotalRecords, Items: []QueueItem{}}
	var issues []QueueItem
	for _, r := range page.Records {
		item := QueueItem{
			Title:    r.Title,
			Status:   strings.ToLower(firstNonEmpty(r.TrackedDownloadState, r.Status)),
			TimeLeft: r.TimeLeft,
			Client:   r.DownloadClient,
			Warning:  strings.EqualFold(r.TrackedDownloadStatus, "warning") || strings.EqualFold(r.TrackedDownloadStatus, "error"),
		}
		if r.Size > 0 {
			item.Progress = clamp01(1 - r.SizeLeft/r.Size)
		}
		if item.Warning {
			flagged := item
			if r.ErrorMessage != "" {
				flagged.Messages = append(flagged.Messages, r.ErrorMessage)
			}
			for _, m := range r.StatusMessages {
				flagged.Messages = append(flagged.Messages, m.Messages...)
			}
			issues = append(issues, flagged)
		}
		if len(q.Items) < queueItemsShown {
			q.Items = append(q.Items, item)
		}
	}
	mu.Lock()
	st.Queue = q
	st.QueueIssues = issues
	mu.Unlock()
	return nil
}

// fetchArrMissing counts monitored items without a file. Sonarr and newer
// Radarr serve /wanted/missing; older Radarr needs the whole movie list.
func fetchArrMissing(ctx context.Context, c *client, st *Status, mu *sync.Mutex) error {
	var page struct {
		TotalRecords int `json:"totalRecords"`
	}
	err := c.get(ctx, "/api/v3/wanted/missing", url.Values{"page": {"1"}, "pageSize": {"1"}, "monitored": {"true"}}, &page)
	count := page.TotalRecords
	if errors.Is(err, errNotFound) && c.inst.Type == models.AppTypeRadarr {
		var movies []struct {
			Monitored   bool `json:"monitored"`
			HasFile     bool `json:"hasFile"`
			IsAvailable bool `json:"isAvailable"`
		}
		if err = c.get(ctx, "/api/v3/movie", nil, &movies); err == nil {
			count = 0
			for _, m := range movies {
				if m.Monitored && !m.HasFile && m.IsAvailable {
					count++
				}
			}
		}
	}
	if err != nil {
		return err
	}
	mu.Lock()
	st.Missing = &count
	mu.Unlock()
	return nil
}

func fetchArrDisks(ctx context.Context, c *client, st *Status, mu *sync.Mutex) error {
	var disks []struct {
		Path       string `json:"path"`
		FreeSpace  int64  `json:"freeSpace"`
		TotalSpace int64  `json:"totalSpace"`
	}
	if err := c.get(ctx, "/api/v3/diskspace", nil, &disks); err != nil {
		return err
	}
	out := []Disk{}
	for _, d := range disks {
		if d.TotalSpace > 0 {
			out = append(out, Disk{Path: d.Path, Free: d.FreeSpace, Total: d.TotalSpace})
		}
	}
	mu.Lock()
	st.Disks = out
	mu.Unlock()
	return nil
}

func fetchArrCalendar(ctx context.Context, c *client, st *Status, mu *sync.Mutex) error {
	now := time.Now().UTC()
	query := url.Values{
		"start":         {now.Format(time.RFC3339)},
		"end":           {now.AddDate(0, 0, upcomingDays).Format(time.RFC3339)},
		"includeSeries": {"true"},
	}

	var upcoming []Upcoming
	if c.inst.Type == models.AppTypeSonarr {
		var episodes []struct {
			Series struct {
				Title string `json:"title"`
			} `json:"series"`
			SeasonNumber  int    `json:"seasonNumber"`
			EpisodeNumber int    `json:"episodeNumber"`
			Title         string `json:"title"`
			AirDateUTC    string `json:"airDateUtc"`
			HasFile       bool   `json:"hasFile"`
		}
		if err := c.get(ctx, "/api/v3/calendar", query, &episodes); err != nil {
			return err
		}
		for _, e := range episodes {
			upcoming = append(upcoming, Upcoming{
				Title:    e.Series.Title,
				Subtitle: fmt.Sprintf("S%02dE%02d · %s", e.SeasonNumber, e.EpisodeNumber, e.Title),
				Date:     e.AirDateUTC,
				HasFile:  e.HasFile,
			})
		}
	} else {
		var movies []struct {
			Title           string `json:"title"`
			Year            int    `json:"year"`
			InCinemas       string `json:"inCinemas"`
			DigitalRelease  string `json:"digitalRelease"`
			PhysicalRelease string `json:"physicalRelease"`
			HasFile         bool   `json:"hasFile"`
		}
		if err := c.get(ctx, "/api/v3/calendar", query, &movies); err != nil {
			return err
		}
		for _, m := range movies {
			// The release that falls inside the window.
			date := ""
			for _, d := range []string{m.DigitalRelease, m.PhysicalRelease, m.InCinemas} {
				if t, err := time.Parse(time.RFC3339, d); err == nil && !t.Before(now.AddDate(0, 0, -1)) {
					if date == "" || d < date {
						date = d
					}
				}
			}
			if date == "" {
				date = firstNonEmpty(m.DigitalRelease, m.PhysicalRelease, m.InCinemas)
			}
			title := m.Title
			if m.Year > 0 {
				title = fmt.Sprintf("%s (%d)", m.Title, m.Year)
			}
			upcoming = append(upcoming, Upcoming{Title: title, Date: date, HasFile: m.HasFile})
		}
	}

	sort.SliceStable(upcoming, func(i, j int) bool { return upcoming[i].Date < upcoming[j].Date })
	if len(upcoming) > upcomingItemsShown {
		upcoming = upcoming[:upcomingItemsShown]
	}
	if upcoming == nil {
		upcoming = []Upcoming{}
	}
	mu.Lock()
	st.Upcoming = upcoming
	mu.Unlock()
	return nil
}

// --- Prowlarr (API v1) ---

func fetchProwlarr(ctx context.Context, c *client, st *Status) error {
	var status struct {
		Version string `json:"version"`
	}
	if err := c.get(ctx, "/api/v1/system/status", nil, &status); err != nil {
		return err
	}
	st.Online, st.Version = true, status.Version
	st.Indexers = &Indexers{Failing: []string{}}

	p := &parallel{st: st}
	p.run("health", func() error { return fetchHealth(ctx, c, "/api/v1/health", st, &p.mu) })
	p.run("indexers", func() error {
		var indexers []struct {
			ID     int    `json:"id"`
			Name   string `json:"name"`
			Enable bool   `json:"enable"`
		}
		if err := c.get(ctx, "/api/v1/indexer", nil, &indexers); err != nil {
			return err
		}
		var statuses []struct {
			IndexerID    int    `json:"indexerId"`
			DisabledTill string `json:"disabledTill"`
		}
		if err := c.get(ctx, "/api/v1/indexerstatus", nil, &statuses); err != nil {
			return err
		}
		failing := map[int]bool{}
		for _, s := range statuses {
			if t, err := time.Parse(time.RFC3339, s.DisabledTill); err == nil && t.After(time.Now()) {
				failing[s.IndexerID] = true
			}
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		st.Indexers.Total = len(indexers)
		for _, idx := range indexers {
			if idx.Enable {
				st.Indexers.Enabled++
				if failing[idx.ID] {
					st.Indexers.Failing = append(st.Indexers.Failing, idx.Name)
				}
			}
		}
		sort.Strings(st.Indexers.Failing)
		return nil
	})
	p.run("indexer stats", func() error {
		var stats struct {
			Indexers []struct {
				NumberOfQueries int `json:"numberOfQueries"`
				NumberOfGrabs   int `json:"numberOfGrabs"`
			} `json:"indexers"`
		}
		if err := c.get(ctx, "/api/v1/indexerstats", nil, &stats); err != nil {
			return err
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		for _, s := range stats.Indexers {
			st.Indexers.Queries += s.NumberOfQueries
			st.Indexers.Grabs += s.NumberOfGrabs
		}
		return nil
	})
	p.wait()
	return nil
}

// --- Bazarr ---

func fetchBazarr(ctx context.Context, c *client, st *Status) error {
	var episodes struct {
		Total int `json:"total"`
	}
	// The wanted list doubles as the connection check: Bazarr answers it on
	// every version, and with an error when the key is wrong.
	if err := c.get(ctx, "/api/episodes/wanted", url.Values{"start": {"0"}, "length": {"1"}}, &episodes); err != nil {
		return err
	}
	st.Online = true
	st.Subtitles = &Subtitles{WantedEpisodes: episodes.Total}

	p := &parallel{st: st}
	p.run("wanted movies", func() error {
		var movies struct {
			Total int `json:"total"`
		}
		if err := c.get(ctx, "/api/movies/wanted", url.Values{"start": {"0"}, "length": {"1"}}, &movies); err != nil {
			return err
		}
		p.mu.Lock()
		st.Subtitles.WantedMovies = movies.Total
		p.mu.Unlock()
		return nil
	})
	p.run("version", func() error {
		var status struct {
			Data struct {
				Version string `json:"bazarr_version"`
			} `json:"data"`
		}
		if err := c.get(ctx, "/api/system/status", nil, &status); err != nil {
			return err
		}
		p.mu.Lock()
		st.Version = status.Data.Version
		p.mu.Unlock()
		return nil
	})
	p.run("health", func() error {
		var health struct {
			Data []struct {
				Object string `json:"object"`
				Issue  string `json:"issue"`
			} `json:"data"`
		}
		if err := c.get(ctx, "/api/system/health", nil, &health); err != nil {
			return err
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		for _, h := range health.Data {
			st.Health = append(st.Health, HealthItem{Level: "warning", Source: h.Object, Message: h.Issue})
		}
		return nil
	})
	p.wait()
	return nil
}

// --- Tautulli (API v2) ---

// tautulliResponse wraps every Tautulli answer.
type tautulliResponse[T any] struct {
	Response struct {
		Result  string `json:"result"`
		Message string `json:"message"`
		Data    T      `json:"data"`
	} `json:"response"`
}

func tautulliCall[T any](ctx context.Context, c *client, cmd string) (T, error) {
	var out tautulliResponse[T]
	err := c.get(ctx, "/api/v2", url.Values{"cmd": {cmd}}, &out)
	if err == nil && out.Response.Result != "success" {
		err = fmt.Errorf("tautulli: %s", firstNonEmpty(out.Response.Message, out.Response.Result))
	}
	return out.Response.Data, err
}

// flexNumber reads a number Tautulli may send as a string.
type flexNumber float64

func (n *flexNumber) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*n = 0
		return nil
	}
	*n = flexNumber(v)
	return nil
}

func fetchTautulli(ctx context.Context, c *client, st *Status) error {
	activity, err := tautulliCall[struct {
		StreamCount    flexNumber `json:"stream_count"`
		TotalBandwidth flexNumber `json:"total_bandwidth"`
		Sessions       []struct {
			FriendlyName      string     `json:"friendly_name"`
			User              string     `json:"user"`
			FullTitle         string     `json:"full_title"`
			State             string     `json:"state"`
			ProgressPercent   flexNumber `json:"progress_percent"`
			Player            string     `json:"player"`
			Product           string     `json:"product"`
			TranscodeDecision string     `json:"transcode_decision"`
			QualityProfile    string     `json:"quality_profile"`
		} `json:"sessions"`
	}](ctx, c, "get_activity")
	if err != nil {
		return err
	}
	st.Online = true
	streams := &Streams{Count: int(activity.StreamCount), Bandwidth: int(activity.TotalBandwidth), Sessions: []Session{}}
	for _, s := range activity.Sessions {
		player := s.Player
		if s.Product != "" && s.Product != s.Player {
			player = strings.TrimSpace(s.Product + " · " + s.Player)
		}
		streams.Sessions = append(streams.Sessions, Session{
			User:     firstNonEmpty(s.FriendlyName, s.User),
			Title:    s.FullTitle,
			State:    strings.ToLower(s.State),
			Progress: clamp01(float64(s.ProgressPercent) / 100),
			Player:   player,
			Decision: strings.ToLower(s.TranscodeDecision),
			Quality:  s.QualityProfile,
		})
	}
	st.Streams = streams

	p := &parallel{st: st}
	p.run("version", func() error {
		info, err := tautulliCall[struct {
			Version string `json:"tautulli_version"`
		}](ctx, c, "get_tautulli_info")
		if err != nil {
			return err
		}
		p.mu.Lock()
		st.Version = strings.TrimPrefix(info.Version, "v")
		p.mu.Unlock()
		return nil
	})
	p.run("plex", func() error {
		server, err := tautulliCall[struct {
			Connected bool `json:"connected"`
		}](ctx, c, "server_status")
		if err != nil {
			return err
		}
		if !server.Connected {
			p.mu.Lock()
			st.Health = append(st.Health, HealthItem{Level: "error", Source: "Plex", Message: "Tautulli is not connected to the Plex server"})
			p.mu.Unlock()
		}
		return nil
	})
	p.wait()
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func clamp01(v float64) float64 {
	return max(0, min(1, v))
}
