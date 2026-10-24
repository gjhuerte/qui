// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package update

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAvailability(t *testing.T) {
	tests := []struct {
		name        string
		files       []string
		pid         int
		kubernetes  bool
		wantRestart bool
	}{
		{name: "host", wantRestart: true},
		{name: "docker", files: []string{"/.dockerenv"}},
		{name: "podman", files: []string{"/run/.containerenv"}},
		{name: "pid 1", pid: 1},
		{name: "kubernetes pod", kubernetes: true},
		{name: "lxc system container", files: []string{"/dev/.lxc-boot-id"}, wantRestart: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists := func(path string) bool { return slices.Contains(tt.files, path) }
			got := Decide(Inputs{AppContainer: appContainer(exists, tt.pid, tt.kubernetes)})
			require.Equal(t, Availability{Restart: tt.wantRestart}, got)
		})
	}
}
