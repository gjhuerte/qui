/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

export const PLAYLIST_SERVICES = ["jellyfin", "plex", "spotify", "ytmusic"] as const
export type PlaylistService = (typeof PLAYLIST_SERVICES)[number]

export interface PlaylistTrack {
  artist: string
  title: string
  album?: string
}

/** The copy of a playlist on one service. remoteId is empty until the first sync. */
export interface PlaylistLink {
  id: number
  playlistId: number
  service: PlaylistService
  serverId?: number
  accountId?: string
  accountName?: string
  remoteId?: string
  remoteName?: string
  matched: number
  missing: number
  message?: string
  lastSyncedAt?: string
}

export interface Playlist {
  id: number
  name: string
  description: string
  request?: string
  trackCount: number
  /** Only on the single-playlist endpoints. */
  tracks?: PlaylistTrack[]
  links: PlaylistLink[]
  createdAt: string
  updatedAt: string
}

/** A service account: for Jellyfin and Plex a server and an account on it. */
export interface PlaylistAccount {
  service: PlaylistService
  serverId?: number
  accountId?: string
}

export interface PlaylistLinkInput extends PlaylistAccount {
  accountName?: string
  remoteId?: string
  remoteName?: string
}

export interface RemotePlaylist {
  id: string
  name: string
  count: number
}

export interface PlaylistCompareRow {
  key: string
  artist: string
  title: string
  album?: string
  /** Empty when the service does not have the track. */
  trackId?: string
  entryId?: string
  local: boolean
  remote: boolean
  keep: boolean
}

export interface PlaylistSyncResult {
  link: PlaylistLink
  missing: PlaylistTrack[]
}

export interface PlaylistServicesStatus {
  spotify: {
    clientId?: string
    hasSecret: boolean
    connected: boolean
    accountName?: string
    redirectUri: string
  }
  ytmusic: {
    connected: boolean
    accountName?: string
  }
}
