// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package gluetun

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gjhuerte/qui/internal/models"
)

type memoryStore struct {
	settings *models.GluetunSettings
}

func (m *memoryStore) Get(context.Context) (*models.GluetunSettings, error) {
	s := *m.settings
	return &s, nil
}

func (m *memoryStore) Save(_ context.Context, s *models.GluetunSettings) error {
	m.settings = s
	return nil
}

func TestParseLogLine(t *testing.T) {
	cases := []struct {
		in   string
		want LogLine
	}{
		{
			"2026-10-24T10:00:00.123456789Z 2026-10-24T10:00:00Z INFO [wireguard] Connecting to 1.2.3.4:51820",
			LogLine{Time: "2026-10-24T10:00:00Z", Level: "info", Module: "wireguard", Message: "Connecting to 1.2.3.4:51820"},
		},
		{
			"2026-10-24T10:00:01.5Z 2026-10-24T10:00:01Z ERROR [vpn] \x1b[31mconnection failed\x1b[0m",
			LogLine{Time: "2026-10-24T10:00:01Z", Level: "error", Module: "vpn", Message: "connection failed"},
		},
		{
			"2026-10-24T10:00:02Z WARN no module here",
			LogLine{Time: "2026-10-24T10:00:02Z", Level: "warn", Message: "no module here"},
		},
		{
			"2026-10-24T10:00:03.0Z ========================================",
			LogLine{Time: "2026-10-24T10:00:03.0Z", Level: "other", Message: "========================================"},
		},
	}
	for _, tc := range cases {
		if got := parseLogLine(tc.in); got != tc.want {
			t.Errorf("parseLogLine(%q)\n got %+v\nwant %+v", tc.in, got, tc.want)
		}
	}
}

func frame(stream byte, payload string) []byte {
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
	return append(header, payload...)
}

func TestDemux(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(frame(1, "one\n"))
	buf.Write(frame(2, "two\nthree\n"))
	got, err := demux(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "one\ntwo\nthree\n" {
		t.Fatalf("got %q", got)
	}
}

func TestRedactVPNSettings(t *testing.T) {
	var decoded any
	_ = json.Unmarshal([]byte(`{"wireguard":{"private_key":"abc","public_key":"pub","addresses":["10.2.0.2/32"]},"provider":{"name":"protonvpn","server_selection":{"openvpn":{"password":"p","user":"u"}}}}`), &decoded)
	data, _ := json.Marshal(redactValue("", decoded))
	out := string(data)
	for _, leak := range []string{`"abc"`, `"p"`} {
		if strings.Contains(out, leak) {
			t.Errorf("secret %s leaked: %s", leak, out)
		}
	}
	for _, keep := range []string{`"pub"`, `"protonvpn"`, `"u"`} {
		if !strings.Contains(out, keep) {
			t.Errorf("value %s missing: %s", keep, out)
		}
	}
}

func TestOverriddenEnv(t *testing.T) {
	got := overriddenEnv(
		[]string{"PATH=/bin", "VPN_TYPE=wireguard", "FIREWALL=on", "WIREGUARD_PRIVATE_KEY=secret", "HEALTH_TARGET_ADDRESS=1.1.1.1"},
		[]string{"PATH=/bin", "VPN_TYPE=openvpn", "FIREWALL=on", "WIREGUARD_PRIVATE_KEY="},
	)
	want := []Environment{
		{Name: "HEALTH_TARGET_ADDRESS", Value: "1.1.1.1"},
		{Name: "VPN_TYPE", Value: "wireguard"},
		{Name: "WIREGUARD_PRIVATE_KEY", Value: redacted, Redacted: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("env[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// fakeEngine serves the Docker API routes the service uses.
type fakeEngine struct {
	mu       sync.Mutex
	restarts []string
	healthy  bool
	// hold, when set, keeps restart requests waiting until it is closed.
	hold chan struct{}
}

const gluetunID = "aaaaaaaaaaaa1111111111111111111111111111111111111111111111111111"

func (f *fakeEngine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.hold != nil && strings.HasSuffix(r.URL.Path, "/restart") {
		<-f.hold
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.URL.Path == "/containers/gluetun/json" || r.URL.Path == "/containers/"+gluetunID+"/json":
		health := "starting"
		if f.healthy {
			health = "healthy"
		}
		_, _ = w.Write([]byte(`{"Id":"` + gluetunID + `","Name":"/gluetun","Image":"sha256:img","State":{"Status":"running","Running":true,"StartedAt":"2026-10-24T10:00:00Z","Health":{"Status":"` + health + `","Log":[{"ExitCode":0,"Output":"ok"}]}},"Config":{"Image":"qmcgaw/gluetun","Env":["VPN_TYPE=wireguard"]},"HostConfig":{"Memory":1073741824,"NanoCpus":500000000},"NetworkSettings":{"Ports":{"8000/tcp":[{"HostIp":"0.0.0.0","HostPort":"8003"},{"HostIp":"::","HostPort":"8003"}]}}}`))
	case r.URL.Path == "/images/sha256:img/json":
		_, _ = w.Write([]byte(`{"Config":{"Env":["VPN_TYPE=openvpn"]}}`))
	case r.URL.Path == "/containers/json":
		_, _ = w.Write([]byte(`[
			{"Id":"` + gluetunID + `","Names":["/gluetun"],"HostConfig":{"NetworkMode":"bridge"}},
			{"Id":"qbit","Names":["/qbittorrent"],"State":"running","HostConfig":{"NetworkMode":"container:` + gluetunID + `"}},
			{"Id":"qui","Names":["/qui"],"State":"running","HostConfig":{"NetworkMode":"container:gluetun"}},
			{"Id":"other","Names":["/other"],"HostConfig":{"NetworkMode":"container:somethingelse"}}
		]`))
	case strings.HasSuffix(r.URL.Path, "/stats"):
		_, _ = w.Write([]byte(`{"memory_stats":{"usage":300,"limit":1000,"stats":{"inactive_file":100}},"cpu_stats":{"cpu_usage":{"total_usage":200},"system_cpu_usage":2000,"online_cpus":2},"precpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":1000}}`))
	case strings.HasSuffix(r.URL.Path, "/restart") && r.Method == http.MethodPost:
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/containers/"), "/restart")
		f.restarts = append(f.restarts, id)
		if id == gluetunID {
			f.healthy = true
		}
		w.WriteHeader(http.StatusNoContent)
	case strings.HasSuffix(r.URL.Path, "/logs"):
		_, _ = w.Write(frame(1, "2026-10-24T10:00:00Z 2026-10-24T10:00:00Z INFO [vpn] up\n"))
	default:
		http.NotFound(w, r)
	}
}

func newControlServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v1/vpn/status", "/v1/dns/status":
			_, _ = w.Write([]byte(`{"status":"running"}`))
		case "/v1/publicip/ip":
			_, _ = w.Write([]byte(`{"public_ip":"1.2.3.4","country":"Singapore","city":"Singapore","organization":"Proton AG"}`))
		case "/v1/openvpn/portforwarded":
			_, _ = w.Write([]byte(`{"port":51413}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestStatus(t *testing.T) {
	engine := &fakeEngine{healthy: true}
	dockerServer := httptest.NewServer(engine)
	defer dockerServer.Close()
	control := newControlServer()
	defer control.Close()

	svc := NewService(&memoryStore{settings: &models.GluetunSettings{
		ControlURL: control.URL, AuthType: models.GluetunAuthAPIKey, Secret: "key",
		DockerHost: "tcp://" + strings.TrimPrefix(dockerServer.URL, "http://"), ContainerName: "gluetun",
	}})

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	c := status.Control
	if !c.Reachable || c.VPNStatus != "running" || c.PublicIP == nil || c.PublicIP.Country != "Singapore" || len(c.ForwardedPorts) != 1 || c.ForwardedPorts[0] != 51413 {
		t.Fatalf("control status %+v (error %q)", c, c.Error)
	}
	d := status.Docker
	if !d.Available || d.Container == nil || d.Container.Health.Status != "healthy" || len(d.Container.Ports) != 1 {
		t.Fatalf("docker status %+v (error %q)", d, d.Error)
	}
	if len(d.Dependents) != 2 || d.Dependents[0].Name != "qbittorrent" || d.Dependents[1].Name != "qui" {
		t.Fatalf("dependents %+v", d.Dependents)
	}
	if d.Stats == nil || d.Stats.MemoryUsage != 200 || d.Stats.CPUPercent != 20 {
		t.Fatalf("stats %+v", d.Stats)
	}
	if len(d.Container.Environment) != 1 || d.Container.Environment[0].Value != "wireguard" {
		t.Fatalf("environment %+v", d.Container.Environment)
	}

	lines, err := svc.Logs(context.Background(), 100)
	if err != nil || len(lines) != 1 || lines[0].Module != "vpn" {
		t.Fatalf("logs %+v, %v", lines, err)
	}
}

func TestStatusReportsAuthFailure(t *testing.T) {
	control := newControlServer()
	defer control.Close()
	svc := NewService(&memoryStore{settings: &models.GluetunSettings{ControlURL: control.URL, AuthType: models.GluetunAuthNone, ContainerName: "gluetun"}})

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Control.Reachable || !strings.Contains(status.Control.Error, "authentication") {
		t.Fatalf("control %+v", status.Control)
	}
	if status.Docker.Available {
		t.Fatalf("docker should be off: %+v", status.Docker)
	}
}

func TestRestartContainerRestartsDependents(t *testing.T) {
	engine := &fakeEngine{hold: make(chan struct{})}
	dockerServer := httptest.NewServer(engine)
	defer dockerServer.Close()

	svc := NewService(&memoryStore{settings: &models.GluetunSettings{
		ControlURL: "http://127.0.0.1:1", AuthType: models.GluetunAuthNone,
		DockerHost: dockerServer.URL, ContainerName: "gluetun",
	}})

	state, err := svc.RestartContainer(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Running || len(state.Containers) != 3 {
		t.Fatalf("state %+v", state)
	}
	if _, err := svc.RestartContainer(context.Background(), true); err != ErrRestartRunning {
		t.Fatalf("second restart: %v", err)
	}
	close(engine.hold)

	deadline := time.Now().Add(5 * time.Second)
	for {
		svc.mu.Lock()
		running, finalErr := svc.restart.Running, svc.restart.Error
		svc.mu.Unlock()
		if !running {
			if finalErr != "" {
				t.Fatal(finalErr)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("restart did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}

	engine.mu.Lock()
	defer engine.mu.Unlock()
	if len(engine.restarts) != 3 || engine.restarts[0] != gluetunID {
		t.Fatalf("restarts %v", engine.restarts)
	}
}

func TestSaveSettingsValidates(t *testing.T) {
	svc := NewService(&memoryStore{settings: models.DefaultGluetunSettings()})
	bad := []*models.GluetunSettings{
		{ControlURL: "127.0.0.1:8000", AuthType: "none", ContainerName: "gluetun"},
		{ControlURL: "http://127.0.0.1:8000", AuthType: "token", ContainerName: "gluetun"},
		{ControlURL: "http://127.0.0.1:8000", AuthType: "none", ContainerName: ""},
		{ControlURL: "http://127.0.0.1:8000", AuthType: "none", ContainerName: "g", DockerHost: "ssh://host"},
	}
	for _, s := range bad {
		if err := svc.SaveSettings(context.Background(), s); err == nil {
			t.Errorf("settings %+v accepted", s)
		}
	}
	good := &models.GluetunSettings{ControlURL: "http://127.0.0.1:8000/", AuthType: "apikey", Secret: "k", ContainerName: "/gluetun", DockerHost: "unix:///var/run/docker.sock"}
	if err := svc.SaveSettings(context.Background(), good); err != nil {
		t.Fatal(err)
	}
	if good.ControlURL != "http://127.0.0.1:8000" || good.ContainerName != "gluetun" {
		t.Fatalf("not normalised: %+v", good)
	}
}
