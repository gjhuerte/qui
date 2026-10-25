// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/gjhuerte/qui/internal/models"
)

// Song is a track on a service: one library track on Jellyfin or Plex, or
// one search result on Spotify or YouTube Music.
type Song struct {
	ID      string
	Title   string
	Artists []string
	Album   string
}

var (
	bracketed = regexp.MustCompile(`[(\[][^)\]]*[)\]]`)
	// " - Remastered 2011", " - Live", " - Radio Edit": the part after the
	// dash names a version, not another song.
	versionSuffix = regexp.MustCompile(`\s[-–—]\s.*$`)
	featuring     = regexp.MustCompile(`(?i)\s(?:feat\.?|ft\.?|featuring)\s.*$`)
)

// normalize folds a title or artist for comparison: lower case, without
// bracketed parts, version suffixes, featured artists and punctuation.
func normalize(s string) string {
	s = bracketed.ReplaceAllString(s, " ")
	s = versionSuffix.ReplaceAllString(s, "")
	s = featuring.ReplaceAllString(s, "")
	s = strings.ToLower(strings.ReplaceAll(s, "&", " and "))
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			space = false
		} else {
			space = true
		}
	}
	return strings.TrimPrefix(b.String(), "the ")
}

// sameText reports whether two normalized strings name the same thing:
// equal, or one contains the other ("beatles" in "beatles and friends").
func sameText(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.Contains(a, b) || strings.Contains(b, a)
}

func artistMatches(want string, artists []string) bool {
	w := normalize(want)
	if w == "" {
		return true
	}
	for _, a := range artists {
		if sameText(w, normalize(a)) {
			return true
		}
	}
	return false
}

// best returns the candidate that is the track: the same title and one of its
// artists. An exact title wins over a title that only contains the other.
func best(t models.PlaylistTrack, candidates []Song) (Song, bool) {
	title := normalize(t.Title)
	var fallback *Song
	for i, c := range candidates {
		ct := normalize(c.Title)
		if !sameText(title, ct) || !artistMatches(t.Artist, c.Artists) {
			continue
		}
		if ct == title {
			return c, true
		}
		if fallback == nil {
			fallback = &candidates[i]
		}
	}
	if fallback != nil {
		return *fallback, true
	}
	return Song{}, false
}

// libraryIndex finds tracks in a whole library by title.
type libraryIndex map[string][]Song

func newLibraryIndex(songs []Song) libraryIndex {
	idx := libraryIndex{}
	for _, s := range songs {
		key := normalize(s.Title)
		idx[key] = append(idx[key], s)
	}
	return idx
}

// find returns the ID of the track in the library, or "".
func (idx libraryIndex) find(t models.PlaylistTrack) string {
	if s, ok := best(t, idx[normalize(t.Title)]); ok {
		return s.ID
	}
	return ""
}
