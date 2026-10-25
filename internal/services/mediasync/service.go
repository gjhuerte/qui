// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// Package mediasync copies watched state between media servers (Jellyfin and
// Plex). A pair links two servers with a direction, user maps say which
// accounts are the same person, and each run marks as watched on one side
// what that person watched on the other. Items match on IMDb, TMDB and TVDB
// IDs. It never marks anything unwatched.
package mediasync

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/gjhuerte/qui/internal/models"
)

// ServerStore persists media servers (models.MediaSyncStore).
type ServerStore interface {
	ListServers(ctx context.Context) ([]*models.MediaServer, error)
	GetServer(ctx context.Context, id int) (*models.MediaServer, error)
	CreateServer(ctx context.Context, srv *models.MediaServer) (*models.MediaServer, error)
	UpdateServer(ctx context.Context, srv *models.MediaServer) (*models.MediaServer, error)
	DeleteServer(ctx context.Context, id int) error
}

// PairStore persists sync pairs (models.MediaSyncStore).
type PairStore interface {
	ListPairs(ctx context.Context) ([]*models.SyncPair, error)
	GetPair(ctx context.Context, id int) (*models.SyncPair, error)
	CreatePair(ctx context.Context, p *models.SyncPair) (*models.SyncPair, error)
	UpdatePair(ctx context.Context, p *models.SyncPair) (*models.SyncPair, error)
	DeletePair(ctx context.Context, id int) error
}

// UserMapStore persists the account maps of a pair (models.MediaSyncStore).
type UserMapStore interface {
	ListUserMaps(ctx context.Context, pairID int) ([]models.SyncUserMap, error)
	ReplaceUserMaps(ctx context.Context, pairID int, maps []models.SyncUserMap) error
}

// RunStore persists the sync log (models.MediaSyncStore).
type RunStore interface {
	StartRun(ctx context.Context, pairID int, source string) (*models.SyncRun, error)
	FinishRun(ctx context.Context, run *models.SyncRun) error
	GetRun(ctx context.Context, id int64) (*models.SyncRun, error)
	ListRuns(ctx context.Context, pairID, limit int) ([]*models.SyncRun, error)
	LastRunStarted(ctx context.Context, pairID int) (*time.Time, error)
}

type Store interface {
	ServerStore
	PairStore
	UserMapStore
	RunStore
	FailInterruptedRuns(ctx context.Context) error
	PruneRuns(ctx context.Context, before time.Time) error
}

const (
	requestTimeout = 15 * time.Second
	// A full library listing of a large server takes a while.
	libraryTimeout = 5 * time.Minute
	statusCacheTTL = 20 * time.Second
	tickInterval   = time.Minute
	historyDays    = 30
	// maxRunItems caps the titles stored per run; the first sync of a big
	// library can mark thousands.
	maxRunItems = 1000

	MinIntervalMinutes = 5
	MaxIntervalMinutes = 7 * 24 * 60
)

var (
	// ErrInvalid is a validation error the handler maps to 400.
	ErrInvalid = errors.New("invalid input")
	// ErrRunning: the pair is already syncing. The handler maps it to 409.
	ErrRunning = errors.New("this pair is already syncing")
)

// User is an account on a media server.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Admin bool   `json:"admin"`
}

// Library is one library on a server. Items is unknown on Jellyfin.
type Library struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Items *int   `json:"items,omitempty"`
}

// Session is one stream that is playing now.
type Session struct {
	User        string  `json:"user"`
	Title       string  `json:"title"`
	Client      string  `json:"client"`
	State       string  `json:"state"` // playing | paused
	Progress    float64 `json:"progress"`
	Transcoding bool    `json:"transcoding"`
}

// ServerStatus is what the media servers dashboard shows for one server.
type ServerStatus struct {
	Server     *models.MediaServer `json:"server"`
	Online     bool                `json:"online"`
	Error      string              `json:"error,omitempty"`
	ServerName string              `json:"serverName,omitempty"`
	Version    string              `json:"version,omitempty"`
	Platform   string              `json:"platform,omitempty"`
	Movies     int                 `json:"movies"`
	Series     int                 `json:"series"`
	Episodes   int                 `json:"episodes"`
	// Users is 0 on Plex: the count needs plex.tv.
	Users     int       `json:"users"`
	Libraries []Library `json:"libraries"`
	Sessions  []Session `json:"sessions"`
}

// backend is one media server API.
type backend interface {
	status(ctx context.Context) (*ServerStatus, error)
	users(ctx context.Context) ([]User, error)
	library(ctx context.Context, userID string) ([]Item, error)
	markWatched(ctx context.Context, userID string, item Item) error
}

type Service struct {
	store  Store
	logger zerolog.Logger
	// newBackend builds the API client of a server; tests replace it.
	newBackend func(srv *models.MediaServer, timeout time.Duration) backend

	// ctx is the lifetime of background runs, set by Start.
	ctx context.Context

	mu sync.Mutex
	// running holds the cancel func of each pair that is syncing.
	running  map[int]context.CancelFunc
	cached   []*ServerStatus
	cachedAt time.Time
}

func NewService(store Store) *Service {
	return &Service{
		store:      store,
		logger:     log.Logger.With().Str("module", "mediasync").Logger(),
		newBackend: newBackend,
		ctx:        context.Background(),
		running:    map[int]context.CancelFunc{},
	}
}

func newBackend(srv *models.MediaServer, timeout time.Duration) backend {
	hc := &http.Client{Timeout: timeout}
	if srv.Type == models.MediaServerPlex {
		return &plex{srv: srv, http: hc, plexTV: defaultPlexTV}
	}
	return &jellyfin{srv: srv, http: hc}
}

// ServerInput is the editable part of a server. An empty Token on update
// keeps the stored token.
type ServerInput struct {
	Type    models.MediaServerType `json:"type"`
	Name    string                 `json:"name"`
	BaseURL string                 `json:"baseUrl"`
	Token   string                 `json:"token"`
	Enabled *bool                  `json:"enabled"`
}

func (in *ServerInput) normalize() error {
	in.Name = strings.TrimSpace(in.Name)
	in.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	in.Token = strings.TrimSpace(in.Token)
	if !in.Type.Valid() {
		return fmt.Errorf("%w: unknown server type %q", ErrInvalid, in.Type)
	}
	if in.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalid)
	}
	parsed, err := url.Parse(in.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%w: the URL must look like http://192.168.1.10:8096", ErrInvalid)
	}
	return nil
}

func (s *Service) ListServers(ctx context.Context) ([]*models.MediaServer, error) {
	return s.store.ListServers(ctx)
}

func (s *Service) CreateServer(ctx context.Context, in ServerInput) (*models.MediaServer, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	if in.Token == "" {
		return nil, fmt.Errorf("%w: a token is required", ErrInvalid)
	}
	enabled := in.Enabled == nil || *in.Enabled
	defer s.invalidate()
	return s.store.CreateServer(ctx, &models.MediaServer{Type: in.Type, Name: in.Name, BaseURL: in.BaseURL, Token: in.Token, Enabled: enabled})
}

func (s *Service) UpdateServer(ctx context.Context, id int, in ServerInput) (*models.MediaServer, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	current, err := s.store.GetServer(ctx, id)
	if err != nil {
		return nil, err
	}
	current.Type, current.Name, current.BaseURL = in.Type, in.Name, in.BaseURL
	if in.Token != "" {
		current.Token = in.Token
	}
	if in.Enabled != nil {
		current.Enabled = *in.Enabled
	}
	defer s.invalidate()
	return s.store.UpdateServer(ctx, current)
}

func (s *Service) DeleteServer(ctx context.Context, id int) error {
	defer s.invalidate()
	return s.store.DeleteServer(ctx, id)
}

// TestServer checks a connection before it is saved. With id set and no
// token, it uses the stored token.
func (s *Service) TestServer(ctx context.Context, id int, in ServerInput) (*ServerStatus, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	if in.Token == "" && id > 0 {
		current, err := s.store.GetServer(ctx, id)
		if err != nil {
			return nil, err
		}
		in.Token = current.Token
	}
	if in.Token == "" {
		return nil, fmt.Errorf("%w: a token is required", ErrInvalid)
	}
	srv := &models.MediaServer{ID: id, Type: in.Type, Name: in.Name, BaseURL: in.BaseURL, Token: in.Token, Enabled: true}
	return s.fetchStatus(ctx, srv), nil
}

func (s *Service) fetchStatus(ctx context.Context, srv *models.MediaServer) *ServerStatus {
	st, err := s.newBackend(srv, requestTimeout).status(ctx)
	if err != nil {
		st = &ServerStatus{Error: err.Error()}
	}
	st.Server = srv
	if st.Libraries == nil {
		st.Libraries = []Library{}
	}
	if st.Sessions == nil {
		st.Sessions = []Session{}
	}
	return st
}

// Status returns the status of every enabled server, queried in parallel.
// refresh skips the short cache.
func (s *Service) Status(ctx context.Context, refresh bool) ([]*ServerStatus, error) {
	s.mu.Lock()
	if !refresh && s.cached != nil && time.Since(s.cachedAt) < statusCacheTTL {
		cached := s.cached
		s.mu.Unlock()
		return cached, nil
	}
	s.mu.Unlock()

	servers, err := s.store.ListServers(ctx)
	if err != nil {
		return nil, err
	}
	// The result is cached for other requests, so a closed browser tab must
	// not cut the round short.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*requestTimeout)
	defer cancel()

	results := make([]*ServerStatus, len(servers))
	var wg sync.WaitGroup
	for i, srv := range servers {
		if !srv.Enabled {
			continue
		}
		wg.Go(func() { results[i] = s.fetchStatus(ctx, srv) })
	}
	wg.Wait()

	out := make([]*ServerStatus, 0, len(servers))
	for _, st := range results {
		if st != nil {
			out = append(out, st)
		}
	}
	s.mu.Lock()
	s.cached, s.cachedAt = out, time.Now()
	s.mu.Unlock()
	return out, nil
}

func (s *Service) invalidate() {
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
}

// Users lists the accounts on a server.
func (s *Service) Users(ctx context.Context, serverID int) ([]User, error) {
	srv, err := s.store.GetServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	return s.newBackend(srv, requestTimeout).users(ctx)
}

// PairInput is the editable part of a pair. The servers are set on create only.
type PairInput struct {
	ServerAID       int                  `json:"serverAId"`
	ServerBID       int                  `json:"serverBId"`
	Direction       models.SyncDirection `json:"direction"`
	Enabled         bool                 `json:"enabled"`
	IntervalMinutes int                  `json:"intervalMinutes"`
}

func (in *PairInput) validate() error {
	if !in.Direction.Valid() {
		return fmt.Errorf("%w: unknown direction %q", ErrInvalid, in.Direction)
	}
	if in.IntervalMinutes < MinIntervalMinutes || in.IntervalMinutes > MaxIntervalMinutes {
		return fmt.Errorf("%w: the interval must be between %d and %d minutes", ErrInvalid, MinIntervalMinutes, MaxIntervalMinutes)
	}
	return nil
}

func (s *Service) ListPairs(ctx context.Context) ([]*models.SyncPair, error) {
	return s.store.ListPairs(ctx)
}

func (s *Service) CreatePair(ctx context.Context, in PairInput) (*models.SyncPair, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	if in.ServerAID == in.ServerBID {
		return nil, fmt.Errorf("%w: pick two different servers", ErrInvalid)
	}
	for _, id := range []int{in.ServerAID, in.ServerBID} {
		if _, err := s.store.GetServer(ctx, id); err != nil {
			if errors.Is(err, models.ErrMediaServerNotFound) {
				return nil, fmt.Errorf("%w: server %d does not exist", ErrInvalid, id)
			}
			return nil, err
		}
	}
	return s.store.CreatePair(ctx, &models.SyncPair{
		ServerAID: in.ServerAID, ServerBID: in.ServerBID, Direction: in.Direction,
		Enabled: in.Enabled, IntervalMinutes: in.IntervalMinutes,
	})
}

func (s *Service) UpdatePair(ctx context.Context, id int, in PairInput) (*models.SyncPair, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	current, err := s.store.GetPair(ctx, id)
	if err != nil {
		return nil, err
	}
	current.Direction, current.Enabled, current.IntervalMinutes = in.Direction, in.Enabled, in.IntervalMinutes
	return s.store.UpdatePair(ctx, current)
}

func (s *Service) DeletePair(ctx context.Context, id int) error {
	return s.store.DeletePair(ctx, id)
}

func (s *Service) UserMaps(ctx context.Context, pairID int) ([]models.SyncUserMap, error) {
	if _, err := s.store.GetPair(ctx, pairID); err != nil {
		return nil, err
	}
	return s.store.ListUserMaps(ctx, pairID)
}

// SetUserMaps replaces the account maps of a pair. Duplicates are dropped.
func (s *Service) SetUserMaps(ctx context.Context, pairID int, maps []models.SyncUserMap) ([]models.SyncUserMap, error) {
	if _, err := s.store.GetPair(ctx, pairID); err != nil {
		return nil, err
	}
	seen := map[[2]string]bool{}
	clean := make([]models.SyncUserMap, 0, len(maps))
	for _, m := range maps {
		m.UserAID, m.UserBID = strings.TrimSpace(m.UserAID), strings.TrimSpace(m.UserBID)
		if m.UserAID == "" || m.UserBID == "" {
			return nil, fmt.Errorf("%w: each map needs an account on both servers", ErrInvalid)
		}
		key := [2]string{m.UserAID, m.UserBID}
		if seen[key] {
			continue
		}
		seen[key] = true
		clean = append(clean, m)
	}
	if err := s.store.ReplaceUserMaps(ctx, pairID, clean); err != nil {
		return nil, err
	}
	return s.store.ListUserMaps(ctx, pairID)
}

func (s *Service) ListRuns(ctx context.Context, pairID, limit int) ([]*models.SyncRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.store.ListRuns(ctx, pairID, limit)
}

func (s *Service) GetRun(ctx context.Context, id int64) (*models.SyncRun, error) {
	return s.store.GetRun(ctx, id)
}

// Start marks runs a restart interrupted as failed, then runs the scheduler
// until ctx ends.
func (s *Service) Start(ctx context.Context) {
	s.ctx = ctx
	if err := s.store.FailInterruptedRuns(ctx); err != nil {
		s.logger.Warn().Err(err).Msg("mediasync: failed to close interrupted runs")
	}
	go func() {
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		lastPrune := time.Time{}
		for {
			s.runDue(ctx)
			if time.Since(lastPrune) > 24*time.Hour {
				lastPrune = time.Now()
				if err := s.store.PruneRuns(ctx, time.Now().AddDate(0, 0, -historyDays)); err != nil && ctx.Err() == nil {
					s.logger.Warn().Err(err).Msg("mediasync: failed to prune the sync log")
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *Service) runDue(ctx context.Context) {
	pairs, err := s.store.ListPairs(ctx)
	if err != nil {
		if ctx.Err() == nil {
			s.logger.Warn().Err(err).Msg("mediasync: failed to list pairs")
		}
		return
	}
	for _, p := range pairs {
		if !p.Enabled || ctx.Err() != nil {
			continue
		}
		last, err := s.store.LastRunStarted(ctx, p.ID)
		if err != nil {
			s.logger.Warn().Err(err).Int("pair", p.ID).Msg("mediasync: failed to read the last run")
			continue
		}
		if last != nil && time.Since(*last) < time.Duration(p.IntervalMinutes)*time.Minute-tickInterval/2 {
			continue
		}
		run, runCtx, err := s.begin(ctx, ctx, p.ID, models.SyncSourceSchedule)
		if err != nil {
			if !errors.Is(err, ErrRunning) {
				s.logger.Warn().Err(err).Int("pair", p.ID).Msg("mediasync: failed to start a scheduled run")
			}
			continue
		}
		s.execute(runCtx, p, run)
	}
}

// RunNow starts a manual sync of a pair in the background and returns the
// new run in the running state.
func (s *Service) RunNow(ctx context.Context, pairID int) (*models.SyncRun, error) {
	pair, err := s.store.GetPair(ctx, pairID)
	if err != nil {
		return nil, err
	}
	// The run outlives the request, so it hangs off the service lifetime.
	run, runCtx, err := s.begin(ctx, s.ctx, pairID, models.SyncSourceManual)
	if err != nil {
		return nil, err
	}
	go s.execute(runCtx, pair, run)
	return run, nil
}

// Cancel stops the sync of a pair. The run keeps what it marked so far.
func (s *Service) Cancel(pairID int) error {
	s.mu.Lock()
	cancel := s.running[pairID]
	s.mu.Unlock()
	if cancel == nil {
		return fmt.Errorf("%w: this pair is not syncing", ErrInvalid)
	}
	cancel()
	return nil
}

// begin claims the pair, records a running run, and returns the context the
// run works under: parent, cancelled by Cancel.
func (s *Service) begin(ctx, parent context.Context, pairID int, source string) (*models.SyncRun, context.Context, error) {
	runCtx, cancel := context.WithCancel(parent)
	s.mu.Lock()
	if s.running[pairID] != nil {
		s.mu.Unlock()
		cancel()
		return nil, nil, ErrRunning
	}
	s.running[pairID] = cancel
	s.mu.Unlock()

	run, err := s.store.StartRun(ctx, pairID, source)
	if err != nil {
		s.release(pairID)
		return nil, nil, err
	}
	return run, runCtx, nil
}

func (s *Service) release(pairID int) {
	s.mu.Lock()
	if cancel := s.running[pairID]; cancel != nil {
		cancel()
	}
	delete(s.running, pairID)
	s.mu.Unlock()
}

// execute runs a claimed sync and stores its outcome.
func (s *Service) execute(ctx context.Context, pair *models.SyncPair, run *models.SyncRun) {
	defer s.release(pair.ID)

	problems := s.sync(ctx, pair, run)
	if ctx.Err() != nil {
		problems = append(problems, "the sync was stopped before the end")
	}
	switch {
	case len(problems) == 0 && run.Failed == 0:
		run.Status = models.SyncRunSuccess
	case run.MarkedA+run.MarkedB > 0 || (len(problems) < run.Users && run.Users > 0):
		run.Status = models.SyncRunPartial
	default:
		run.Status = models.SyncRunFailed
	}
	if run.Failed > 0 {
		problems = append(problems, fmt.Sprintf("%d titles failed to mark", run.Failed))
	}
	run.Message = strings.Join(problems, "; ")
	if len(run.Message) > 2000 {
		run.Message = run.Message[:2000]
	}

	// Store the outcome even when a cancel or shutdown stopped the run.
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := s.store.FinishRun(saveCtx, run); err != nil {
		s.logger.Error().Err(err).Int64("run", run.ID).Msg("mediasync: failed to save the run")
	}

	event := s.logger.Debug()
	switch {
	case run.Status != models.SyncRunSuccess:
		event = s.logger.Warn().Str("problems", run.Message)
	case run.MarkedA+run.MarkedB > 0:
		event = s.logger.Info()
	}
	event.Int("pair", pair.ID).Str("source", run.Source).Int("markedA", run.MarkedA).Int("markedB", run.MarkedB).
		Int("notFound", run.NotFound).Msgf("mediasync: run %s", run.Status)
}

// sync does the work of one run and returns the problems to report. The
// totals and items go into run.
func (s *Service) sync(ctx context.Context, pair *models.SyncPair, run *models.SyncRun) []string {
	serverA, err := s.store.GetServer(ctx, pair.ServerAID)
	if err != nil {
		return []string{"server A: " + err.Error()}
	}
	serverB, err := s.store.GetServer(ctx, pair.ServerBID)
	if err != nil {
		return []string{"server B: " + err.Error()}
	}
	maps, err := s.store.ListUserMaps(ctx, pair.ID)
	if err != nil {
		return []string{err.Error()}
	}
	if len(maps) == 0 {
		return []string{"no accounts are mapped for this pair"}
	}

	a := s.newBackend(serverA, libraryTimeout)
	b := s.newBackend(serverB, libraryTimeout)
	run.Users = len(maps)

	var problems []string
	for _, m := range maps {
		if ctx.Err() != nil {
			break
		}
		libA, err := a.library(ctx, m.UserAID)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s on %s: %v", m.UserAName, serverA.Name, err))
			continue
		}
		libB, err := b.library(ctx, m.UserBID)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s on %s: %v", m.UserBName, serverB.Name, err))
			continue
		}
		if pair.Direction.ToB() {
			marks, notFound := plan(libA, libB)
			run.NotFound += notFound
			run.MarkedB += s.apply(ctx, b, m.UserBID, m.UserBName, "b", marks, run)
		}
		if pair.Direction.ToA() {
			marks, notFound := plan(libB, libA)
			run.NotFound += notFound
			run.MarkedA += s.apply(ctx, a, m.UserAID, m.UserAName, "a", marks, run)
		}
	}
	return problems
}

// apply marks the planned items watched and returns how many succeeded.
func (s *Service) apply(ctx context.Context, target backend, userID, userName, side string, marks []mark, run *models.SyncRun) int {
	done := 0
	for _, mk := range marks {
		if ctx.Err() != nil {
			break
		}
		item := models.SyncRunItem{UserName: userName, Target: side, Title: mk.Target.DisplayTitle()}
		if err := target.markWatched(ctx, userID, mk.Target); err != nil {
			run.Failed++
			item.Error = err.Error()
		} else {
			done++
		}
		if len(run.Items) < maxRunItems {
			run.Items = append(run.Items, item)
		}
	}
	return done
}
