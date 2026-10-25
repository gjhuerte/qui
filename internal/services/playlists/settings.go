// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package playlists

import (
	"context"
	"fmt"

	"github.com/gjhuerte/qui/internal/models"
)

// ServicesStatus is what the Playlist services page shows. Secrets never
// leave the server.
type ServicesStatus struct {
	Spotify struct {
		ClientID    string `json:"clientId,omitempty"`
		HasSecret   bool   `json:"hasSecret"`
		Connected   bool   `json:"connected"`
		AccountName string `json:"accountName,omitempty"`
		RedirectURI string `json:"redirectUri"`
	} `json:"spotify"`
	YTMusic struct {
		Connected   bool   `json:"connected"`
		AccountName string `json:"accountName,omitempty"`
	} `json:"ytmusic"`
}

func (s *Service) Services(ctx context.Context) (*ServicesStatus, error) {
	out := &ServicesStatus{}
	out.Spotify.RedirectURI = SpotifyRedirectURI

	settings, stored, err := s.loadSpotify(ctx)
	if err != nil {
		return nil, err
	}
	out.Spotify.ClientID = settings.ClientID
	out.Spotify.HasSecret = settings.ClientSecret != ""
	out.Spotify.Connected = settings.RefreshToken != ""
	if stored != nil && out.Spotify.Connected {
		out.Spotify.AccountName = stored.AccountName
	}

	yt, err := s.store.GetServiceSettings(ctx, models.PlaylistYTMusic)
	if err != nil {
		return nil, err
	}
	if yt != nil {
		out.YTMusic.Connected, out.YTMusic.AccountName = true, yt.AccountName
	}
	return out, nil
}

// Disconnect forgets the sign-in of Spotify or YouTube Music.
func (s *Service) Disconnect(ctx context.Context, service models.PlaylistService) error {
	if service != models.PlaylistSpotify && service != models.PlaylistYTMusic {
		return fmt.Errorf("%w: only Spotify and YouTube Music sign in here", ErrInvalid)
	}
	return s.store.DeleteServiceSettings(ctx, service)
}
