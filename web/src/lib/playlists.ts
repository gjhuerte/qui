/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { PlaylistCompareRow } from "@/types"

// i18n-namespace: services

type Translate = (key: string, options?: Record<string, unknown>) => string

export interface PromptInput {
  request: string
  count: number
  /** Pick only songs from the exported library file. */
  onlyLibrary: boolean
}

/** The prompt the user sends to an AI assistant. It asks for the JSON that the import reads. */
export function buildPlaylistPrompt(input: PromptInput, t: Translate): string {
  return t("playlists.prompt.text", {
    request: input.request.trim() || t("playlists.prompt.noRequest"),
    count: input.count,
    source: input.onlyLibrary ? t("playlists.prompt.sourceLibrary") : t("playlists.prompt.sourceAny"),
    spelling: input.onlyLibrary ? t("playlists.prompt.spellingLibrary") : t("playlists.prompt.spellingAny"),
  })
}

export type CompareStatus = "both" | "localOnly" | "remoteOnly" | "notFound"

/** Where a compare row is now. notFound: in the playlist, but the service does not have the track. */
export function compareStatus(row: PlaylistCompareRow): CompareStatus {
  if (row.local && row.remote) return "both"
  if (row.remote) return "remoteOnly"
  return row.trackId ? "localOnly" : "notFound"
}

export type ComparePreset = "everything" | "local" | "remote" | "both"

/**
 * Sets keep on every row: everything keeps all tracks on both sides; local
 * makes the service match the playlist; remote makes the playlist match the
 * service; both keeps only the tracks both sides have.
 */
export function applyComparePreset(rows: PlaylistCompareRow[], preset: ComparePreset): PlaylistCompareRow[] {
  return rows.map(row => {
    switch (preset) {
      case "local":
        return { ...row, keep: row.local }
      case "remote":
        return { ...row, keep: row.remote }
      case "both":
        return { ...row, keep: row.local && row.remote }
      default:
        return { ...row, keep: true }
    }
  })
}

export interface CompareChanges {
  addToService: number
  removeFromService: number
  addToPlaylist: number
  removeFromPlaylist: number
}

/** What applying the rows would change on each side. */
export function compareChanges(rows: PlaylistCompareRow[]): CompareChanges {
  const out: CompareChanges = { addToService: 0, removeFromService: 0, addToPlaylist: 0, removeFromPlaylist: 0 }
  for (const row of rows) {
    if (row.keep && !row.remote && row.trackId) out.addToService++
    if (!row.keep && row.remote) out.removeFromService++
    if (row.keep && !row.local) out.addToPlaylist++
    if (!row.keep && row.local) out.removeFromPlaylist++
  }
  return out
}
