// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package seedbox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gjhuerte/qui/internal/models"
)

type memoryStore struct {
	mu       sync.Mutex
	settings models.SeedboxSettings
	contacts []models.SeedboxContact
}

func (m *memoryStore) GetSettings(context.Context) (*models.SeedboxSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.settings
	return &s, nil
}

func (m *memoryStore) SetMamID(_ context.Context, mamID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings.MamID = mamID
	return nil
}

func (m *memoryStore) UpdateSchedule(_ context.Context, enabled bool, interval int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings.Enabled, m.settings.IntervalSeconds = enabled, interval
	return nil
}

func (m *memoryStore) AddContact(_ context.Context, c *models.SeedboxContact) (*models.SeedboxContact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c.ID = int64(len(m.contacts) + 1)
	c.CreatedAt = time.Now()
	c.State = c.ComputeState()
	m.contacts = append([]models.SeedboxContact{*c}, m.contacts...)
	return c, nil
}

func (m *memoryStore) ListContacts(_ context.Context, limit int) ([]models.SeedboxContact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.contacts[:min(limit, len(m.contacts))], nil
}

func (m *memoryStore) PruneContacts(context.Context, time.Time) (int64, error) { return 0, nil }

func newTestService(t *testing.T, handler http.HandlerFunc) (*Service, *memoryStore) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	store := &memoryStore{settings: models.SeedboxSettings{Enabled: true, IntervalSeconds: 600}}
	svc := NewService(store)
	svc.client.updateURL = server.URL + "/update"
	svc.client.ipURL = server.URL + "/ip"
	return svc, store
}

func TestSyncWithoutMamIDLooksUpIP(t *testing.T) {
	svc, _ := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ip" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ip":"1.2.3.4","ASN":12345,"AS":"Example VPN"}`))
	})

	contact, err := svc.Sync(context.Background(), SourceManual)
	if err != nil {
		t.Fatal(err)
	}
	if contact.State != models.SeedboxStateWaiting || contact.IP != "1.2.3.4" || contact.ASN == nil || *contact.ASN != 12345 {
		t.Fatalf("unexpected contact %+v", contact)
	}
}

func TestSyncSavesRotatedCookie(t *testing.T) {
	svc, store := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("mam_id")
		if err != nil || cookie.Value != "old" {
			t.Errorf("cookie not sent: %v", err)
		}
		http.SetCookie(w, &http.Cookie{Name: "mam_id", Value: "new"})
		_, _ = w.Write([]byte(`{"Success":true,"msg":"Completed","ip":"5.6.7.8","ASN":"999","AS":"VPN"}`))
	})
	store.settings.MamID = "old"

	contact, err := svc.Sync(context.Background(), SourceManual)
	if err != nil {
		t.Fatal(err)
	}
	if contact.State != models.SeedboxStateUpdated || store.settings.MamID != "new" || contact.ASN == nil || *contact.ASN != 999 {
		t.Fatalf("contact %+v, stored %q", contact, store.settings.MamID)
	}
}

func TestSyncStates(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"unchanged", 200, `{"Success":true,"msg":"No change","ip":"1.1.1.1"}`, models.SeedboxStateUnchanged},
		{"throttled", 429, `{"Success":false,"msg":"Last Change too recent","ip":"1.1.1.1"}`, models.SeedboxStateThrottled},
		{"rejected", 403, `{"Success":false,"msg":"ASN mismatch","ip":"1.1.1.1"}`, models.SeedboxStateRejected},
		{"not json", 502, `<html>bad gateway</html>`, models.SeedboxStateUnreachable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, store := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			store.settings.MamID = "abc"
			contact, err := svc.Sync(context.Background(), SourceSchedule)
			if err != nil {
				t.Fatal(err)
			}
			if contact.State != tc.want {
				t.Fatalf("state %q, want %q (%+v)", contact.State, tc.want, contact)
			}
		})
	}
}

func TestRunIfDueHonoursInterval(t *testing.T) {
	calls := 0
	svc, _ := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"ip":"1.2.3.4"}`))
	})
	ctx := context.Background()
	for range 3 {
		if err := svc.runIfDue(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("contacted MAM %d times, want 1", calls)
	}
}

func TestCleanMamID(t *testing.T) {
	for in, want := range map[string]string{
		"abc":             "abc",
		" mam_id=abc; ":   "abc",
		`"MAM_ID=a+b/c="`: "a+b/c=",
	} {
		if got := CleanMamID(in); got != want {
			t.Errorf("CleanMamID(%q) = %q, want %q", in, got, want)
		}
	}
}
