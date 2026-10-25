// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// Package seedbox keeps the MyAnonamouse dynamic seedbox IP up to date. It does
// what the myanonamouse/seedboxapi container does: call dynamicSeedbox.php with
// the mam_id cookie every interval, keep the cookie MAM rotates, and record
// each outcome. When qui runs inside gluetun's network, MAM sees the VPN's IP.
package seedbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/gjhuerte/qui/internal/models"
)

// Contact sources.
const (
	SourceSchedule = "schedule"
	SourceManual   = "manual"
	SourceCookie   = "cookie"
)

const (
	tickInterval   = 30 * time.Second
	dueSlack       = 10 * time.Second
	historyDays    = 14
	contactsListed = 50
)

// Store is the persistence the service needs (models.SeedboxStore).
type Store interface {
	GetSettings(ctx context.Context) (*models.SeedboxSettings, error)
	SetMamID(ctx context.Context, mamID string) error
	UpdateSchedule(ctx context.Context, enabled bool, intervalSeconds int) error
	AddContact(ctx context.Context, c *models.SeedboxContact) (*models.SeedboxContact, error)
	ListContacts(ctx context.Context, limit int) ([]models.SeedboxContact, error)
	PruneContacts(ctx context.Context, before time.Time) (int64, error)
}

type Service struct {
	store  Store
	client *client
	logger zerolog.Logger

	// mu serialises contacts: the scheduler, "Update now" and saving a mam_id
	// must not race over the rotated cookie.
	mu sync.Mutex
}

func NewService(store Store) *Service {
	return &Service{
		store:  store,
		client: newClient(),
		logger: log.Logger.With().Str("module", "seedbox").Logger(),
	}
}

// Status is what the Seedbox page shows.
type Status struct {
	MamID struct {
		Configured bool   `json:"configured"`
		Masked     string `json:"masked,omitempty"`
	} `json:"mamId"`
	Enabled         bool                    `json:"enabled"`
	IntervalSeconds int                     `json:"intervalSeconds"`
	NextContactAt   *time.Time              `json:"nextContactAt,omitempty"`
	Latest          *models.SeedboxContact  `json:"latest,omitempty"`
	Contacts        []models.SeedboxContact `json:"contacts"`
}

// Start runs the scheduler until ctx ends.
func (s *Service) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		lastPrune := time.Time{}

		for {
			if err := s.runIfDue(ctx); err != nil && !errors.Is(err, context.Canceled) {
				s.logger.Warn().Err(err).Msg("seedbox: scheduled update failed")
			}
			if time.Since(lastPrune) > 24*time.Hour {
				lastPrune = time.Now()
				if _, err := s.store.PruneContacts(ctx, time.Now().AddDate(0, 0, -historyDays)); err != nil && ctx.Err() == nil {
					s.logger.Warn().Err(err).Msg("seedbox: failed to prune history")
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

func (s *Service) runIfDue(ctx context.Context) error {
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return nil
	}
	last, err := s.lastContactAt(ctx)
	if err != nil {
		return err
	}
	if last != nil && time.Since(*last) < time.Duration(settings.IntervalSeconds)*time.Second-dueSlack {
		return nil
	}
	_, err = s.Sync(ctx, SourceSchedule)
	return err
}

// Sync contacts MAM once and records the outcome. Without a mam_id it only
// looks up the IP MAM sees, so the page can show it before the first session.
func (s *Service) Sync(ctx context.Context, source string) (*models.SeedboxContact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	contact := &models.SeedboxContact{Source: source}

	if settings.MamID == "" {
		host, err := s.client.hostInfo(ctx)
		if err != nil {
			return s.recordFailure(ctx, contact, err)
		}
		s.logger.Info().Str("ip", host.IP).Msg("seedbox: no mam_id set yet; add one on the Services page")
		contact.Reached = true
		contact.HTTPStatus = &host.HTTPStatus
		contact.Message = "No mam_id set"
		contact.IP, contact.ASN, contact.ASOrg = host.IP, host.ASN, host.ASOrg
		return s.store.AddContact(ctx, contact)
	}

	res, err := s.client.updateSeedboxIP(ctx, settings.MamID)
	if err != nil {
		return s.recordFailure(ctx, contact, err)
	}

	if res.RotatedCookie != "" && res.RotatedCookie != settings.MamID {
		if err := s.store.SetMamID(ctx, res.RotatedCookie); err != nil {
			s.logger.Error().Err(err).Msg("seedbox: failed to save the rotated mam_id")
		}
	}

	contact.Reached = true
	contact.Success = res.Success
	contact.HTTPStatus = &res.HTTPStatus
	contact.Message = res.Message
	contact.IP, contact.ASN, contact.ASOrg = res.IP, res.ASN, res.ASOrg

	saved, err := s.store.AddContact(ctx, contact)
	if err != nil {
		return nil, err
	}

	event := s.logger.Info()
	switch saved.State {
	case models.SeedboxStateUnchanged:
		event = s.logger.Debug()
	case models.SeedboxStateRejected, models.SeedboxStateThrottled:
		event = s.logger.Warn()
	}
	event.Int("status", res.HTTPStatus).Str("ip", res.IP).Msgf("seedbox: MAM said %q", res.Message)

	return saved, nil
}

func (s *Service) recordFailure(ctx context.Context, contact *models.SeedboxContact, cause error) (*models.SeedboxContact, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if !errors.Is(cause, errUnreachable) {
		return nil, cause
	}
	message := strings.TrimPrefix(cause.Error(), errUnreachable.Error()+": ")
	s.logger.Warn().Msg("seedbox: " + message)
	contact.Reached = false
	f := false
	contact.Success = &f
	contact.Message = truncate(message, maxMessageLength)
	return s.store.AddContact(ctx, contact)
}

// SetMamID saves a new cookie and contacts MAM with it right away.
func (s *Service) SetMamID(ctx context.Context, value string) (*models.SeedboxContact, error) {
	mamID := CleanMamID(value)
	if mamID == "" || strings.ContainsAny(mamID, " \t\r\n") {
		return nil, ErrInvalidMamID
	}
	if len(mamID) > 2048 {
		return nil, ErrInvalidMamID
	}

	s.mu.Lock()
	err := s.store.SetMamID(ctx, mamID)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return s.Sync(ctx, SourceCookie)
}

// ClearMamID removes the cookie. Updates pause until a new one is added.
func (s *Service) ClearMamID(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.SetMamID(ctx, "")
}

// UpdateSchedule turns automatic updates on or off and sets their interval.
func (s *Service) UpdateSchedule(ctx context.Context, enabled bool, intervalSeconds int) error {
	if intervalSeconds < models.SeedboxMinInterval || intervalSeconds > models.SeedboxMaxInterval {
		return fmt.Errorf("%w: between %d and %d seconds", ErrInvalidInterval, models.SeedboxMinInterval, models.SeedboxMaxInterval)
	}
	return s.store.UpdateSchedule(ctx, enabled, intervalSeconds)
}

// Status returns the settings and the recent history.
func (s *Service) Status(ctx context.Context) (*Status, error) {
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	contacts, err := s.store.ListContacts(ctx, contactsListed)
	if err != nil {
		return nil, err
	}

	status := &Status{
		Enabled:         settings.Enabled,
		IntervalSeconds: settings.IntervalSeconds,
		Contacts:        contacts,
	}
	if settings.MamID != "" {
		status.MamID.Configured = true
		status.MamID.Masked = maskMamID(settings.MamID)
	}
	if len(contacts) > 0 {
		status.Latest = &contacts[0]
	}
	if settings.Enabled {
		next := time.Now()
		if status.Latest != nil {
			next = status.Latest.CreatedAt.Add(time.Duration(settings.IntervalSeconds) * time.Second)
		}
		// The scheduler ticks every 30 seconds.
		next = next.Add(-dueSlack).Truncate(tickInterval).Add(tickInterval)
		status.NextContactAt = &next
	}
	return status, nil
}

func (s *Service) lastContactAt(ctx context.Context) (*time.Time, error) {
	contacts, err := s.store.ListContacts(ctx, 1)
	if err != nil || len(contacts) == 0 {
		return nil, err
	}
	return &contacts[0].CreatedAt, nil
}

// Validation errors the handler maps to 400.
var (
	ErrInvalidMamID    = errors.New("paste only the mam_id value, without spaces")
	ErrInvalidInterval = errors.New("interval out of range")
)

// maskMamID keeps the last four characters, enough to recognise the cookie.
func maskMamID(value string) string {
	if len(value) <= 4 {
		return "••••"
	}
	return "••••" + value[len(value)-4:]
}
