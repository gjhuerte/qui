// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package gluetun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gjhuerte/qui/internal/models"
)

// controlClient talks to gluetun's HTTP control server (port 8000 by default).
type controlClient struct {
	baseURL  string
	authType string
	username string
	secret   string
	http     *http.Client
}

func newControlClient(settings *models.GluetunSettings) *controlClient {
	return &controlClient{
		baseURL:  strings.TrimRight(settings.ControlURL, "/"),
		authType: settings.AuthType,
		username: settings.Username,
		secret:   settings.Secret,
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

// errNotFound is a 404 from the control server: the route does not exist in
// this gluetun version.
var errNotFound = errors.New("route not found")

func (c *controlClient) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch c.authType {
	case models.GluetunAuthAPIKey:
		req.Header.Set("X-API-Key", c.secret)
	case models.GluetunAuthBasic:
		req.SetBasicAuth(c.username, c.secret)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return errNotFound
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%s %s: HTTP %d, check the control server authentication", method, path, resp.StatusCode)
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}

	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if raw, ok := out.(*json.RawMessage); ok {
		*raw = append((*raw)[:0], data...)
		return nil
	}
	return json.Unmarshal(data, out)
}

func (c *controlClient) status(ctx context.Context, path string) (string, error) {
	var out struct {
		Status string `json:"status"`
	}
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out.Status, err
}

func (c *controlClient) setVPNStatus(ctx context.Context, status string) error {
	return c.do(ctx, http.MethodPut, "/v1/vpn/status", map[string]string{"status": status}, nil)
}

// PublicIP is what gluetun knows about the IP the VPN exits from.
type PublicIP struct {
	IP           string `json:"ip"`
	Country      string `json:"country,omitempty"`
	Region       string `json:"region,omitempty"`
	City         string `json:"city,omitempty"`
	Location     string `json:"location,omitempty"`
	Organization string `json:"organization,omitempty"`
	PostalCode   string `json:"postalCode,omitempty"`
	Timezone     string `json:"timezone,omitempty"`
	Hostname     string `json:"hostname,omitempty"`
}

func (c *controlClient) publicIP(ctx context.Context) (*PublicIP, error) {
	var out struct {
		PublicIP     string `json:"public_ip"`
		Country      string `json:"country"`
		Region       string `json:"region"`
		City         string `json:"city"`
		Location     string `json:"location"`
		Organization string `json:"organization"`
		PostalCode   string `json:"postal_code"`
		Timezone     string `json:"timezone"`
		Hostname     string `json:"hostname"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/publicip/ip", nil, &out); err != nil {
		return nil, err
	}
	return &PublicIP{
		IP: out.PublicIP, Country: out.Country, Region: out.Region, City: out.City,
		Location: out.Location, Organization: out.Organization, PostalCode: out.PostalCode,
		Timezone: out.Timezone, Hostname: out.Hostname,
	}, nil
}

// forwardedPorts returns the ports forwarded by the VPN provider. Newer gluetun
// serves /v1/portforward ({"port":N} or {"ports":[...]}); older versions only
// /v1/openvpn/portforwarded.
func (c *controlClient) forwardedPorts(ctx context.Context) ([]int, error) {
	var out struct {
		Port  int   `json:"port"`
		Ports []int `json:"ports"`
	}
	err := c.do(ctx, http.MethodGet, "/v1/portforward", nil, &out)
	if errors.Is(err, errNotFound) {
		err = c.do(ctx, http.MethodGet, "/v1/openvpn/portforwarded", nil, &out)
	}
	if err != nil {
		return nil, err
	}
	ports := out.Ports
	if len(ports) == 0 && out.Port != 0 {
		ports = []int{out.Port}
	}
	return ports, nil
}

func (c *controlClient) vpnSettings(ctx context.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/v1/vpn/settings", nil, &raw); err != nil {
		return nil, err
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode VPN settings: %w", err)
	}
	return json.Marshal(redactValue("", decoded))
}

// redactValue hides keys, passwords and other secrets in the VPN settings.
func redactValue(key string, value any) any {
	switch v := value.(type) {
	case map[string]any:
		for k, inner := range v {
			v[k] = redactValue(k, inner)
		}
		return v
	case []any:
		for i, inner := range v {
			v[i] = redactValue(key, inner)
		}
		return v
	case string:
		if v != "" && isSecretName(key) {
			return redacted
		}
		return v
	default:
		return v
	}
}

const redacted = "[redacted]"

func isSecretName(name string) bool {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "public") {
		return false
	}
	for _, marker := range []string{"key", "password", "passwd", "secret", "token", "private", "psk", "credential"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
