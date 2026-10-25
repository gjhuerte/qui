// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package mediasync

import (
	"context"
	"encoding/json"
	"encoding/xml"
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
		return fmt.Sprintf("HTTP %d: check the token", e.Status)
	default:
		if e.Body != "" {
			return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body)
		}
		return fmt.Sprintf("HTTP %d", e.Status)
	}
}

// do sends one request. out may be nil; xmlBody decodes XML instead of JSON
// (plex.tv's shared_servers answers only in XML).
func do(ctx context.Context, hc *http.Client, method, target string, header http.Header, out any, xmlBody bool) error {
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	if xmlBody {
		req.Header.Set("Accept", "application/xml")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	req.Header.Set("User-Agent", "qui")

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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &httpError{Status: resp.StatusCode, Body: firstLine(string(body))}
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil
	}
	body := io.LimitReader(resp.Body, 256<<20)
	if xmlBody {
		err = xml.NewDecoder(body).Decode(out)
	} else {
		err = json.NewDecoder(body).Decode(out)
	}
	if err != nil {
		return errors.New("the server did not answer as expected: is the URL right?")
	}
	return nil
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
