/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { AppMovie, AppSeries, AppStatus, AppWantedSubtitle } from "@/types"

export type MovieFilter = "all" | "downloaded" | "missing" | "unreleased" | "unmonitored"
export type SeriesFilter = "all" | "missing" | "complete" | "continuing" | "ended" | "unmonitored"
export type SubtitleFilter = "all" | "episode" | "movie"

/** "all" or the id of one instance. */
export type InstanceFilter = "all" | number

function matches(text: string, needle: string): boolean {
  return needle === "" || text.toLowerCase().includes(needle)
}

function inInstance(item: { instance: { id: number } }, instance: InstanceFilter): boolean {
  return instance === "all" || item.instance.id === instance
}

/** A monitored movie without a file that is already out: what Radarr lists as missing. */
export function movieState(m: AppMovie): "downloaded" | "missing" | "unreleased" | "unmonitored" {
  if (m.hasFile) return "downloaded"
  if (!m.monitored) return "unmonitored"
  return m.isAvailable ? "missing" : "unreleased"
}

export function filterMovies(items: AppMovie[], filter: MovieFilter, instance: InstanceFilter, search: string): AppMovie[] {
  const needle = search.trim().toLowerCase()
  return items.filter(m =>
    inInstance(m, instance) &&
    (filter === "all" || movieState(m) === filter) &&
    matches(`${m.title} ${m.year ?? ""} ${m.imdbId ?? ""}`, needle)
  )
}

export function filterSeries(items: AppSeries[], filter: SeriesFilter, instance: InstanceFilter, search: string): AppSeries[] {
  const needle = search.trim().toLowerCase()
  return items.filter(s => {
    if (!inInstance(s, instance) || !matches(`${s.title} ${s.year ?? ""} ${s.network ?? ""}`, needle)) return false
    switch (filter) {
      case "missing": return s.monitored && s.missing > 0
      case "complete": return s.missing === 0 && s.episodes > 0
      case "continuing": return s.status === "continuing"
      case "ended": return s.status === "ended"
      case "unmonitored": return !s.monitored
      default: return true
    }
  })
}

export function filterSubtitles(items: AppWantedSubtitle[], filter: SubtitleFilter, instance: InstanceFilter, search: string): AppWantedSubtitle[] {
  const needle = search.trim().toLowerCase()
  return items.filter(w =>
    inInstance(w, instance) &&
    (filter === "all" || w.kind === filter) &&
    matches(`${w.title} ${w.episode ?? ""} ${w.languages.join(" ")}`, needle)
  )
}

export type IssueSeverity = "error" | "warning"
export type IssueKind = "offline" | "health" | "download" | "indexer" | "partial"

export interface AppIssue {
  severity: IssueSeverity
  kind: IssueKind
  app: AppStatus["instance"]
  title: string
  detail?: string
}

/** Everything that needs attention across all apps, errors first. */
export function collectIssues(statuses: AppStatus[]): AppIssue[] {
  const issues: AppIssue[] = []
  for (const st of statuses) {
    const app = st.instance
    if (!st.online) {
      issues.push({ severity: "error", kind: "offline", app, title: st.error ?? "" })
      continue
    }
    for (const h of st.health) {
      if (h.level === "ok" || h.level === "notice") continue
      issues.push({ severity: h.level === "error" ? "error" : "warning", kind: "health", app, title: h.message, detail: h.source })
    }
    for (const q of st.queueIssues ?? []) {
      issues.push({ severity: "warning", kind: "download", app, title: q.title, detail: q.messages?.join(" · ") })
    }
    for (const name of st.indexers?.failing ?? []) {
      issues.push({ severity: "warning", kind: "indexer", app, title: name })
    }
    for (const w of st.warnings ?? []) {
      issues.push({ severity: "warning", kind: "partial", app, title: w })
    }
  }
  return issues.sort((a, b) => (a.severity === b.severity ? 0 : a.severity === "error" ? -1 : 1))
}
