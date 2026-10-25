/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { describe, expect, it } from "vitest"

import { collectIssues, filterMovies, filterSeries, filterSubtitles, movieState } from "@/lib/media-library"
import type { AppMovie, AppSeries, AppStatus, AppWantedSubtitle } from "@/types"

const radarr = { id: 1, name: "Radarr" }
const radarr4k = { id: 2, name: "Radarr 4K" }

const movie = (over: Partial<AppMovie>): AppMovie => ({
  instance: radarr, id: 1, title: "Film", monitored: true, hasFile: false, isAvailable: true, sizeOnDisk: 0, link: "", ...over,
})

describe("movies", () => {
  const movies = [
    movie({ id: 1, title: "Have It", hasFile: true }),
    movie({ id: 2, title: "Want It" }),
    movie({ id: 3, title: "Soon", isAvailable: false }),
    movie({ id: 4, title: "Ignored", monitored: false }),
    movie({ id: 5, title: "Want It Too", instance: radarr4k, imdbId: "tt123" }),
  ]

  it("names the state", () => {
    expect(movies.map(movieState)).toEqual(["downloaded", "missing", "unreleased", "unmonitored", "missing"])
  })

  it("filters by state, instance and search", () => {
    expect(filterMovies(movies, "missing", "all", "").map(m => m.id)).toEqual([2, 5])
    expect(filterMovies(movies, "missing", 2, "").map(m => m.id)).toEqual([5])
    expect(filterMovies(movies, "all", "all", "TT123").map(m => m.id)).toEqual([5])
  })
})

describe("series", () => {
  const series = (over: Partial<AppSeries>): AppSeries => ({
    instance: { id: 3, name: "Sonarr" }, id: 1, title: "Show", monitored: true, seasons: 1, episodeFiles: 10, episodes: 10,
    totalEpisodes: 10, missing: 0, sizeOnDisk: 0, link: "", status: "ended", ...over,
  })
  const list = [series({ id: 1 }), series({ id: 2, missing: 3, status: "continuing" }), series({ id: 3, missing: 3, monitored: false })]

  it("filters missing only for monitored series", () => {
    expect(filterSeries(list, "missing", "all", "").map(s => s.id)).toEqual([2])
    expect(filterSeries(list, "complete", "all", "").map(s => s.id)).toEqual([1])
    expect(filterSeries(list, "continuing", "all", "").map(s => s.id)).toEqual([2])
  })
})

describe("subtitles", () => {
  const items: AppWantedSubtitle[] = [
    { instance: { id: 4, name: "Bazarr" }, kind: "episode", title: "Show", episode: "1x02", languages: ["English"], link: "" },
    { instance: { id: 4, name: "Bazarr" }, kind: "movie", title: "Film", languages: ["Spanish"], link: "" },
  ]

  it("filters by kind and language", () => {
    expect(filterSubtitles(items, "movie", "all", "")).toHaveLength(1)
    expect(filterSubtitles(items, "all", "all", "english")[0].title).toBe("Show")
  })
})

describe("collectIssues", () => {
  const instance = (id: number) => ({ id, type: "sonarr" as const, name: `App ${id}`, baseUrl: "", enabled: true, createdAt: "", updatedAt: "" })

  it("lists errors first and skips notices", () => {
    const statuses: AppStatus[] = [
      {
        instance: instance(1), online: true, checkedAt: "",
        health: [{ level: "notice", message: "update" }, { level: "warning", message: "indexers down" }],
        queueIssues: [{ title: "stuck", progress: 1, messages: ["No files found"] }],
      },
      { instance: instance(2), online: false, error: "timeout", checkedAt: "", health: [] },
    ]
    const issues = collectIssues(statuses)
    expect(issues.map(i => i.kind)).toEqual(["offline", "health", "download"])
    expect(issues[2].detail).toBe("No files found")
  })
})
