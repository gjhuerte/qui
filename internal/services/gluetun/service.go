// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// Package gluetun shows the state of the gluetun VPN container and restarts it.
// The VPN state, public IP and forwarded port come from gluetun's control
// server; logs, health, resource use and container restarts come from the
// Docker API, which works only when qui can reach the Docker socket.
package gluetun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/gjhuerte/qui/internal/models"
)

// Store is the persistence the service needs (models.GluetunSettingsStore).
type Store interface {
	Get(ctx context.Context) (*models.GluetunSettings, error)
	Save(ctx context.Context, settings *models.GluetunSettings) error
}

type Service struct {
	store  Store
	logger zerolog.Logger

	mu      sync.Mutex
	restart RestartState
}

func NewService(store Store) *Service {
	return &Service{store: store, logger: log.Logger.With().Str("module", "gluetun").Logger()}
}

// ErrRestartRunning means a container restart is already in progress.
var ErrRestartRunning = errors.New("a restart is already running")

// RestartState reports the last container restart qui ran.
type RestartState struct {
	Running    bool       `json:"running"`
	Step       string     `json:"step,omitempty"` // gluetun | waiting | dependents | done
	Containers []string   `json:"containers,omitempty"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// Status is everything the Gluetun page shows in one request.
type Status struct {
	Control ControlStatus `json:"control"`
	Docker  DockerStatus  `json:"docker"`
	Restart RestartState  `json:"restart"`
}

type ControlStatus struct {
	Reachable      bool      `json:"reachable"`
	Error          string    `json:"error,omitempty"`
	VPNStatus      string    `json:"vpnStatus,omitempty"`
	DNSStatus      string    `json:"dnsStatus,omitempty"`
	PublicIP       *PublicIP `json:"publicIp,omitempty"`
	ForwardedPorts []int     `json:"forwardedPorts"`
}

type DockerStatus struct {
	Available  bool           `json:"available"`
	Error      string         `json:"error,omitempty"`
	Container  *ContainerInfo `json:"container,omitempty"`
	Stats      *Stats         `json:"stats,omitempty"`
	Dependents []Dependent    `json:"dependents"`
}

type ContainerInfo struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Image         string        `json:"image"`
	State         string        `json:"state"`
	Running       bool          `json:"running"`
	ExitCode      int           `json:"exitCode"`
	Error         string        `json:"error,omitempty"`
	CreatedAt     string        `json:"createdAt,omitempty"`
	StartedAt     string        `json:"startedAt,omitempty"`
	FinishedAt    string        `json:"finishedAt,omitempty"`
	RestartCount  int           `json:"restartCount"`
	RestartPolicy string        `json:"restartPolicy,omitempty"`
	MemoryLimit   int64         `json:"memoryLimit"`
	CPULimit      float64       `json:"cpuLimit"`
	CapAdd        []string      `json:"capAdd"`
	Health        *Health       `json:"health,omitempty"`
	Ports         []Port        `json:"ports"`
	Environment   []Environment `json:"environment"`
}

type Health struct {
	Status        string        `json:"status"`
	FailingStreak int           `json:"failingStreak"`
	Checks        []HealthCheck `json:"checks"`
}

type HealthCheck struct {
	Start    string `json:"start"`
	ExitCode int    `json:"exitCode"`
	Output   string `json:"output"`
}

type Port struct {
	Container string `json:"container"`
	Host      string `json:"host"`
}

// Environment is one variable the container sets on top of the image defaults.
type Environment struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Redacted bool   `json:"redacted,omitempty"`
}

type Stats struct {
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryUsage uint64  `json:"memoryUsage"`
	MemoryLimit uint64  `json:"memoryLimit"`
	RxBytes     uint64  `json:"rxBytes"`
	TxBytes     uint64  `json:"txBytes"`
	Pids        uint64  `json:"pids"`
}

// Dependent is a container that shares gluetun's network namespace.
type Dependent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	State  string `json:"state"`
	Status string `json:"status"`
}

// Settings returns the stored settings.
func (s *Service) Settings(ctx context.Context) (*models.GluetunSettings, error) {
	return s.store.Get(ctx)
}

// SaveSettings validates and stores the settings.
func (s *Service) SaveSettings(ctx context.Context, settings *models.GluetunSettings) error {
	settings.ControlURL = strings.TrimRight(strings.TrimSpace(settings.ControlURL), "/")
	settings.DockerHost = strings.TrimSpace(settings.DockerHost)
	settings.ContainerName = strings.TrimPrefix(strings.TrimSpace(settings.ContainerName), "/")

	if !strings.HasPrefix(settings.ControlURL, "http://") && !strings.HasPrefix(settings.ControlURL, "https://") {
		return fmt.Errorf("%w: the control server URL must start with http:// or https://", ErrInvalidSettings)
	}
	switch settings.AuthType {
	case models.GluetunAuthNone, models.GluetunAuthAPIKey, models.GluetunAuthBasic:
	default:
		return fmt.Errorf("%w: unknown auth type %q", ErrInvalidSettings, settings.AuthType)
	}
	if settings.ContainerName == "" {
		return fmt.Errorf("%w: the container name is required", ErrInvalidSettings)
	}
	if settings.DockerHost != "" {
		if _, err := newDockerClient(settings.DockerHost); err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidSettings, err.Error())
		}
	}
	return s.store.Save(ctx, settings)
}

// ErrInvalidSettings is a validation error the handler maps to 400.
var ErrInvalidSettings = errors.New("invalid settings")

// Status queries the control server and Docker in parallel.
func (s *Service) Status(ctx context.Context) (*Status, error) {
	settings, err := s.store.Get(ctx)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	status := &Status{}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		status.Control = s.controlStatus(ctx, newControlClient(settings))
	}()
	go func() {
		defer wg.Done()
		status.Docker = s.dockerStatus(ctx, settings)
	}()
	wg.Wait()

	s.mu.Lock()
	status.Restart = s.restart
	s.mu.Unlock()
	return status, nil
}

func (s *Service) controlStatus(ctx context.Context, control *controlClient) ControlStatus {
	out := ControlStatus{ForwardedPorts: []int{}}

	vpn, err := control.status(ctx, "/v1/vpn/status")
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.Reachable = true
	out.VPNStatus = vpn

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []string
	addErr := func(what string, err error) {
		// Routes missing from older gluetun versions are not errors.
		if err == nil || errors.Is(err, errNotFound) {
			return
		}
		mu.Lock()
		errs = append(errs, what+": "+err.Error())
		mu.Unlock()
	}

	wg.Add(3)
	go func() {
		defer wg.Done()
		dns, err := control.status(ctx, "/v1/dns/status")
		out.DNSStatus = dns
		addErr("dns", err)
	}()
	go func() {
		defer wg.Done()
		ip, err := control.publicIP(ctx)
		out.PublicIP = ip
		addErr("public ip", err)
	}()
	go func() {
		defer wg.Done()
		ports, err := control.forwardedPorts(ctx)
		if ports != nil {
			out.ForwardedPorts = ports
		}
		addErr("port forwarding", err)
	}()
	wg.Wait()

	sort.Strings(errs)
	out.Error = strings.Join(errs, "; ")
	return out
}

func (s *Service) dockerStatus(ctx context.Context, settings *models.GluetunSettings) DockerStatus {
	out := DockerStatus{Dependents: []Dependent{}}
	if settings.DockerHost == "" {
		out.Error = "Docker access is turned off"
		return out
	}

	docker, err := newDockerClient(settings.DockerHost)
	if err != nil {
		out.Error = err.Error()
		return out
	}

	container, err := docker.inspect(ctx, settings.ContainerName)
	if err != nil {
		out.Error = dockerErrorMessage(err, settings)
		return out
	}
	out.Available = true

	var imageEnv []string
	var stats *statsJSON
	var dependents []Dependent
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		imageEnv, _ = docker.imageEnv(ctx, container.Image)
	}()
	go func() {
		defer wg.Done()
		if container.State.Running {
			stats, _ = docker.stats(ctx, container.ID)
		}
	}()
	go func() {
		defer wg.Done()
		dependents, _ = s.dependents(ctx, docker, container)
	}()
	wg.Wait()

	out.Container = toContainerInfo(container, imageEnv)
	if stats != nil {
		out.Stats = toStats(stats)
	}
	if dependents != nil {
		out.Dependents = dependents
	}
	return out
}

func dockerErrorMessage(err error, settings *models.GluetunSettings) string {
	var dockerErr *dockerError
	if errors.As(err, &dockerErr) && dockerErr.Status == 404 {
		return fmt.Sprintf("no container named %q", settings.ContainerName)
	}
	message := err.Error()
	if strings.Contains(message, "permission denied") {
		return message + " (qui needs read and write access to the Docker socket)"
	}
	if strings.Contains(message, "no such file or directory") && strings.HasPrefix(settings.DockerHost, "unix://") {
		return message + " (mount the Docker socket into the qui container)"
	}
	return message
}

// dependents lists containers that run with network_mode: "service:gluetun".
// Docker records that as container:<id> (or container:<name>).
func (s *Service) dependents(ctx context.Context, docker *dockerClient, gluetun *containerJSON) ([]Dependent, error) {
	containers, err := docker.list(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimPrefix(gluetun.Name, "/")
	out := []Dependent{}
	for _, c := range containers {
		target, ok := strings.CutPrefix(c.HostConfig.NetworkMode, "container:")
		if !ok || (target != gluetun.ID && target != name && !strings.HasPrefix(gluetun.ID, target)) {
			continue
		}
		depName := c.ID[:min(12, len(c.ID))]
		if len(c.Names) > 0 {
			depName = strings.TrimPrefix(c.Names[0], "/")
		}
		out = append(out, Dependent{ID: c.ID, Name: depName, Image: c.Image, State: c.State, Status: c.Status})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func toContainerInfo(c *containerJSON, imageEnv []string) *ContainerInfo {
	info := &ContainerInfo{
		ID:            c.ID[:min(12, len(c.ID))],
		Name:          strings.TrimPrefix(c.Name, "/"),
		Image:         c.Config.Image,
		State:         c.State.Status,
		Running:       c.State.Running,
		ExitCode:      c.State.ExitCode,
		Error:         c.State.Error,
		CreatedAt:     c.Created,
		StartedAt:     zeroTimeToEmpty(c.State.StartedAt),
		FinishedAt:    zeroTimeToEmpty(c.State.FinishedAt),
		RestartCount:  c.RestartCount,
		RestartPolicy: c.HostConfig.RestartPolicy.Name,
		MemoryLimit:   c.HostConfig.Memory,
		CPULimit:      float64(c.HostConfig.NanoCpus) / 1e9,
		CapAdd:        c.HostConfig.CapAdd,
		Ports:         []Port{},
		Environment:   overriddenEnv(c.Config.Env, imageEnv),
	}
	if info.CapAdd == nil {
		info.CapAdd = []string{}
	}

	if h := c.State.Health; h != nil {
		health := &Health{Status: h.Status, FailingStreak: h.FailingStreak, Checks: []HealthCheck{}}
		for i := len(h.Log) - 1; i >= 0 && len(health.Checks) < 5; i-- {
			entry := h.Log[i]
			health.Checks = append(health.Checks, HealthCheck{
				Start:    entry.Start,
				ExitCode: entry.ExitCode,
				Output:   strings.TrimSpace(entry.Output),
			})
		}
		info.Health = health
	}

	for containerPort, bindings := range c.NetworkSettings.Ports {
		for _, b := range bindings {
			host := b.HostPort
			if b.HostIP != "" && b.HostIP != "0.0.0.0" && b.HostIP != "::" {
				host = b.HostIP + ":" + b.HostPort
			}
			info.Ports = append(info.Ports, Port{Container: containerPort, Host: host})
		}
	}
	// One entry per port: Docker lists IPv4 and IPv6 bindings separately.
	info.Ports = slices.CompactFunc(sortPorts(info.Ports), func(a, b Port) bool { return a == b })
	return info
}

func sortPorts(ports []Port) []Port {
	sort.Slice(ports, func(i, j int) bool {
		if ports[i].Container != ports[j].Container {
			return portNumber(ports[i].Container) < portNumber(ports[j].Container)
		}
		return ports[i].Host < ports[j].Host
	})
	return ports
}

func portNumber(spec string) int {
	var n int
	_, _ = fmt.Sscanf(spec, "%d", &n)
	return n
}

func zeroTimeToEmpty(value string) string {
	if strings.HasPrefix(value, "0001-01-01") {
		return ""
	}
	return value
}

// overriddenEnv keeps the variables the container sets on top of the image
// (the compose environment), with secrets hidden.
func overriddenEnv(containerEnv, imageEnv []string) []Environment {
	defaults := make(map[string]string, len(imageEnv))
	for _, kv := range imageEnv {
		k, v, _ := strings.Cut(kv, "=")
		defaults[k] = v
	}

	out := []Environment{}
	for _, kv := range containerEnv {
		k, v, _ := strings.Cut(kv, "=")
		if d, ok := defaults[k]; ok && d == v {
			continue
		}
		if k == "PATH" || k == "HOSTNAME" || k == "HOME" {
			continue
		}
		env := Environment{Name: k, Value: v}
		if v != "" && isSecretName(k) {
			env.Value, env.Redacted = redacted, true
		}
		out = append(out, env)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func toStats(s *statsJSON) *Stats {
	out := &Stats{
		MemoryUsage: s.MemoryStats.Usage,
		MemoryLimit: s.MemoryStats.Limit,
		Pids:        s.PidsStats.Current,
	}
	// Match `docker stats`: page cache does not count as used memory.
	if cache, ok := s.MemoryStats.Stats["inactive_file"]; ok && cache < out.MemoryUsage {
		out.MemoryUsage -= cache
	}

	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage) - float64(s.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(s.CPUStats.SystemCPUUsage) - float64(s.PreCPUStats.SystemCPUUsage)
	if cpuDelta > 0 && systemDelta > 0 {
		cpus := float64(s.CPUStats.OnlineCPUs)
		if cpus == 0 {
			cpus = 1
		}
		out.CPUPercent = cpuDelta / systemDelta * cpus * 100
	}
	for _, n := range s.Networks {
		out.RxBytes += n.RxBytes
		out.TxBytes += n.TxBytes
	}
	return out
}

// VPNSettings returns gluetun's VPN settings with secrets hidden.
func (s *Service) VPNSettings(ctx context.Context) (json.RawMessage, error) {
	settings, err := s.store.Get(ctx)
	if err != nil {
		return nil, err
	}
	return newControlClient(settings).vpnSettings(ctx)
}

// SetVPNStatus starts or stops the VPN inside gluetun.
func (s *Service) SetVPNStatus(ctx context.Context, status string) error {
	if status != "running" && status != "stopped" {
		return fmt.Errorf("%w: status must be running or stopped", ErrInvalidSettings)
	}
	settings, err := s.store.Get(ctx)
	if err != nil {
		return err
	}
	s.logger.Info().Str("status", status).Msg("gluetun: setting VPN status")
	return newControlClient(settings).setVPNStatus(ctx, status)
}

// RestartVPN stops and starts the VPN through the control server. The
// container and its network namespace stay up, so containers that share it
// (qui, qBittorrent) keep working once the tunnel is back.
func (s *Service) RestartVPN(ctx context.Context) error {
	settings, err := s.store.Get(ctx)
	if err != nil {
		return err
	}
	control := newControlClient(settings)
	s.logger.Info().Msg("gluetun: restarting the VPN")

	if err := control.setVPNStatus(ctx, "stopped"); err != nil {
		return fmt.Errorf("stop VPN: %w", err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if status, err := control.status(ctx, "/v1/vpn/status"); err == nil && status == "stopped" {
			break
		}
		if err := sleep(ctx, time.Second); err != nil {
			return err
		}
	}
	if err := control.setVPNStatus(ctx, "running"); err != nil {
		return fmt.Errorf("start VPN: %w", err)
	}
	return nil
}

// RestartContainer restarts the gluetun container in the background. A
// container that shares gluetun's network keeps a dead network namespace after
// gluetun restarts, so with withDependents those are restarted too once gluetun
// is up again. qui itself may be one of them: Docker receives every restart
// request before any of them stops qui, so the order does not matter.
func (s *Service) RestartContainer(ctx context.Context, withDependents bool) (*RestartState, error) {
	settings, err := s.store.Get(ctx)
	if err != nil {
		return nil, err
	}
	if settings.DockerHost == "" {
		return nil, fmt.Errorf("%w: Docker access is turned off", ErrInvalidSettings)
	}
	docker, err := newDockerClient(settings.DockerHost)
	if err != nil {
		return nil, err
	}
	container, err := docker.inspect(ctx, settings.ContainerName)
	if err != nil {
		return nil, errors.New(dockerErrorMessage(err, settings))
	}

	var dependents []Dependent
	if withDependents {
		if dependents, err = s.dependents(ctx, docker, container); err != nil {
			return nil, err
		}
	}

	s.mu.Lock()
	if s.restart.Running {
		s.mu.Unlock()
		return nil, ErrRestartRunning
	}
	now := time.Now()
	names := []string{strings.TrimPrefix(container.Name, "/")}
	for _, d := range dependents {
		names = append(names, d.Name)
	}
	s.restart = RestartState{Running: true, Step: "gluetun", Containers: names, StartedAt: &now}
	state := s.restart
	s.mu.Unlock()

	go s.runRestart(docker, container.ID, dependents)
	return &state, nil
}

func (s *Service) runRestart(docker *dockerClient, id string, dependents []Dependent) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err := s.restartSequence(ctx, docker, id, dependents)

	s.mu.Lock()
	now := time.Now()
	s.restart.Running = false
	s.restart.FinishedAt = &now
	s.restart.Step = "done"
	if err != nil {
		s.restart.Error = err.Error()
		s.logger.Error().Err(err).Msg("gluetun: container restart failed")
	} else {
		s.logger.Info().Msg("gluetun: container restart finished")
	}
	s.mu.Unlock()
}

func (s *Service) restartSequence(ctx context.Context, docker *dockerClient, id string, dependents []Dependent) error {
	s.logger.Info().Int("dependents", len(dependents)).Msg("gluetun: restarting the container")
	if err := docker.restart(ctx, id, 30*time.Second); err != nil {
		return fmt.Errorf("restart gluetun: %w", err)
	}

	s.setStep("waiting")
	if err := waitHealthy(ctx, docker, id, 2*time.Minute); err != nil {
		return err
	}
	if len(dependents) == 0 {
		return nil
	}

	s.setStep("dependents")
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []string
	for _, d := range dependents {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.logger.Info().Str("container", d.Name).Msg("gluetun: restarting dependent container")
			if err := docker.restart(ctx, d.ID, 10*time.Second); err != nil {
				mu.Lock()
				errs = append(errs, d.Name+": "+err.Error())
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(errs) > 0 {
		sort.Strings(errs)
		return fmt.Errorf("restart dependents: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (s *Service) setStep(step string) {
	s.mu.Lock()
	s.restart.Step = step
	s.mu.Unlock()
}

// waitHealthy waits until the container runs and, when it has a health check,
// reports healthy. A container without a health check counts as ready once it
// runs.
func waitHealthy(ctx context.Context, docker *dockerClient, id string, limit time.Duration) error {
	deadline := time.Now().Add(limit)
	last := ""
	for time.Now().Before(deadline) {
		c, err := docker.inspect(ctx, id)
		if err == nil {
			switch {
			case c.State.Running && (c.State.Health == nil || c.State.Health.Status == "healthy"):
				return nil
			case c.State.Health != nil:
				last = "health " + c.State.Health.Status
			default:
				last = c.State.Status
			}
		} else {
			last = err.Error()
		}
		if err := sleep(ctx, 2*time.Second); err != nil {
			return err
		}
	}
	return fmt.Errorf("gluetun did not become healthy within %s (last state: %s)", limit, last)
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// LogLine is one line of the container log.
type LogLine struct {
	Time    string `json:"time,omitempty"`
	Level   string `json:"level"` // debug | info | warn | error | other
	Module  string `json:"module,omitempty"`
	Message string `json:"message"`
}

// Logs returns the last tail lines of the gluetun container log, oldest first.
func (s *Service) Logs(ctx context.Context, tail int) ([]LogLine, error) {
	settings, err := s.store.Get(ctx)
	if err != nil {
		return nil, err
	}
	if settings.DockerHost == "" {
		return nil, fmt.Errorf("%w: Docker access is turned off", ErrInvalidSettings)
	}
	docker, err := newDockerClient(settings.DockerHost)
	if err != nil {
		return nil, err
	}
	container, err := docker.inspect(ctx, settings.ContainerName)
	if err != nil {
		return nil, errors.New(dockerErrorMessage(err, settings))
	}
	raw, err := docker.logs(ctx, container.ID, container.Config.Tty, tail)
	if err != nil {
		return nil, err
	}

	lines := make([]LogLine, 0, len(raw))
	for _, line := range raw {
		lines = append(lines, parseLogLine(line))
	}
	return lines, nil
}

// Gluetun writes "2026-10-24T10:00:00Z INFO [wireguard] message". Docker adds
// its own RFC 3339 timestamp in front.
var (
	ansiEscape  = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	gluetunLine = regexp.MustCompile(`^(?:(\d{4}-\d{2}-\d{2}T\S+)\s+)?(DEBUG|INFO|WARN|WARNING|ERROR)\s+(?:\[([^\]]+)\]\s*)?(.*)$`)
)

func parseLogLine(line string) LogLine {
	line = ansiEscape.ReplaceAllString(line, "")
	out := LogLine{Level: "other"}

	if ts, rest, ok := strings.Cut(line, " "); ok {
		if _, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			out.Time = ts
			line = rest
		}
	}

	m := gluetunLine.FindStringSubmatch(line)
	if m == nil {
		out.Message = line
		return out
	}
	if m[1] != "" {
		out.Time = m[1]
	}
	switch m[2] {
	case "DEBUG":
		out.Level = "debug"
	case "INFO":
		out.Level = "info"
	case "WARN", "WARNING":
		out.Level = "warn"
	case "ERROR":
		out.Level = "error"
	}
	out.Module = m[3]
	out.Message = m[4]
	return out
}
