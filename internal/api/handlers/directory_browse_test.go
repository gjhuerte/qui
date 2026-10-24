// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/fsops"
	localbackend "github.com/gjhuerte/qui/internal/fsops/local"
	"github.com/gjhuerte/qui/internal/models"
)

type stubFolderLister struct {
	backend fsops.Backend
	err     error
}

func (s stubFolderLister) Require(context.Context, int, models.FilesystemCapability, ...models.FilesystemCapability) (fsops.Backend, *models.Instance, error) {
	return s.backend, nil, s.err
}

func getDirectoryContent(t *testing.T, h *TorrentsHandler, dirPath, mode string) *httptest.ResponseRecorder {
	t.Helper()
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("instanceID", "1")
	ctx := context.WithValue(t.Context(), chi.RouteCtxKey, routeCtx)
	query := url.Values{"dirPath": {dirPath}, "mode": {mode}}
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/getDirectoryContent?"+query.Encode(), http.NoBody)
	rec := httptest.NewRecorder()
	h.GetDirectoryContent(rec, req)
	return rec
}

// Without a qBittorrent client that can list folders (syncManager is nil
// here, like qBittorrent before 5.0), qui lists the folders from disk.
func TestGetDirectoryContentListsFromDiskWhenQBittorrentCannot(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "Movies"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, "anime"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"), nil, 0o600))
	if err := os.Symlink(filepath.Join(root, "Movies"), filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	h := &TorrentsHandler{folderLister: stubFolderLister{backend: localbackend.NewBackend()}}
	dir := root + string(filepath.Separator)

	rec := getDirectoryContent(t, h, dir, "dirs")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var dirs []string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &dirs))
	require.Equal(t, []string{dir + "anime", dir + "linked", dir + "Movies"}, dirs)

	rec = getDirectoryContent(t, h, root, "files")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var files []string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &files))
	require.Equal(t, []string{dir + "notes.txt"}, files)

	rec = getDirectoryContent(t, h, filepath.Join(root, "missing"), "dirs")
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// An instance without filesystem access is left to qBittorrent.
func TestListFromDiskWithoutFilesystemAccessIsNotHandled(t *testing.T) {
	for _, err := range []error{fsops.ErrNotCapable, fsops.ErrNoFilesystemAccess} {
		h := &TorrentsHandler{folderLister: stubFolderLister{err: err}}
		paths, handled, listErr := h.listFromDisk(t.Context(), 1, "/data/", "dirs")
		require.False(t, handled)
		require.NoError(t, listErr)
		require.Nil(t, paths)
	}

	paths, handled, listErr := (&TorrentsHandler{}).listFromDisk(t.Context(), 1, "/data/", "dirs")
	require.False(t, handled)
	require.NoError(t, listErr)
	require.Nil(t, paths)
}
