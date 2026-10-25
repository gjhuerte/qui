/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { LibraryShell } from "@/components/services/LibraryShell"
import { ToneBadge } from "@/components/services/shared"
import type { ServicesTab } from "@/components/services/tabs"
import { Badge } from "@/components/ui/badge"
import { Progress } from "@/components/ui/progress"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { PAGE_SIZE, useLibraryControls, useLibraryRefresh } from "@/hooks/useLibraryControls"
import { appKeys, useAppMovies, useAppSeries, useAppSubtitles } from "@/hooks/useServices"
import { api } from "@/lib/api"
import { formatRelativeTime } from "@/lib/dateTimeUtils"
import {
  filterMovies,
  filterSeries,
  filterSubtitles,
  movieState,
  type MovieFilter,
  type SeriesFilter,
  type SubtitleFilter
} from "@/lib/media-library"
import type { Tone } from "@/lib/services-tone"
import { formatBytes } from "@/lib/utils"
import { useQueryClient } from "@tanstack/react-query"
import { ExternalLink } from "lucide-react"
import { useMemo, type ReactNode } from "react"
import { useTranslation } from "react-i18next"

const MOVIE_FILTERS: MovieFilter[] = ["all", "missing", "downloaded", "unreleased", "unmonitored"]
const SERIES_FILTERS: SeriesFilter[] = ["all", "missing", "complete", "continuing", "ended", "unmonitored"]
const SUBTITLE_FILTERS: SubtitleFilter[] = ["all", "episode", "movie"]

const movieTone: Record<ReturnType<typeof movieState>, Tone> = {
  downloaded: "good",
  missing: "bad",
  unreleased: "neutral",
  unmonitored: "neutral",
}

/** The title links to the item's page in its app. */
function TitleLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a href={href} target="_blank" rel="noreferrer" className="group inline-flex items-center gap-1 font-medium hover:underline">
      {children}
      <ExternalLink className="h-3 w-3 shrink-0 opacity-0 transition-opacity group-hover:opacity-60" />
    </a>
  )
}

function InstanceName({ name, show }: { name: string; show: boolean }) {
  return show ? <Badge variant="outline" className="font-normal">{name}</Badge> : null
}

export function MoviesView({ onNavigate }: { onNavigate: (tab: ServicesTab) => void }) {
  const { t } = useTranslation("services")
  const queryClient = useQueryClient()
  const { data, isLoading, error } = useAppMovies()
  const controls = useLibraryControls<MovieFilter>("all")
  const { refreshing, refresh } = useLibraryRefresh(async () => queryClient.setQueryData(appKeys.movies, await api.getAppMovies(true)))
  const filtered = useMemo(
    () => filterMovies(data?.items ?? [], controls.filter, controls.instance, controls.search),
    [data, controls.filter, controls.instance, controls.search]
  )
  const pageItems = filtered.slice(controls.page * PAGE_SIZE, (controls.page + 1) * PAGE_SIZE)
  const multi = (data?.instances.length ?? 0) > 1

  return (
    <LibraryShell
      title={t("library.movies.title")}
      description={t("library.movies.description")}
      library={data}
      isLoading={isLoading}
      error={error}
      refreshing={refreshing}
      onRefresh={refresh}
      noInstancesText={t("library.movies.noInstances")}
      onNavigate={onNavigate}
      filters={MOVIE_FILTERS.map(f => ({ value: f, label: t(`library.movies.filters.${f}`) }))}
      shown={filtered.length}
      {...controls}
    >
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("library.columns.title")}</TableHead>
            <TableHead>{t("library.columns.status")}</TableHead>
            <TableHead className="hidden md:table-cell">{t("library.columns.quality")}</TableHead>
            <TableHead className="hidden md:table-cell text-right">{t("library.columns.size")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {pageItems.map(m => {
            const state = movieState(m)
            return (
              <TableRow key={`${m.instance.id}-${m.id}`}>
                <TableCell>
                  <div className="flex flex-wrap items-center gap-2">
                    <TitleLink href={m.link}>{m.title}</TitleLink>
                    {m.year ? <span className="text-xs text-muted-foreground">{m.year}</span> : null}
                    <InstanceName name={m.instance.name} show={multi} />
                  </div>
                  {state === "unreleased" && m.release && (
                    <p className="text-xs text-muted-foreground">{t("library.movies.releases", { when: formatRelativeTime(m.release) })}</p>
                  )}
                </TableCell>
                <TableCell><ToneBadge tone={movieTone[state]}>{t(`library.movies.states.${state}`)}</ToneBadge></TableCell>
                <TableCell className="hidden md:table-cell text-xs">{m.quality || "—"}</TableCell>
                <TableCell className="hidden md:table-cell text-right text-xs">{m.sizeOnDisk > 0 ? formatBytes(m.sizeOnDisk) : "—"}</TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
      {filtered.length === 0 && <p className="text-sm text-muted-foreground">{t("library.noMatches")}</p>}
    </LibraryShell>
  )
}

export function SeriesView({ onNavigate }: { onNavigate: (tab: ServicesTab) => void }) {
  const { t } = useTranslation("services")
  const queryClient = useQueryClient()
  const { data, isLoading, error } = useAppSeries()
  const controls = useLibraryControls<SeriesFilter>("all")
  const { refreshing, refresh } = useLibraryRefresh(async () => queryClient.setQueryData(appKeys.series, await api.getAppSeries(true)))
  const filtered = useMemo(
    () => filterSeries(data?.items ?? [], controls.filter, controls.instance, controls.search),
    [data, controls.filter, controls.instance, controls.search]
  )
  const pageItems = filtered.slice(controls.page * PAGE_SIZE, (controls.page + 1) * PAGE_SIZE)
  const multi = (data?.instances.length ?? 0) > 1

  return (
    <LibraryShell
      title={t("library.series.title")}
      description={t("library.series.description")}
      library={data}
      isLoading={isLoading}
      error={error}
      refreshing={refreshing}
      onRefresh={refresh}
      noInstancesText={t("library.series.noInstances")}
      onNavigate={onNavigate}
      filters={SERIES_FILTERS.map(f => ({ value: f, label: t(`library.series.filters.${f}`) }))}
      shown={filtered.length}
      {...controls}
    >
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("library.columns.title")}</TableHead>
            <TableHead className="w-48">{t("library.columns.episodes")}</TableHead>
            <TableHead className="hidden md:table-cell">{t("library.columns.next")}</TableHead>
            <TableHead className="hidden md:table-cell text-right">{t("library.columns.size")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {pageItems.map(s => (
            <TableRow key={`${s.instance.id}-${s.id}`}>
              <TableCell>
                <div className="flex flex-wrap items-center gap-2">
                  <TitleLink href={s.link}>{s.title}</TitleLink>
                  {s.year ? <span className="text-xs text-muted-foreground">{s.year}</span> : null}
                  <InstanceName name={s.instance.name} show={multi} />
                </div>
                <p className="text-xs text-muted-foreground">
                  {[s.network, t([`library.series.states.${s.status}`, "library.series.states.unknown"]), t("library.series.seasons", { count: s.seasons }), !s.monitored ? t("library.series.unmonitored") : ""].filter(Boolean).join(" · ")}
                </p>
              </TableCell>
              <TableCell>
                <div className="space-y-1">
                  <div className="flex justify-between gap-2 text-xs">
                    <span>{t("library.series.episodeCount", { files: s.episodeFiles, episodes: s.episodes })}</span>
                    {s.missing > 0 && s.monitored && <span className="text-destructive">{t("library.series.missing", { count: s.missing })}</span>}
                  </div>
                  <Progress value={s.episodes > 0 ? (s.episodeFiles / s.episodes) * 100 : 0} className="h-1.5" />
                </div>
              </TableCell>
              <TableCell className="hidden md:table-cell text-xs">{s.nextAiring ? formatRelativeTime(s.nextAiring) : "—"}</TableCell>
              <TableCell className="hidden md:table-cell text-right text-xs">{s.sizeOnDisk > 0 ? formatBytes(s.sizeOnDisk) : "—"}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {filtered.length === 0 && <p className="text-sm text-muted-foreground">{t("library.noMatches")}</p>}
    </LibraryShell>
  )
}

export function SubtitlesView({ onNavigate }: { onNavigate: (tab: ServicesTab) => void }) {
  const { t } = useTranslation("services")
  const queryClient = useQueryClient()
  const { data, isLoading, error } = useAppSubtitles()
  const controls = useLibraryControls<SubtitleFilter>("all")
  const { refreshing, refresh } = useLibraryRefresh(async () => queryClient.setQueryData(appKeys.subtitles, await api.getAppSubtitles(true)))
  const filtered = useMemo(
    () => filterSubtitles(data?.items ?? [], controls.filter, controls.instance, controls.search),
    [data, controls.filter, controls.instance, controls.search]
  )
  const pageItems = filtered.slice(controls.page * PAGE_SIZE, (controls.page + 1) * PAGE_SIZE)
  const multi = (data?.instances.length ?? 0) > 1

  return (
    <LibraryShell
      title={t("library.subtitles.title")}
      description={t("library.subtitles.description")}
      library={data}
      isLoading={isLoading}
      error={error}
      refreshing={refreshing}
      onRefresh={refresh}
      noInstancesText={t("library.subtitles.noInstances")}
      onNavigate={onNavigate}
      filters={SUBTITLE_FILTERS.map(f => ({ value: f, label: t(`library.subtitles.filters.${f}`) }))}
      shown={filtered.length}
      {...controls}
    >
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("library.columns.title")}</TableHead>
            <TableHead>{t("library.columns.missingLanguages")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {pageItems.map((w, i) => (
            <TableRow key={`${w.instance.id}-${w.kind}-${w.title}-${w.episode ?? ""}-${i}`}>
              <TableCell>
                <div className="flex flex-wrap items-center gap-2">
                  <TitleLink href={w.link}>{w.title}</TitleLink>
                  <Badge variant="secondary" className="font-normal">{t(`library.subtitles.kinds.${w.kind}`)}</Badge>
                  <InstanceName name={w.instance.name} show={multi} />
                </div>
                {w.episode && <p className="text-xs text-muted-foreground">{w.episode}</p>}
              </TableCell>
              <TableCell>
                <div className="flex flex-wrap gap-1">
                  {w.languages.map(l => <Badge key={l} variant="outline" className="font-normal">{l}</Badge>)}
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {filtered.length === 0 && <p className="text-sm text-muted-foreground">{t("library.noMatches")}</p>}
    </LibraryShell>
  )
}
