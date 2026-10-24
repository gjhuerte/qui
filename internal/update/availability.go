// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package update

import (
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

// Inputs are the facts that decide Availability. qui measures them once at startup.
type Inputs struct {
	AppContainer bool
	// BinaryPath is the resolved binary that a Restart execs. Decide ignores it.
	BinaryPath string
}

// Availability reports whether qui offers Restart.
type Availability struct {
	Restart bool
}

func Decide(in Inputs) Availability {
	return Availability{Restart: !in.AppContainer}
}

// Measure reads the Availability inputs from the host and logs each one at DEBUG level.
func Measure(log zerolog.Logger) Inputs {
	in := Inputs{
		AppContainer: appContainer(func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		}, os.Getpid(), os.Getenv("KUBERNETES_SERVICE_HOST") != ""),
	}

	binaryPath, err := resolveBinaryPath()
	if err == nil {
		in.BinaryPath = binaryPath
	}

	log.Debug().
		Err(err).
		Bool("appContainer", in.AppContainer).
		Str("binaryPath", binaryPath).
		Msg("availability inputs")

	return in
}

// appContainer reports whether qui runs as the application of a container.
// Unlike config.detectContainer it ignores /dev/.lxc-boot-id: a system container
// such as Proxmox LXC runs qui under its own init, so a Restart works there.
// The kubelet sets KUBERNETES_SERVICE_HOST in every pod, which covers a pod on
// containerd where qui is not PID 1 (shareProcessNamespace, an init wrapper).
func appContainer(exists func(path string) bool, pid int, kubernetes bool) bool {
	return exists("/.dockerenv") || exists("/run/.containerenv") || pid == 1 || kubernetes
}

func resolveBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}
