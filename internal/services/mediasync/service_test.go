// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package mediasync

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/models"
)

// fakeBackend serves fixed libraries and records what it marked.
type fakeBackend struct {
	mu      sync.Mutex
	libs    map[string][]Item
	marked  []string
	failIDs map[string]bool
}

func (f *fakeBackend) status(context.Context) (*ServerStatus, error) {
	return &ServerStatus{Online: true}, nil
}
func (f *fakeBackend) users(context.Context) ([]User, error) { return nil, nil }
func (f *fakeBackend) library(_ context.Context, userID string) ([]Item, error) {
	return f.libs[userID], nil
}
func (f *fakeBackend) markWatched(_ context.Context, userID string, item Item) error {
	if f.failIDs[item.ID] {
		return errors.New("HTTP 500")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.marked = append(f.marked, userID+"/"+item.ID)
	return nil
}

func setupPair(t *testing.T, direction models.SyncDirection, a, b *fakeBackend) (*Service, *models.MediaSyncStore, *models.SyncPair) {
	t.Helper()
	store := testStore(t)
	ctx := t.Context()
	srvA, err := store.CreateServer(ctx, &models.MediaServer{Type: models.MediaServerJellyfin, Name: "Jellyfin", BaseURL: "http://a", Token: "a", Enabled: true})
	require.NoError(t, err)
	srvB, err := store.CreateServer(ctx, &models.MediaServer{Type: models.MediaServerPlex, Name: "Plex", BaseURL: "http://b", Token: "b", Enabled: true})
	require.NoError(t, err)

	svc := NewService(store)
	svc.newBackend = func(srv *models.MediaServer, _ time.Duration) backend {
		if srv.ID == srvA.ID {
			return a
		}
		return b
	}
	pair, err := svc.CreatePair(ctx, PairInput{ServerAID: srvA.ID, ServerBID: srvB.ID, Direction: direction, Enabled: true, IntervalMinutes: 60})
	require.NoError(t, err)
	_, err = svc.SetUserMaps(ctx, pair.ID, []models.SyncUserMap{{UserAID: "ua", UserAName: "Ann", UserBID: "ub", UserBName: "ann_plex"}})
	require.NoError(t, err)
	return svc, store, pair
}

func runPair(t *testing.T, svc *Service, pair *models.SyncPair) *models.SyncRun {
	t.Helper()
	run, runCtx, err := svc.begin(t.Context(), t.Context(), pair.ID, models.SyncSourceManual)
	require.NoError(t, err)
	svc.execute(runCtx, pair, run)
	stored, err := svc.GetRun(t.Context(), run.ID)
	require.NoError(t, err)
	return stored
}

func TestRunSyncsBothWays(t *testing.T) {
	a := &fakeBackend{libs: map[string][]Item{"ua": {
		{ID: "a-movie", Kind: kindMovie, Title: "Example Movie", IDs: []string{"imdb:tt1"}, Watched: true},
		{ID: "a-only", Kind: kindMovie, Title: "Only On A", IDs: []string{"imdb:tt2"}, Watched: true},
		{ID: "a-ep", Kind: kindEpisode, Series: "Show", SeriesIDs: []string{"tvdb:100"}, Season: 1, Episode: 2},
	}}}
	b := &fakeBackend{libs: map[string][]Item{"ub": {
		{ID: "b-movie", Kind: kindMovie, Title: "Example Movie", IDs: []string{"tmdb:9", "imdb:tt1"}},
		{ID: "b-ep", Kind: kindEpisode, Series: "Show", SeriesIDs: []string{"tvdb:100"}, Season: 1, Episode: 2, Watched: true},
	}}}

	svc, _, pair := setupPair(t, models.SyncBoth, a, b)
	run := runPair(t, svc, pair)

	require.Equal(t, models.SyncRunSuccess, run.Status, run.Message)
	require.Equal(t, []string{"ub/b-movie"}, b.marked)
	require.Equal(t, []string{"ua/a-ep"}, a.marked)
	require.Equal(t, 1, run.MarkedA)
	require.Equal(t, 1, run.MarkedB)
	require.Equal(t, 1, run.NotFound)
	require.Len(t, run.Items, 2)
	require.Empty(t, svc.running, "the pair is released after the run")
}

func TestRunOneWayAndFailures(t *testing.T) {
	a := &fakeBackend{libs: map[string][]Item{"ua": {
		{ID: "a1", Kind: kindMovie, IDs: []string{"imdb:tt1"}, Watched: true},
		{ID: "a2", Kind: kindMovie, IDs: []string{"imdb:tt2"}, Watched: true},
		{ID: "a3", Kind: kindMovie, IDs: []string{"imdb:tt3"}},
	}}}
	b := &fakeBackend{
		libs: map[string][]Item{"ub": {
			{ID: "b1", Kind: kindMovie, IDs: []string{"imdb:tt1"}},
			{ID: "b2", Kind: kindMovie, IDs: []string{"imdb:tt2"}},
			{ID: "b3", Kind: kindMovie, IDs: []string{"imdb:tt3"}, Watched: true},
		}},
		failIDs: map[string]bool{"b2": true},
	}

	svc, _, pair := setupPair(t, models.SyncAToB, a, b)
	run := runPair(t, svc, pair)

	require.Equal(t, models.SyncRunPartial, run.Status)
	require.Equal(t, []string{"ub/b1"}, b.marked)
	require.Empty(t, a.marked, "a one-way pair never writes to server A")
	require.Equal(t, 1, run.MarkedB)
	require.Equal(t, 1, run.Failed)
	require.Equal(t, "HTTP 500", run.Items[1].Error)
}

func TestRunWithoutMapsFails(t *testing.T) {
	svc, _, pair := setupPair(t, models.SyncBoth, &fakeBackend{}, &fakeBackend{})
	_, err := svc.SetUserMaps(t.Context(), pair.ID, nil)
	require.NoError(t, err)

	run := runPair(t, svc, pair)
	require.Equal(t, models.SyncRunFailed, run.Status)
	require.Contains(t, run.Message, "no accounts are mapped")
}

func TestRunNowRefusesASecondRun(t *testing.T) {
	svc, _, pair := setupPair(t, models.SyncBoth, &fakeBackend{}, &fakeBackend{})
	_, _, err := svc.begin(t.Context(), t.Context(), pair.ID, models.SyncSourceSchedule)
	require.NoError(t, err)
	_, err = svc.RunNow(t.Context(), pair.ID)
	require.ErrorIs(t, err, ErrRunning)
}

// blockingBackend waits in library until the run is cancelled.
type blockingBackend struct {
	fakeBackend
	started chan struct{}
}

func (b *blockingBackend) library(ctx context.Context, _ string) ([]Item, error) {
	close(b.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestCancelStopsTheRun(t *testing.T) {
	a := &blockingBackend{started: make(chan struct{})}
	svc, store, pair := setupPair(t, models.SyncBoth, nil, &fakeBackend{})
	svc.newBackend = func(*models.MediaServer, time.Duration) backend { return a }

	require.ErrorIs(t, svc.Cancel(pair.ID), ErrInvalid, "nothing to cancel yet")
	run, err := svc.RunNow(t.Context(), pair.ID)
	require.NoError(t, err)
	<-a.started
	require.NoError(t, svc.Cancel(pair.ID))

	require.Eventually(t, func() bool {
		got, err := store.GetRun(t.Context(), run.ID)
		return err == nil && got.Status != models.SyncRunRunning
	}, 5*time.Second, 10*time.Millisecond)
	got, err := store.GetRun(t.Context(), run.ID)
	require.NoError(t, err)
	require.Equal(t, models.SyncRunFailed, got.Status)
	require.Contains(t, got.Message, "stopped before the end")
}

func TestRunDueRespectsInterval(t *testing.T) {
	a := &fakeBackend{libs: map[string][]Item{"ua": {{ID: "a1", Kind: kindMovie, IDs: []string{"imdb:tt1"}, Watched: true}}}}
	b := &fakeBackend{libs: map[string][]Item{"ub": {{ID: "b1", Kind: kindMovie, IDs: []string{"imdb:tt1"}}}}}
	svc, store, pair := setupPair(t, models.SyncBoth, a, b)

	svc.runDue(t.Context())
	svc.runDue(t.Context())
	runs, err := store.ListRuns(t.Context(), pair.ID, 10)
	require.NoError(t, err)
	require.Len(t, runs, 1, "the second tick is inside the interval")
	require.Equal(t, models.SyncSourceSchedule, runs[0].Source)
}

func TestPairValidation(t *testing.T) {
	svc, _, pair := setupPair(t, models.SyncBoth, &fakeBackend{}, &fakeBackend{})
	ctx := t.Context()

	_, err := svc.CreatePair(ctx, PairInput{ServerAID: pair.ServerAID, ServerBID: pair.ServerAID, Direction: models.SyncBoth, IntervalMinutes: 60})
	require.ErrorIs(t, err, ErrInvalid)
	_, err = svc.CreatePair(ctx, PairInput{ServerAID: pair.ServerAID, ServerBID: 999, Direction: models.SyncBoth, IntervalMinutes: 60})
	require.ErrorIs(t, err, ErrInvalid)
	_, err = svc.UpdatePair(ctx, pair.ID, PairInput{Direction: "sideways", IntervalMinutes: 60})
	require.ErrorIs(t, err, ErrInvalid)
	_, err = svc.UpdatePair(ctx, pair.ID, PairInput{Direction: models.SyncBToA, IntervalMinutes: 1})
	require.ErrorIs(t, err, ErrInvalid)

	updated, err := svc.UpdatePair(ctx, pair.ID, PairInput{Direction: models.SyncBToA, IntervalMinutes: 15})
	require.NoError(t, err)
	require.Equal(t, models.SyncBToA, updated.Direction)
	require.Equal(t, pair.ServerAID, updated.ServerAID)
}
