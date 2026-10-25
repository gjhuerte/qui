// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gjhuerte/qui/internal/models"
)

func TestParseAIAnswer(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantName string
		want     []models.PlaylistTrack
	}{
		{
			name: "json in a code fence with chatter around it",
			text: "Here is your playlist!\n```json\n{\"name\": \"Rainy Sunday\", \"description\": \"Slow songs\", \"tracks\": [" +
				"{\"artist\": \"Nora Vale\", \"title\": \"Grey Windows\", \"album\": \"Weather\"}," +
				"{\"artist\": \"The Lanterns\", \"title\": \"Puddles\"}]}\n```\nEnjoy!",
			wantName: "Rainy Sunday",
			want: []models.PlaylistTrack{
				{Artist: "Nora Vale", Title: "Grey Windows", Album: "Weather"},
				{Artist: "The Lanterns", Title: "Puddles"},
			},
		},
		{
			name: "numbered lines with dashes",
			text: "1. Nora Vale - Grey Windows\n2) The Lanterns – Puddles\n- Kite Club — Summer Static\nnot a track line",
			want: []models.PlaylistTrack{
				{Artist: "Nora Vale", Title: "Grey Windows"},
				{Artist: "The Lanterns", Title: "Puddles"},
				{Artist: "Kite Club", Title: "Summer Static"},
			},
		},
		{
			name: "title by artist, duplicates dropped",
			text: "\"Grey Windows\" by Nora Vale\nNora Vale - Grey Windows",
			want: []models.PlaylistTrack{{Artist: "Nora Vale", Title: "Grey Windows"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAIAnswer(tt.text)
			require.NoError(t, err)
			require.Equal(t, tt.wantName, got.Name)
			require.Equal(t, tt.want, got.Tracks)
		})
	}

	_, err := ParseAIAnswer("I could not find any songs, sorry.")
	require.ErrorIs(t, err, ErrInvalid)
	_, err = ParseAIAnswer("   ")
	require.ErrorIs(t, err, ErrInvalid)
}

func TestNameFromRequest(t *testing.T) {
	require.Equal(t, "Upbeat indie for a morning run", nameFromRequest("  Upbeat indie\nfor a morning run "))
	require.Equal(t, "Slow songs for a long rainy Sunday afternoon with tea and a…",
		nameFromRequest("Slow songs for a long rainy Sunday afternoon with tea and a book by the window"))
}
