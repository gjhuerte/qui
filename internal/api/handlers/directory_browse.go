// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"errors"
	"slices"
	"strings"

	qbt "github.com/autobrr/go-qbittorrent"

	"github.com/gjhuerte/qui/internal/fsops"
	"github.com/gjhuerte/qui/internal/models"
)

// folderLister resolves the filesystem backend of an instance. Satisfied by
// *fsops.Pool.
type folderLister interface {
	Require(ctx context.Context, instanceID int, capability models.FilesystemCapability, more ...models.FilesystemCapability) (fsops.Backend, *models.Instance, error)
}

// SetFolderLister lets the folder picker list folders through qui's own
// filesystem access (local or SSH) when qBittorrent cannot list them, which is
// every qBittorrent before 5.0.
func (h *TorrentsHandler) SetFolderLister(pool *fsops.Pool) {
	if pool != nil {
		h.folderLister = pool
	}
}

// listFromDisk lists dirPath through the filesystem access of the instance,
// in the same shape as the qBittorrent listing: full paths, sorted by name.
// handled is false when the instance has no filesystem access, so the caller
// falls back to qBittorrent.
func (h *TorrentsHandler) listFromDisk(ctx context.Context, instanceID int, dirPath string, mode qbt.DirectoryContentMode) (paths []string, handled bool, err error) {
	if h.folderLister == nil {
		return nil, false, nil
	}
	backend, _, err := h.folderLister.Require(ctx, instanceID, models.CapabilityRead)
	if err != nil {
		if errors.Is(err, fsops.ErrNotCapable) || errors.Is(err, fsops.ErrNoFilesystemAccess) {
			return nil, false, nil
		}
		return nil, true, err
	}

	entries, err := backend.ReadDir(ctx, dirPath)
	if err != nil {
		return nil, true, err
	}

	base := dirPath
	if !strings.HasSuffix(base, "/") && !strings.HasSuffix(base, `\`) {
		base += backend.Paths().Separator()
	}

	paths = make([]string, 0, len(entries))
	for _, entry := range entries {
		full := base + entry.Name
		isDir := entry.IsDir
		if entry.IsSymlink {
			// A link to a folder opens like a folder. A broken link is a file.
			if info, statErr := backend.Stat(ctx, full); statErr == nil {
				isDir = info.IsDir
			}
		}
		switch {
		case mode == qbt.DirectoryContentDirs && !isDir:
			continue
		case mode == qbt.DirectoryContentFiles && isDir:
			continue
		}
		paths = append(paths, full)
	}
	slices.SortFunc(paths, func(a, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})
	return paths, true, nil
}
