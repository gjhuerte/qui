// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// Package themes embeds the built-in theme CSS files and exposes their
// metadata so the API can serve and validate them. The CSS is the single
// source of truth; the frontend parses the full stylesheet, this package
// only reads the small metadata header.
package themes

import (
	"cmp"
	"embed"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
)

//go:embed assets
var assetsFS embed.FS

// Theme is one built-in theme: its metadata and raw CSS.
type Theme struct {
	ID          string
	Name        string
	Description string
	CSS         string
}

var (
	nameRe        = regexp.MustCompile(`@name:\s*(.+?)\s*(?:\n|\*)`)
	descriptionRe = regexp.MustCompile(`@description:\s*(.+?)\s*(?:\n|\*)`)
	slugRe        = regexp.MustCompile(`[^a-z0-9]+`)
)

// GenerateID derives a theme id from its name. The rule is frozen: derived
// ids are stored in the database and in browsers, so it must never change.
// TestRegistryIDsPinned enforces this for every committed theme.
func GenerateID(name string) string {
	return strings.Trim(slugRe.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

var registry = load()

// All returns every embedded theme, "minimal" (the default) first, then by
// name, matching the order the old bundled loader presented.
func All() []Theme {
	return registry
}

// Exists reports whether id names an embedded built-in theme.
func Exists(id string) bool {
	return slices.ContainsFunc(registry, func(t Theme) bool { return t.ID == id })
}

func load() []Theme {
	var themes []Theme
	paths, _ := fs.Glob(assetsFS, "assets/*.css")
	for _, p := range paths {
		css, err := fs.ReadFile(assetsFS, p)
		if err != nil {
			continue
		}
		themes = append(themes, parse(string(css), strings.TrimSuffix(path.Base(p), ".css")))
	}
	slices.SortFunc(themes, func(a, b Theme) int {
		if a.ID == "minimal" {
			return -1
		}
		if b.ID == "minimal" {
			return 1
		}
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return themes
}

// parse reads a theme from its CSS.
func parse(css, fallbackID string) Theme {
	t := Theme{CSS: css}

	if m := nameRe.FindStringSubmatch(css); m != nil {
		t.Name = m[1]
	}
	if m := descriptionRe.FindStringSubmatch(css); m != nil {
		t.Description = m[1]
	}
	t.ID = cmp.Or(GenerateID(t.Name), fallbackID)
	return t
}
