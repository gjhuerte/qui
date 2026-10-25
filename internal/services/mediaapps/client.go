// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package mediaapps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gjhuerte/qui/internal/models"
)

// client calls one app's API. Sonarr, Radarr, Prowlarr and Bazarr take the
// key in the X-Api-Key header; Tautulli only takes it as a query parameter.
type client struct {
	inst *models.AppInstance
	http *http.Client
}

var errNotFound = errors.New("not found")

type httpError struct {
	Status int
	Body   string
}

func (e *httpError) Error() string {
	switch e.Status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Sprintf("HTTP %d: check the API key", e.Status)
	default:
		if e.Body != "" {
			return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body)
		}
		return fmt.Sprintf("HTTP %d", e.Status)
	}
}

func (c *client) get(ctx context.Context, path string, query url.Values, out any) error {
	base := strings.TrimRight(c.inst.BaseURL, "/")
	if query == nil {
		query = url.Values{}
	}
	if c.inst.Type == models.AppTypeTautulli || c.inst.Type == models.AppTypeBazarr {
		query.Set("apikey", c.inst.APIKey)
	}
	target := base + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "qui")
	if c.inst.Type != models.AppTypeTautulli {
		req.Header.Set("X-Api-Key", c.inst.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return cleanURLError(err, c.inst.APIKey)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &httpError{Status: resp.StatusCode, Body: firstLine(string(body))}
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(out); err != nil {
		return fmt.Errorf("%s did not answer with JSON: is the URL right?", c.inst.Type)
	}
	return nil
}

// cleanURLError keeps the API key out of error messages: net/http includes the
// full URL, and Tautulli's key is in the query.
func cleanURLError(err error, key string) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	message := err.Error()
	if key != "" {
		message = strings.ReplaceAll(message, key, "***")
	}
	return errors.New(message)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if strings.HasPrefix(s, "<") {
		return ""
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
