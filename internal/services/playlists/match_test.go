// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/models"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"Grey Windows (2011 Remaster)":     "grey windows",
		"Grey Windows - Remastered 2011":   "grey windows",
		"Puddles [Live]":                   "puddles",
		"Summer Static feat. Someone Else": "summer static",
		"The Lanterns":                     "lanterns",
		"Salt & Pepper":                    "salt and pepper",
		"Beyoncé!":                         "beyoncé",
	} {
		require.Equal(t, want, normalize(in), in)
	}
}

func TestBest(t *testing.T) {
	track := models.PlaylistTrack{Artist: "Nora Vale", Title: "Grey Windows"}
	tests := []struct {
		name       string
		candidates []Song
		want       string
	}{
		{"exact title and artist", []Song{{ID: "1", Title: "Grey Windows", Artists: []string{"Nora Vale"}}}, "1"},
		{"version suffix and second artist", []Song{{ID: "2", Title: "Grey Windows - 2020 Remaster", Artists: []string{"Kite Club", "Nora Vale"}}}, "2"},
		{"wrong artist is not the track", []Song{{ID: "3", Title: "Grey Windows", Artists: []string{"Cover Band"}}}, ""},
		{"exact title wins over a longer one", []Song{
			{ID: "4", Title: "Grey Windows Reprise", Artists: []string{"Nora Vale"}},
			{ID: "5", Title: "Grey Windows", Artists: []string{"Nora Vale"}},
		}, "5"},
		{"a longer title is the fallback", []Song{{ID: "6", Title: "Grey Windows Reprise", Artists: []string{"Nora Vale"}}}, "6"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := best(track, tt.candidates)
			require.Equal(t, tt.want, got.ID)
		})
	}
}

func TestFormatLibrary(t *testing.T) {
	text := FormatLibrary("Home", []Song{
		{Title: "Puddles", Artists: []string{"The Lanterns"}, Album: "Wet"},
		{Title: "Grey Windows", Artists: []string{"Nora Vale"}, Album: "Weather"},
		{Title: "Fog", Artists: []string{"Nora Vale"}, Album: "Weather"},
		{Title: "Loose", Artists: []string{"Nora Vale"}},
	})
	require.True(t, strings.HasPrefix(text, "# Music library of Home: 4 tracks by 2 artists\n"))
	require.Contains(t, text, "Nora Vale\n  Singles: Loose\n  Weather: Grey Windows | Fog\nThe Lanterns\n  Wet: Puddles\n")
}
