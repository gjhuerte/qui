// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type httpError struct {
	Status int
	Body   string
}

func (e *httpError) Error() string {
	switch e.Status {
	case http.StatusUnauthorized, http.StatusForbidden:
		if e.Body != "" {
			return fmt.Sprintf("HTTP %d: check the sign-in (%s)", e.Status, e.Body)
		}
		return fmt.Sprintf("HTTP %d: check the sign-in", e.Status)
	default:
		if e.Body != "" {
			return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body)
		}
		return fmt.Sprintf("HTTP %d", e.Status)
	}
}

// request is one HTTP call. body, when set, is sent as JSON; form, when set,
// as a URL-encoded form.
type request struct {
	method string
	url    string
	header http.Header
	body   any
	form   url.Values
}

// send runs the request and decodes a JSON answer into out (nil discards it).
func send(ctx context.Context, hc *http.Client, r request, out any) error {
	var reader io.Reader
	header := http.Header{}
	for k, v := range r.header {
		header[k] = v
	}
	switch {
	case r.body != nil:
		data, err := json.Marshal(r.body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
		header.Set("Content-Type", "application/json")
	case r.form != nil:
		reader = strings.NewReader(r.form.Encode())
		header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	req, err := http.NewRequestWithContext(ctx, r.method, r.url, reader)
	if err != nil {
		return err
	}
	req.Header = header
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "qui")
	}

	resp, err := hc.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return urlErr.Err
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return &httpError{Status: resp.StatusCode, Body: errorText(body)}
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<20)).Decode(out); err != nil {
		return errors.New("the service did not answer as expected: is the URL right?")
	}
	return nil
}

// errorText pulls a message out of an error body: a JSON "message" or
// "error_description" field, or the first line of plain text.
func errorText(body []byte) string {
	var parsed struct {
		Message          string `json:"message"`
		ErrorDescription string `json:"error_description"`
		Error            any    `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		switch e := parsed.Error.(type) {
		case map[string]any:
			if m, ok := e["message"].(string); ok && m != "" {
				return m
			}
		case string:
			if parsed.ErrorDescription != "" {
				return e + ": " + parsed.ErrorDescription
			}
			return e
		}
		if parsed.Message != "" {
			return parsed.Message
		}
	}
	s := strings.TrimSpace(string(body))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if strings.HasPrefix(s, "<") || strings.HasPrefix(s, "{") {
		return ""
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// chunks splits ids into slices of at most n, for APIs that cap a batch.
func chunks[T any](items []T, n int) [][]T {
	var out [][]T
	for len(items) > n {
		out = append(out, items[:n])
		items = items[n:]
	}
	if len(items) > 0 {
		out = append(out, items)
	}
	return out
}
