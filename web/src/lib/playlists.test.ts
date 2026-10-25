/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { applyComparePreset, buildPlaylistPrompt, compareChanges, compareStatus } from "@/lib/playlists"
import type { PlaylistCompareRow } from "@/types"
import { describe, expect, it } from "vitest"

const row = (key: string, local: boolean, remote: boolean, trackId?: string): PlaylistCompareRow =>
  ({ key, artist: "A", title: key, local, remote, trackId, keep: true })

const rows = [row("both", true, true, "1"), row("local", true, false, "2"), row("missing", true, false), row("remote", false, true, "3")]

describe("compareStatus", () => {
  it("names where each row is", () => {
    expect(rows.map(compareStatus)).toEqual(["both", "localOnly", "notFound", "remoteOnly"])
  })
})

describe("applyComparePreset", () => {
  it("keeps the side the preset names", () => {
    const keep = (preset: Parameters<typeof applyComparePreset>[1]) => applyComparePreset(rows, preset).map(r => r.keep)
    expect(keep("everything")).toEqual([true, true, true, true])
    expect(keep("local")).toEqual([true, true, true, false])
    expect(keep("remote")).toEqual([true, false, false, true])
    expect(keep("both")).toEqual([true, false, false, false])
  })
})

describe("compareChanges", () => {
  it("counts the writes on each side", () => {
    expect(compareChanges(rows)).toEqual({ addToService: 1, removeFromService: 0, addToPlaylist: 1, removeFromPlaylist: 0 })
    expect(compareChanges(applyComparePreset(rows, "both"))).toEqual({ addToService: 0, removeFromService: 1, addToPlaylist: 0, removeFromPlaylist: 2 })
  })
})

describe("buildPlaylistPrompt", () => {
  const t = (key: string, options?: Record<string, unknown>) => `${key}${options ? JSON.stringify(options) : ""}`
  it("fills the request, count and library choice", () => {
    const prompt = buildPlaylistPrompt({ request: " rainy day ", count: 25, onlyLibrary: true }, t)
    expect(prompt).toContain("\"request\":\"rainy day\"")
    expect(prompt).toContain("\"count\":25")
    expect(prompt).toContain("playlists.prompt.sourceLibrary")
  })
})
