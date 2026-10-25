// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package gluetun

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// dockerClient is a small client for the Docker Engine API: inspect, logs,
// stats and restart are all qui needs, so it does not pull in the Docker SDK.
type dockerClient struct {
	baseURL string
	http    *http.Client
}

// newDockerClient accepts unix:///path/docker.sock, tcp://host:port or
// http(s)://host:port (for a socket proxy).
func newDockerClient(host string) (*dockerClient, error) {
	host = strings.TrimSpace(host)
	parsed, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("invalid Docker host %q: %w", host, err)
	}

	switch parsed.Scheme {
	case "unix":
		socket := parsed.Path
		if socket == "" {
			return nil, fmt.Errorf("invalid Docker host %q: missing socket path", host)
		}
		dialer := &net.Dialer{Timeout: 5 * time.Second}
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, "unix", socket)
			},
		}
		return &dockerClient{baseURL: "http://docker", http: &http.Client{Transport: transport}}, nil
	case "tcp", "http":
		return &dockerClient{baseURL: "http://" + parsed.Host, http: &http.Client{}}, nil
	case "https":
		return &dockerClient{baseURL: "https://" + parsed.Host, http: &http.Client{}}, nil
	default:
		return nil, fmt.Errorf("invalid Docker host %q: use unix://, tcp:// or http://", host)
	}
}

type dockerError struct {
	Status  int
	Message string
}

func (e *dockerError) Error() string {
	return fmt.Sprintf("docker: HTTP %d: %s", e.Status, e.Message)
}

func (d *dockerClient) request(ctx context.Context, method, path string, query url.Values) (*http.Response, error) {
	target := d.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var body struct {
			Message string `json:"message"`
		}
		message := strings.TrimSpace(string(data))
		if json.Unmarshal(data, &body) == nil && body.Message != "" {
			message = body.Message
		}
		return nil, &dockerError{Status: resp.StatusCode, Message: message}
	}
	return resp, nil
}

func (d *dockerClient) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	resp, err := d.request(ctx, http.MethodGet, path, query)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

// containerJSON is the part of GET /containers/{id}/json qui reads.
type containerJSON struct {
	ID           string `json:"Id"`
	Name         string `json:"Name"`
	Created      string `json:"Created"`
	Image        string `json:"Image"`
	RestartCount int    `json:"RestartCount"`
	State        struct {
		Status     string `json:"Status"`
		Running    bool   `json:"Running"`
		Restarting bool   `json:"Restarting"`
		ExitCode   int    `json:"ExitCode"`
		Error      string `json:"Error"`
		StartedAt  string `json:"StartedAt"`
		FinishedAt string `json:"FinishedAt"`
		Health     *struct {
			Status        string `json:"Status"`
			FailingStreak int    `json:"FailingStreak"`
			Log           []struct {
				Start    string `json:"Start"`
				End      string `json:"End"`
				ExitCode int    `json:"ExitCode"`
				Output   string `json:"Output"`
			} `json:"Log"`
		} `json:"Health"`
	} `json:"State"`
	Config struct {
		Image string   `json:"Image"`
		Env   []string `json:"Env"`
		Tty   bool     `json:"Tty"`
	} `json:"Config"`
	HostConfig struct {
		Memory        int64  `json:"Memory"`
		NanoCpus      int64  `json:"NanoCpus"`
		NetworkMode   string `json:"NetworkMode"`
		RestartPolicy struct {
			Name string `json:"Name"`
		} `json:"RestartPolicy"`
		CapAdd []string `json:"CapAdd"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
	} `json:"NetworkSettings"`
}

func (d *dockerClient) inspect(ctx context.Context, name string) (*containerJSON, error) {
	var out containerJSON
	if err := d.getJSON(ctx, "/containers/"+url.PathEscape(name)+"/json", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// imageEnv returns the environment the image sets, so the page can show only
// the variables the container overrides.
func (d *dockerClient) imageEnv(ctx context.Context, image string) ([]string, error) {
	var out struct {
		Config struct {
			Env []string `json:"Env"`
		} `json:"Config"`
	}
	if err := d.getJSON(ctx, "/images/"+url.PathEscape(image)+"/json", nil, &out); err != nil {
		return nil, err
	}
	return out.Config.Env, nil
}

type containerSummary struct {
	ID         string   `json:"Id"`
	Names      []string `json:"Names"`
	Image      string   `json:"Image"`
	State      string   `json:"State"`
	Status     string   `json:"Status"`
	HostConfig struct {
		NetworkMode string `json:"NetworkMode"`
	} `json:"HostConfig"`
}

func (d *dockerClient) list(ctx context.Context) ([]containerSummary, error) {
	var out []containerSummary
	err := d.getJSON(ctx, "/containers/json", url.Values{"all": {"1"}}, &out)
	return out, err
}

type statsJSON struct {
	CPUStats    cpuStats `json:"cpu_stats"`
	PreCPUStats cpuStats `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
	PidsStats struct {
		Current uint64 `json:"current"`
	} `json:"pids_stats"`
}

type cpuStats struct {
	CPUUsage struct {
		TotalUsage uint64 `json:"total_usage"`
	} `json:"cpu_usage"`
	SystemCPUUsage uint64 `json:"system_cpu_usage"`
	OnlineCPUs     uint32 `json:"online_cpus"`
}

func (d *dockerClient) stats(ctx context.Context, id string) (*statsJSON, error) {
	var out statsJSON
	err := d.getJSON(ctx, "/containers/"+url.PathEscape(id)+"/stats", url.Values{"stream": {"false"}}, &out)
	return &out, err
}

func (d *dockerClient) restart(ctx context.Context, id string, timeout time.Duration) error {
	resp, err := d.request(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/restart",
		url.Values{"t": {strconv.Itoa(int(timeout.Seconds()))}})
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// logs returns the last tail lines of stdout and stderr with Docker's timestamps.
func (d *dockerClient) logs(ctx context.Context, id string, tty bool, tail int) ([]string, error) {
	resp, err := d.request(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/logs", url.Values{
		"stdout":     {"1"},
		"stderr":     {"1"},
		"timestamps": {"1"},
		"tail":       {strconv.Itoa(tail)},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data []byte
	if tty {
		data, err = io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	} else {
		data, err = demux(resp.Body)
	}
	if err != nil {
		return nil, err
	}

	lines := []string{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		if line := strings.TrimRight(scanner.Text(), "\r"); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

// demux reads Docker's multiplexed log stream: frames of an 8-byte header
// (stream type, 3 zero bytes, big-endian length) followed by the payload.
func demux(r io.Reader) ([]byte, error) {
	var out bytes.Buffer
	header := make([]byte, 8)
	for {
		if _, err := io.ReadFull(r, header); err != nil {
			if errors.Is(err, io.EOF) {
				return out.Bytes(), nil
			}
			return out.Bytes(), err
		}
		size := binary.BigEndian.Uint32(header[4:])
		if _, err := io.CopyN(&out, r, int64(size)); err != nil {
			return out.Bytes(), err
		}
		if out.Len() > 16<<20 {
			return out.Bytes(), nil
		}
	}
}
