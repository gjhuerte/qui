// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"fmt"
	"sort"
	"strings"
)

// FormatLibrary writes a music library as compact text for an AI assistant:
// one artist per line, then one indented line per album with its titles.
// Grouping keeps a large library small enough to attach to a chat.
func FormatLibrary(serverName string, songs []Song) string {
	type album struct {
		name   string
		titles []string
	}
	artists := map[string]map[string]*album{}
	for _, s := range songs {
		artist := "Unknown artist"
		if len(s.Artists) > 0 && strings.TrimSpace(s.Artists[0]) != "" {
			artist = strings.TrimSpace(s.Artists[0])
		}
		albums := artists[artist]
		if albums == nil {
			albums = map[string]*album{}
			artists[artist] = albums
		}
		a := albums[s.Album]
		if a == nil {
			a = &album{name: s.Album}
			albums[s.Album] = a
		}
		a.titles = append(a.titles, s.Title)
	}

	names := make([]string, 0, len(artists))
	for name := range artists {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })

	var b strings.Builder
	fmt.Fprintf(&b, "# Music library of %s: %d tracks by %d artists\n", serverName, len(songs), len(names))
	b.WriteString("# Format: artist, then one line per album: \"  Album: title | title\"\n\n")
	for _, name := range names {
		b.WriteString(name)
		b.WriteByte('\n')
		albums := artists[name]
		keys := make([]string, 0, len(albums))
		for k := range albums {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			a := albums[k]
			label := a.name
			if label == "" {
				label = "Singles"
			}
			fmt.Fprintf(&b, "  %s: %s\n", label, strings.Join(a.titles, " | "))
		}
	}
	return b.String()
}
