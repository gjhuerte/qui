// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package seedbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// MAM endpoints, the same ones the official myanonamouse/seedboxapi image calls.
const (
	defaultUpdateURL = "https://t.myanonamouse.net/json/dynamicSeedbox.php"
	defaultIPURL     = "https://t.myanonamouse.net/json/jsonIp.php"
	requestTimeout   = 15 * time.Second
	maxMessageLength = 250
)

// errUnreachable means MAM could not be reached or did not answer with its JSON.
var errUnreachable = errors.New("mam unreachable")

// result is what MAM reported. dynamicSeedbox.php returns the IP/ASN it sees on
// every answer (also on 403 and 429); jsonIp.php returns only those.
type result struct {
	HTTPStatus    int
	Success       *bool
	Message       string
	IP            string
	ASN           *int
	ASOrg         string
	RotatedCookie string
}

type client struct {
	http      *http.Client
	updateURL string
	ipURL     string
}

func newClient() *client {
	return &client{
		http:      &http.Client{Timeout: requestTimeout},
		updateURL: defaultUpdateURL,
		ipURL:     defaultIPURL,
	}
}

type mamAnswer struct {
	Success *bool           `json:"Success"`
	Msg     *string         `json:"msg"`
	IP      string          `json:"ip"`
	ASN     json.RawMessage `json:"ASN"`
	AS      string          `json:"AS"`
}

// updateSeedboxIP registers the IP this request leaves from as the session's
// seedbox IP. MAM answers 200 (Completed / No change), 429 (last change too
// recent) or 403 (session rejected), and may rotate the mam_id cookie.
func (c *client) updateSeedboxIP(ctx context.Context, mamID string) (*result, error) {
	resp, answer, err := c.get(ctx, c.updateURL, mamID)
	if err != nil {
		return nil, err
	}
	if answer.Success == nil && answer.Msg == nil {
		return nil, fmt.Errorf("%w: unexpected answer (HTTP %d)", errUnreachable, resp.StatusCode)
	}

	res := toResult(resp.StatusCode, answer)
	res.Success = answer.Success
	if res.Success == nil {
		f := false
		res.Success = &f
	}
	if answer.Msg != nil {
		res.Message = truncate(*answer.Msg, maxMessageLength)
	}
	res.RotatedCookie = rotatedCookie(resp)
	return res, nil
}

// hostInfo returns the IP/ASN MAM sees without touching a session.
func (c *client) hostInfo(ctx context.Context) (*result, error) {
	resp, answer, err := c.get(ctx, c.ipURL, "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%w: IP lookup failed (HTTP %d)", errUnreachable, resp.StatusCode)
	}
	return toResult(resp.StatusCode, answer), nil
}

func (c *client) get(ctx context.Context, target, mamID string) (*http.Response, *mamAnswer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "qui-seedbox")
	if mamID != "" {
		req.AddCookie(&http.Cookie{Name: "mam_id", Value: mamID})
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: could not reach MAM: %s", errUnreachable, err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s", errUnreachable, err.Error())
	}

	var answer mamAnswer
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, nil, fmt.Errorf("%w: MAM did not answer with JSON (HTTP %d)", errUnreachable, resp.StatusCode)
	}
	return resp, &answer, nil
}

func toResult(status int, answer *mamAnswer) *result {
	res := &result{HTTPStatus: status, IP: answer.IP, ASOrg: answer.AS}
	// MAM sends ASN as a number, older answers as a string.
	raw := strings.Trim(string(answer.ASN), `"`)
	var asn int
	if _, err := fmt.Sscanf(raw, "%d", &asn); err == nil {
		res.ASN = &asn
	}
	return res
}

func rotatedCookie(resp *http.Response) string {
	for _, cookie := range resp.Cookies() {
		if cookie.Name != "mam_id" {
			continue
		}
		value, err := url.QueryUnescape(cookie.Value)
		if err != nil {
			value = cookie.Value
		}
		value = strings.TrimSpace(value)
		if value == "" || value == "deleted" {
			return ""
		}
		return value
	}
	return ""
}

var mamIDPrefix = regexp.MustCompile(`(?i)^mam_id=`)

// CleanMamID accepts "mam_id=abc" or a quoted value as pasted from a browser.
func CleanMamID(value string) string {
	value = strings.Trim(value, " \t\n\r\x00\x0B\"';")
	return mamIDPrefix.ReplaceAllString(value, "")
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}
