// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gjhuerte/qui/internal/models"
)

// maxTracks caps one playlist; AI assistants sometimes ramble.
const maxTracks = 1000

// Imported is a playlist read from an AI assistant's answer.
type Imported struct {
	Name        string
	Description string
	Tracks      []models.PlaylistTrack
}

// aiPlaylist is the JSON shape the prompt asks the assistant for.
type aiPlaylist struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Tracks      []struct {
		Artist string `json:"artist"`
		Title  string `json:"title"`
		Album  string `json:"album"`
	} `json:"tracks"`
}

var listMarker = regexp.MustCompile(`^\s*(?:\d+[.)]\s*|[-*•]\s+)`)

// ParseAIAnswer reads the playlist out of an AI assistant's answer: the JSON
// object the prompt asks for (also inside a code fence or surrounded by
// chatter), or failing that one "Artist - Title" line per track.
func ParseAIAnswer(text string) (*Imported, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("%w: paste the answer of the AI assistant", ErrInvalid)
	}

	out := &Imported{}
	if start, end := strings.Index(text, "{"), strings.LastIndex(text, "}"); start >= 0 && end > start {
		var pl aiPlaylist
		if err := json.Unmarshal([]byte(text[start:end+1]), &pl); err == nil && len(pl.Tracks) > 0 {
			out.Name, out.Description = strings.TrimSpace(pl.Name), strings.TrimSpace(pl.Description)
			for _, t := range pl.Tracks {
				out.Tracks = append(out.Tracks, models.PlaylistTrack{Artist: t.Artist, Title: t.Title, Album: t.Album})
			}
		}
	}
	if len(out.Tracks) == 0 {
		for _, line := range strings.Split(text, "\n") {
			if t, ok := parseLine(line); ok {
				out.Tracks = append(out.Tracks, t)
			}
		}
	}

	out.Tracks = cleanTracks(out.Tracks)
	if len(out.Tracks) == 0 {
		return nil, fmt.Errorf("%w: no tracks found. Paste the JSON the assistant wrote, or one \"Artist - Title\" per line", ErrInvalid)
	}
	return out, nil
}

// parseLine reads "Artist - Title" (also with an en or em dash) or
// "Title by Artist".
func parseLine(line string) (models.PlaylistTrack, bool) {
	line = strings.TrimSpace(listMarker.ReplaceAllString(line, ""))
	line = strings.Trim(line, "\"'`")
	for _, sep := range []string{" - ", " – ", " — "} {
		if artist, title, ok := strings.Cut(line, sep); ok {
			return models.PlaylistTrack{Artist: artist, Title: title}, true
		}
	}
	if i := strings.LastIndex(line, " by "); i > 0 {
		return models.PlaylistTrack{Artist: line[i+4:], Title: line[:i]}, true
	}
	return models.PlaylistTrack{}, false
}

// cleanTracks trims the fields, drops tracks without a title and duplicates,
// and caps the list.
func cleanTracks(tracks []models.PlaylistTrack) []models.PlaylistTrack {
	seen := map[string]bool{}
	out := make([]models.PlaylistTrack, 0, len(tracks))
	for _, t := range tracks {
		t.Artist = strings.Trim(strings.TrimSpace(t.Artist), "\"'")
		t.Title = strings.Trim(strings.TrimSpace(t.Title), "\"'")
		t.Album = strings.TrimSpace(t.Album)
		if t.Title == "" {
			continue
		}
		key := normalize(t.Artist) + "|" + normalize(t.Title)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
		if len(out) == maxTracks {
			break
		}
	}
	return out
}
