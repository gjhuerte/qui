// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// Package mediaapps tracks Sonarr, Radarr, Prowlarr, Bazarr and Tautulli
// instances for the Services page: whether each one is up, its health
// warnings, and what it is doing (downloads, missing items, wanted subtitles,
// indexers, streams). It only reads from the apps.
package mediaapps

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gjhuerte/qui/internal/models"
)

// Store is the persistence the service needs (models.AppInstanceStore).
type Store interface {
	List(ctx context.Context) ([]*models.AppInstance, error)
	Get(ctx context.Context, id int) (*models.AppInstance, error)
	Create(ctx context.Context, inst *models.AppInstance) (*models.AppInstance, error)
	Update(ctx context.Context, inst *models.AppInstance) (*models.AppInstance, error)
	Delete(ctx context.Context, id int) error
}

const (
	requestTimeout = 10 * time.Second
	// Several browser tabs, or a dashboard left open, share one round of
	// requests to the apps.
	cacheTTL = 20 * time.Second
)

// ErrInvalid is a validation error the handler maps to 400.
var ErrInvalid = errors.New("invalid app instance")

type Service struct {
	store Store
	http  *http.Client
	// libraryHTTP allows for full movie and series lists from slow servers.
	libraryHTTP *http.Client
	library     libraryCache

	mu       sync.Mutex
	cached   []*Status
	cachedAt time.Time
}

func NewService(store Store) *Service {
	return &Service{
		store:       store,
		http:        &http.Client{Timeout: requestTimeout},
		libraryHTTP: &http.Client{Timeout: 3 * requestTimeout},
	}
}

// Input is the editable part of an instance. An empty APIKey on update keeps
// the stored key.
type Input struct {
	Type    models.AppType `json:"type"`
	Name    string         `json:"name"`
	BaseURL string         `json:"baseUrl"`
	APIKey  string         `json:"apiKey"`
	Enabled *bool          `json:"enabled"`
}

func (in *Input) normalize() error {
	in.Name = strings.TrimSpace(in.Name)
	in.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	in.APIKey = strings.TrimSpace(in.APIKey)

	if !in.Type.Valid() {
		return fmt.Errorf("%w: unknown type %q", ErrInvalid, in.Type)
	}
	if in.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalid)
	}
	parsed, err := url.Parse(in.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%w: the URL must look like http://192.168.1.10:8989", ErrInvalid)
	}
	return nil
}

func (s *Service) List(ctx context.Context) ([]*models.AppInstance, error) {
	return s.store.List(ctx)
}

func (s *Service) Create(ctx context.Context, in Input) (*models.AppInstance, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	if in.APIKey == "" {
		return nil, fmt.Errorf("%w: API key is required", ErrInvalid)
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	defer s.invalidate()
	return s.store.Create(ctx, &models.AppInstance{Type: in.Type, Name: in.Name, BaseURL: in.BaseURL, APIKey: in.APIKey, Enabled: enabled})
}

func (s *Service) Update(ctx context.Context, id int, in Input) (*models.AppInstance, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	current, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	current.Type, current.Name, current.BaseURL = in.Type, in.Name, in.BaseURL
	if in.APIKey != "" {
		current.APIKey = in.APIKey
	}
	if in.Enabled != nil {
		current.Enabled = *in.Enabled
	}
	defer s.invalidate()
	return s.store.Update(ctx, current)
}

func (s *Service) Delete(ctx context.Context, id int) error {
	defer s.invalidate()
	return s.store.Delete(ctx, id)
}

// Test checks a connection before it is saved. With id set and no API key,
// it uses the stored key, so editing the URL does not require the key again.
func (s *Service) Test(ctx context.Context, id int, in Input) (*Status, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	if in.APIKey == "" && id > 0 {
		current, err := s.store.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		in.APIKey = current.APIKey
	}
	if in.APIKey == "" {
		return nil, fmt.Errorf("%w: API key is required", ErrInvalid)
	}
	inst := &models.AppInstance{ID: id, Type: in.Type, Name: in.Name, BaseURL: in.BaseURL, APIKey: in.APIKey, Enabled: true}
	return fetch(ctx, &client{inst: inst, http: s.http}), nil
}

// Status returns the status of every enabled instance, queried in parallel.
// refresh skips the short cache.
func (s *Service) Status(ctx context.Context, refresh bool) ([]*Status, error) {
	s.mu.Lock()
	if !refresh && s.cached != nil && time.Since(s.cachedAt) < cacheTTL {
		cached := s.cached
		s.mu.Unlock()
		return cached, nil
	}
	s.mu.Unlock()

	instances, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}

	// The result is cached for other requests, so a closed browser tab must
	// not cut the round short.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*requestTimeout)
	defer cancel()

	out := make([]*Status, 0, len(instances))
	var wg sync.WaitGroup
	results := make([]*Status, len(instances))
	for i, inst := range instances {
		if !inst.Enabled {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = fetch(ctx, &client{inst: inst, http: s.http})
		}()
	}
	wg.Wait()
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
	s.library.clear()
}
