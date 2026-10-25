// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package mediaapps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gjhuerte/qui/internal/models"
)

type memoryStore struct {
	items []*models.AppInstance
}

func (m *memoryStore) List(context.Context) ([]*models.AppInstance, error) { return m.items, nil }
func (m *memoryStore) Get(_ context.Context, id int) (*models.AppInstance, error) {
	for _, it := range m.items {
		if it.ID == id {
			c := *it
			return &c, nil
		}
	}
	return nil, models.ErrAppInstanceNotFound
}
func (m *memoryStore) Create(_ context.Context, inst *models.AppInstance) (*models.AppInstance, error) {
	inst.ID = len(m.items) + 1
	m.items = append(m.items, inst)
	return inst, nil
}
func (m *memoryStore) Update(_ context.Context, inst *models.AppInstance) (*models.AppInstance, error) {
	for i, it := range m.items {
		if it.ID == inst.ID {
			m.items[i] = inst
			return inst, nil
		}
	}
	return nil, models.ErrAppInstanceNotFound
}
func (m *memoryStore) Delete(context.Context, int) error { return nil }

// fakeApp answers like the real app for the given routes and checks the key.
func fakeApp(t *testing.T, appType models.AppType, routes map[string]string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Api-Key")
		if appType == models.AppTypeTautulli || appType == models.AppTypeBazarr {
			if q := r.URL.Query().Get("apikey"); q != "" {
				key = q
			}
		}
		if key != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		route := r.URL.Path
		if cmd := r.URL.Query().Get("cmd"); cmd != "" {
			route += "?cmd=" + cmd
		}
		body, ok := routes[route]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func status(t *testing.T, appType models.AppType, routes map[string]string, key string) *Status {
	t.Helper()
	server := fakeApp(t, appType, routes)
	svc := NewService(&memoryStore{})
	st, err := svc.Test(context.Background(), 0, Input{Type: appType, Name: "test", BaseURL: server.URL, APIKey: key})
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestSonarrStatus(t *testing.T) {
	st := status(t, models.AppTypeSonarr, map[string]string{
		"/api/v3/system/status":  `{"version":"4.0.9.2244"}`,
		"/api/v3/health":         `[{"source":"IndexerStatusCheck","type":"warning","message":"Indexers unavailable"}]`,
		"/api/v3/queue":          `{"totalRecords":3,"records":[{"title":"Show.S01E01","status":"downloading","trackedDownloadStatus":"ok","trackedDownloadState":"downloading","size":1000,"sizeleft":250,"timeleft":"00:05:00","downloadClient":"qBittorrent"}]}`,
		"/api/v3/wanted/missing": `{"totalRecords":42}`,
		"/api/v3/diskspace":      `[{"path":"/tv","freeSpace":100,"totalSpace":1000},{"path":"/empty","freeSpace":0,"totalSpace":0}]`,
		"/api/v3/calendar":       `[{"series":{"title":"Show"},"seasonNumber":1,"episodeNumber":2,"title":"Pilot","airDateUtc":"2026-10-26T01:00:00Z","hasFile":false}]`,
	}, "secret")

	if !st.Online || st.Version != "4.0.9.2244" || len(st.Warnings) != 0 {
		t.Fatalf("status %+v", st)
	}
	if st.Queue.Total != 3 || st.Queue.Items[0].Progress != 0.75 || st.Queue.Items[0].Client != "qBittorrent" {
		t.Fatalf("queue %+v", st.Queue)
	}
	if *st.Missing != 42 || len(st.Disks) != 1 || len(st.Health) != 1 || st.Health[0].Level != "warning" {
		t.Fatalf("missing %d disks %+v health %+v", *st.Missing, st.Disks, st.Health)
	}
	if len(st.Upcoming) != 1 || st.Upcoming[0].Subtitle != "S01E02 · Pilot" {
		t.Fatalf("upcoming %+v", st.Upcoming)
	}
}

func TestRadarrMissingFallsBackToMovieList(t *testing.T) {
	st := status(t, models.AppTypeRadarr, map[string]string{
		"/api/v3/system/status": `{"version":"5.2.0"}`,
		"/api/v3/health":        `[]`,
		"/api/v3/queue":         `{"totalRecords":0,"records":[]}`,
		"/api/v3/movie":         `[{"monitored":true,"hasFile":false,"isAvailable":true},{"monitored":true,"hasFile":true,"isAvailable":true},{"monitored":false,"hasFile":false,"isAvailable":true}]`,
		"/api/v3/diskspace":     `[]`,
		"/api/v3/calendar":      `[{"title":"Film","year":2026,"digitalRelease":"2026-10-27T00:00:00Z","hasFile":false}]`,
	}, "secret")
	if !st.Online || st.Missing == nil || *st.Missing != 1 {
		t.Fatalf("status %+v", st)
	}
	if len(st.Upcoming) != 1 || st.Upcoming[0].Title != "Film (2026)" {
		t.Fatalf("upcoming %+v", st.Upcoming)
	}
}

func TestWrongKeyIsOffline(t *testing.T) {
	st := status(t, models.AppTypeSonarr, map[string]string{"/api/v3/system/status": `{}`}, "wrong")
	if st.Online || !strings.Contains(st.Error, "API key") {
		t.Fatalf("status %+v", st)
	}
}

func TestProwlarrStatus(t *testing.T) {
	st := status(t, models.AppTypeProwlarr, map[string]string{
		"/api/v1/system/status": `{"version":"1.24.3"}`,
		"/api/v1/health":        `[]`,
		"/api/v1/indexer":       `[{"id":1,"name":"Alpha","enable":true},{"id":2,"name":"Beta","enable":true},{"id":3,"name":"Gamma","enable":false}]`,
		"/api/v1/indexerstatus": `[{"indexerId":2,"disabledTill":"2999-01-01T00:00:00Z"},{"indexerId":1,"disabledTill":"2000-01-01T00:00:00Z"}]`,
		"/api/v1/indexerstats":  `{"indexers":[{"numberOfQueries":10,"numberOfGrabs":2},{"numberOfQueries":5,"numberOfGrabs":1}]}`,
	}, "secret")
	i := st.Indexers
	if !st.Online || i.Total != 3 || i.Enabled != 2 || len(i.Failing) != 1 || i.Failing[0] != "Beta" || i.Queries != 15 || i.Grabs != 3 {
		t.Fatalf("indexers %+v warnings %v", i, st.Warnings)
	}
}

func TestBazarrStatus(t *testing.T) {
	st := status(t, models.AppTypeBazarr, map[string]string{
		"/api/episodes/wanted": `{"data":[],"total":12}`,
		"/api/movies/wanted":   `{"data":[],"total":4}`,
		"/api/system/status":   `{"data":{"bazarr_version":"1.4.5"}}`,
		"/api/system/health":   `{"data":[{"object":"Sonarr","issue":"Unable to connect"}]}`,
	}, "secret")
	if !st.Online || st.Version != "1.4.5" || st.Subtitles.WantedEpisodes != 12 || st.Subtitles.WantedMovies != 4 || len(st.Health) != 1 {
		t.Fatalf("status %+v", st)
	}
}

func TestTautulliStatus(t *testing.T) {
	st := status(t, models.AppTypeTautulli, map[string]string{
		"/api/v2?cmd=get_activity":      `{"response":{"result":"success","data":{"stream_count":"1","total_bandwidth":8000,"sessions":[{"friendly_name":"Gab","full_title":"Show - Pilot","state":"playing","progress_percent":"40","player":"Living Room","product":"Plex for Android (TV)","transcode_decision":"transcode","quality_profile":"1080p"}]}}}`,
		"/api/v2?cmd=get_tautulli_info": `{"response":{"result":"success","data":{"tautulli_version":"v2.14.6"}}}`,
		"/api/v2?cmd=server_status":     `{"response":{"result":"success","data":{"connected":false}}}`,
	}, "secret")
	s := st.Streams
	if !st.Online || st.Version != "2.14.6" || s.Count != 1 || s.Bandwidth != 8000 || s.Sessions[0].Progress != 0.4 || s.Sessions[0].User != "Gab" {
		t.Fatalf("status %+v streams %+v", st, s)
	}
	if len(st.Health) != 1 || st.Health[0].Source != "Plex" {
		t.Fatalf("health %+v", st.Health)
	}
}

func TestTautulliErrorHidesKey(t *testing.T) {
	svc := NewService(&memoryStore{})
	st, err := svc.Test(context.Background(), 0, Input{Type: models.AppTypeTautulli, Name: "t", BaseURL: "http://127.0.0.1:1", APIKey: "supersecretkey"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Online || strings.Contains(st.Error, "supersecretkey") {
		t.Fatalf("error %q", st.Error)
	}
}

func TestValidationAndKeepKey(t *testing.T) {
	store := &memoryStore{}
	svc := NewService(store)
	ctx := context.Background()
	for _, in := range []Input{
		{Type: "plex", Name: "x", BaseURL: "http://h:1", APIKey: "k"},
		{Type: models.AppTypeSonarr, Name: "", BaseURL: "http://h:1", APIKey: "k"},
		{Type: models.AppTypeSonarr, Name: "x", BaseURL: "h:1", APIKey: "k"},
		{Type: models.AppTypeSonarr, Name: "x", BaseURL: "http://h:1", APIKey: ""},
	} {
		if _, err := svc.Create(ctx, in); err == nil {
			t.Errorf("accepted %+v", in)
		}
	}

	created, err := svc.Create(ctx, Input{Type: models.AppTypeSonarr, Name: " TV ", BaseURL: "http://h:8989/", APIKey: "k1"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "TV" || created.BaseURL != "http://h:8989" || !created.Enabled {
		t.Fatalf("created %+v", created)
	}
	updated, err := svc.Update(ctx, created.ID, Input{Type: models.AppTypeSonarr, Name: "TV 4K", BaseURL: "http://h:8990"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.APIKey != "k1" || updated.Name != "TV 4K" {
		t.Fatalf("updated %+v", updated)
	}
}

func TestStatusSkipsDisabled(t *testing.T) {
	server := fakeApp(t, models.AppTypeProwlarr, map[string]string{"/api/v1/system/status": `{"version":"1"}`})
	store := &memoryStore{items: []*models.AppInstance{
		{ID: 1, Type: models.AppTypeProwlarr, Name: "on", BaseURL: server.URL, APIKey: "secret", Enabled: true},
		{ID: 2, Type: models.AppTypeProwlarr, Name: "off", BaseURL: server.URL, APIKey: "secret", Enabled: false},
	}}
	statuses, err := NewService(store).Status(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || statuses[0].Instance.Name != "on" || !statuses[0].Online {
		t.Fatalf("statuses %+v", statuses)
	}
}

func TestLibraries(t *testing.T) {
	radarr := fakeApp(t, models.AppTypeRadarr, map[string]string{
		"/api/v3/movie": `[{"id":1,"title":"Zeta","year":2020,"titleSlug":"zeta-2020","monitored":true,"hasFile":true,"isAvailable":true,"sizeOnDisk":5000,"movieFile":{"quality":{"quality":{"name":"Bluray-1080p"}}}},{"id":2,"title":"alpha","tmdbId":99,"monitored":true,"hasFile":false,"isAvailable":true}]`,
	})
	sonarr := fakeApp(t, models.AppTypeSonarr, map[string]string{
		"/api/v3/series": `[{"id":7,"title":"Show","titleSlug":"show","status":"continuing","monitored":true,"statistics":{"seasonCount":2,"episodeFileCount":8,"episodeCount":10,"totalEpisodeCount":12,"sizeOnDisk":900}}]`,
	})
	bazarr := fakeApp(t, models.AppTypeBazarr, map[string]string{
		"/api/episodes/wanted": `{"data":[{"seriesTitle":"Show","episode_number":"1x02","episodeTitle":"Second","sonarrSeriesId":7,"missing_subtitles":[{"name":"English","code2":"en","forced":false,"hi":true}]}],"total":1}`,
		"/api/movies/wanted":   `{"data":[{"title":"Alpha","radarrId":2,"missing_subtitles":[{"name":"Spanish","code2":"es"}]}],"total":1}`,
	})
	store := &memoryStore{items: []*models.AppInstance{
		{ID: 1, Type: models.AppTypeRadarr, Name: "Radarr", BaseURL: radarr.URL + "/", APIKey: "secret", Enabled: true},
		{ID: 2, Type: models.AppTypeRadarr, Name: "Radarr 4K", BaseURL: "http://127.0.0.1:1", APIKey: "secret", Enabled: true},
		{ID: 3, Type: models.AppTypeSonarr, Name: "Sonarr", BaseURL: sonarr.URL, APIKey: "secret", Enabled: true},
		{ID: 4, Type: models.AppTypeBazarr, Name: "Bazarr", BaseURL: bazarr.URL, APIKey: "secret", Enabled: true},
	}}
	svc := NewService(store)
	ctx := context.Background()

	movies, err := svc.Movies(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies.Items) != 2 || movies.Items[0].Title != "alpha" || movies.Items[1].Quality != "Bluray-1080p" {
		t.Fatalf("movies %+v", movies.Items)
	}
	if movies.Items[0].Link != radarr.URL+"/movie/99" || movies.Items[1].Link != radarr.URL+"/movie/zeta-2020" {
		t.Fatalf("links %q %q", movies.Items[0].Link, movies.Items[1].Link)
	}
	if len(movies.Instances) != 2 || len(movies.Errors) != 1 || movies.Errors[0].Name != "Radarr 4K" {
		t.Fatalf("instances %+v errors %+v", movies.Instances, movies.Errors)
	}

	series, err := svc.Series(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Items) != 1 || series.Items[0].Missing != 2 || series.Items[0].Link != sonarr.URL+"/series/show" {
		t.Fatalf("series %+v", series.Items)
	}

	wanted, err := svc.WantedSubtitles(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(wanted.Items) != 2 || wanted.Items[0].Kind != "movie" || wanted.Items[1].Episode != "1x02 · Second" || wanted.Items[1].Languages[0] != "English (HI)" {
		t.Fatalf("wanted %+v", wanted.Items)
	}
}

func TestQueueIssues(t *testing.T) {
	st := status(t, models.AppTypeRadarr, map[string]string{
		"/api/v3/system/status": `{"version":"5"}`,
		"/api/v3/queue":         `{"totalRecords":2,"records":[{"title":"ok","trackedDownloadStatus":"ok","size":10,"sizeleft":5},{"title":"stuck","trackedDownloadStatus":"warning","trackedDownloadState":"importPending","statusMessages":[{"title":"stuck","messages":["No files found are eligible for import"]}]}]}`,
	}, "secret")
	if st.Queue.Total != 2 || len(st.QueueIssues) != 1 || st.QueueIssues[0].Messages[0] != "No files found are eligible for import" {
		t.Fatalf("queue %+v issues %+v", st.Queue, st.QueueIssues)
	}
}
