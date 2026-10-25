// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package mediasync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlan(t *testing.T) {
	movie := func(id string, watched bool, ids ...string) Item {
		return Item{ID: id, Kind: kindMovie, Title: id, IDs: ids, Watched: watched}
	}
	episode := func(id string, watched bool, series string, season, ep int, ids ...string) Item {
		return Item{ID: id, Kind: kindEpisode, Series: "Show", SeriesIDs: []string{series}, Season: season, Episode: ep, IDs: ids, Watched: watched}
	}

	tests := []struct {
		name     string
		src, dst []Item
		want     []string // target IDs to mark
		notFound int
	}{
		{
			name: "movie matches on any shared ID",
			src:  []Item{movie("s1", true, "imdb:tt1", "tmdb:10")},
			dst:  []Item{movie("d1", false, "tmdb:10")},
			want: []string{"d1"},
		},
		{
			name: "already watched on target is skipped",
			src:  []Item{movie("s1", true, "imdb:tt1")},
			dst:  []Item{movie("d1", true, "imdb:tt1")},
		},
		{
			name: "unwatched source is ignored",
			src:  []Item{movie("s1", false, "imdb:tt1")},
			dst:  []Item{movie("d1", false, "imdb:tt1")},
		},
		{
			name:     "missing on target counts as not found",
			src:      []Item{movie("s1", true, "imdb:tt1"), movie("s2", true)},
			dst:      []Item{movie("d1", false, "imdb:tt9")},
			notFound: 2,
		},
		{
			name: "episode matches on series ID and numbers when it has no IDs",
			src:  []Item{episode("s1", true, "tvdb:100", 1, 2)},
			dst:  []Item{episode("d1", false, "tvdb:100", 1, 3), episode("d2", false, "tvdb:100", 1, 2)},
			want: []string{"d2"},
		},
		{
			name: "episode matches on its own ID",
			src:  []Item{episode("s1", true, "tvdb:100", 1, 2, "tvdb:5555")},
			dst:  []Item{episode("d1", false, "tvdb:999", 0, 0, "tvdb:5555")},
			want: []string{"d1"},
		},
		{
			name:     "movie and episode IDs never cross",
			src:      []Item{movie("s1", true, "tmdb:10")},
			dst:      []Item{episode("d1", false, "tvdb:1", 1, 1, "tmdb:10")},
			notFound: 1,
		},
		{
			name: "two source copies mark the target once",
			src:  []Item{movie("s1", true, "imdb:tt1"), movie("s2", true, "imdb:tt1")},
			dst:  []Item{movie("d1", false, "imdb:tt1")},
			want: []string{"d1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marks, notFound := plan(tt.src, tt.dst)
			got := []string{}
			for _, m := range marks {
				got = append(got, m.Target.ID)
			}
			if tt.want == nil {
				tt.want = []string{}
			}
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.notFound, notFound)
		})
	}
}

func TestProviderIDs(t *testing.T) {
	require.ElementsMatch(t, []string{"imdb:tt0133093", "tmdb:603"},
		jellyfinProviderIDs(map[string]string{"Imdb": "tt0133093", "Tmdb": "603", "TmdbCollection": "2344", "Tvdb": ""}))

	m := plexMetadata{}
	for _, g := range []string{"imdb://tt0133093", "tmdb://603", "plex://movie/5d776"} {
		m.GUID = append(m.GUID, struct {
			ID string `json:"id"`
		}{ID: g})
	}
	require.Equal(t, []string{"imdb:tt0133093", "tmdb:603"}, plexGUIDs(m))
}
