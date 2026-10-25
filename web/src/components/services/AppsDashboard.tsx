/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { InfoGrid, InfoItem, ToneBadge } from "@/components/services/shared"
import type { ServicesTab } from "@/components/services/tabs"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import { appKeys, useAppStatus } from "@/hooks/useServices"
import { api } from "@/lib/api"
import { formatRelativeTime } from "@/lib/dateTimeUtils"
import { collectIssues } from "@/lib/media-library"
import { formatBytes, cn } from "@/lib/utils"
import type { AppStatus } from "@/types"
import { useQueryClient } from "@tanstack/react-query"
import { AlertTriangle, ExternalLink, Loader2, RefreshCw } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

export function AppsDashboard({ onNavigate }: { onNavigate: (tab: ServicesTab) => void }) {
  const { t } = useTranslation("services")
  const queryClient = useQueryClient()
  const { data, isLoading } = useAppStatus()
  const [refreshing, setRefreshing] = useState(false)

  const refresh = async () => {
    setRefreshing(true)
    try {
      queryClient.setQueryData(appKeys.status, await api.getAppStatus(true))
    } finally {
      setRefreshing(false)
    }
  }

  if (isLoading) {
    return <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
  }

  if (!data || data.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("apps.empty.title")}</CardTitle>
          <CardDescription>{t("apps.empty.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button onClick={() => onNavigate("apps-manage")}>{t("apps.actions.add")}</Button>
        </CardContent>
      </Card>
    )
  }

  const offline = data.filter(s => !s.online).length
  const issueCount = collectIssues(data).length

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <ToneBadge tone={offline === 0 ? "good" : "bad"}>
          {offline === 0 ? t("apps.summary.allOnline", { count: data.length }) : t("apps.summary.offline", { count: offline })}
        </ToneBadge>
        {issueCount > 0 && (
          <Button variant="link" size="sm" className="h-auto px-0" onClick={() => onNavigate("apps-issues")}>
            {t("apps.summary.issues", { count: issueCount })}
          </Button>
        )}
        <Button variant="ghost" size="sm" className="ml-auto" onClick={refresh} disabled={refreshing}>
          <RefreshCw className={cn("mr-2 h-4 w-4", refreshing && "animate-spin")} />
          {t("actions.refresh")}
        </Button>
      </div>
      <div className="grid items-start gap-4 xl:grid-cols-2">
        {data.map(status => <AppCard key={status.instance.id} status={status} />)}
      </div>
    </div>
  )
}

function AppCard({ status }: { status: AppStatus }) {
  const { t } = useTranslation("services")
  const inst = status.instance
  const problems = status.health.filter(h => h.level !== "ok" && h.level !== "notice")

  return (
    <Card className="min-w-0">
      <CardHeader>
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <CardTitle className="truncate">{inst.name}</CardTitle>
            <CardDescription>
              {t(`apps.types.${inst.type}`)}
              {status.version && <span> · {t("apps.version", { version: status.version })}</span>}
            </CardDescription>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            <ToneBadge tone={!status.online ? "bad" : problems.length > 0 ? "warn" : "good"}>
              {!status.online ? t("apps.offline") : problems.length > 0 ? t("apps.issues", { count: problems.length }) : t("apps.online")}
            </ToneBadge>
            <Button asChild variant="ghost" size="icon" className="h-8 w-8" aria-label={t("apps.actions.open")}>
              <a href={inst.baseUrl} target="_blank" rel="noreferrer"><ExternalLink className="h-4 w-4" /></a>
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {!status.online ? (
          <p className="text-sm text-destructive break-words">{status.error}</p>
        ) : (
          <>
            <Summary status={status} />
            {problems.length > 0 && (
              <ul className="space-y-1 text-xs">
                {problems.map((h, i) => (
                  <li key={i} className="flex gap-2">
                    <AlertTriangle className={cn("mt-0.5 h-3.5 w-3.5 shrink-0", h.level === "error" ? "text-destructive" : "text-amber-600 dark:text-amber-400")} />
                    <span className="break-words">{h.message}</span>
                  </li>
                ))}
              </ul>
            )}
            <QueueList status={status} />
            <StreamList status={status} />
            <UpcomingList status={status} />
            <DiskList status={status} />
            {status.indexers && status.indexers.failing.length > 0 && (
              <p className="text-xs text-destructive">{t("apps.indexers.failingList", { names: status.indexers.failing.join(", ") })}</p>
            )}
            {status.warnings && status.warnings.length > 0 && (
              <p className="text-xs text-muted-foreground break-words">{t("apps.partial", { errors: status.warnings.join("; ") })}</p>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}

function Summary({ status }: { status: AppStatus }) {
  const { t } = useTranslation("services")
  return (
    <InfoGrid>
      {status.queue && <InfoItem label={t("apps.queue.label")}>{status.queue.total}</InfoItem>}
      {status.missing !== undefined && <InfoItem label={t("apps.missing")}>{status.missing}</InfoItem>}
      {status.indexers && (
        <>
          <InfoItem label={t("apps.indexers.enabled")}>{t("apps.indexers.enabledValue", { enabled: status.indexers.enabled, total: status.indexers.total })}</InfoItem>
          <InfoItem label={t("apps.indexers.failing")}>{status.indexers.failing.length}</InfoItem>
          <InfoItem label={t("apps.indexers.grabs")}>{t("apps.indexers.grabsValue", { grabs: status.indexers.grabs, queries: status.indexers.queries })}</InfoItem>
        </>
      )}
      {status.subtitles && (
        <>
          <InfoItem label={t("apps.subtitles.episodes")}>{status.subtitles.wantedEpisodes}</InfoItem>
          <InfoItem label={t("apps.subtitles.movies")}>{status.subtitles.wantedMovies}</InfoItem>
        </>
      )}
      {status.streams && (
        <>
          <InfoItem label={t("apps.streams.count")}>{status.streams.count}</InfoItem>
          <InfoItem label={t("apps.streams.bandwidth")}>{t("apps.streams.bandwidthValue", { mbps: (status.streams.bandwidth / 1000).toFixed(1) })}</InfoItem>
        </>
      )}
    </InfoGrid>
  )
}

function QueueList({ status }: { status: AppStatus }) {
  const { t } = useTranslation("services")
  const items = status.queue?.items ?? []
  if (items.length === 0) return null
  return (
    <div className="space-y-2">
      <h3 className="text-sm font-semibold">{t("apps.queue.title")}</h3>
      <ul className="space-y-2">
        {items.map((item, i) => (
          <li key={i} className="space-y-1">
            <div className="flex items-center justify-between gap-2 text-xs">
              <span className={cn("truncate", item.warning && "text-amber-600 dark:text-amber-400")} title={item.title}>{item.title}</span>
              <span className="shrink-0 text-muted-foreground">
                {Math.round(item.progress * 100)}%{item.timeLeft ? ` · ${item.timeLeft}` : ""}
              </span>
            </div>
            <Progress value={item.progress * 100} className="h-1.5" />
          </li>
        ))}
      </ul>
    </div>
  )
}

function StreamList({ status }: { status: AppStatus }) {
  const { t } = useTranslation("services")
  const sessions = status.streams?.sessions ?? []
  if (sessions.length === 0) return null
  return (
    <div className="space-y-2">
      <h3 className="text-sm font-semibold">{t("apps.streams.title")}</h3>
      <ul className="space-y-2">
        {sessions.map((s, i) => (
          <li key={i} className="space-y-1">
            <div className="flex items-center justify-between gap-2 text-xs">
              <span className="truncate" title={s.title}><span className="font-medium">{s.user}</span> · {s.title}</span>
              <span className="shrink-0 text-muted-foreground">
                {t([`apps.streams.states.${s.state}`, "apps.streams.states.unknown"])}
                {s.decision && <> · {t([`apps.streams.decisions.${s.decision}`, "apps.streams.decisions.unknown"])}</>}
              </span>
            </div>
            <Progress value={s.progress * 100} className="h-1.5" />
            {(s.player || s.quality) && (
              <p className="text-xs text-muted-foreground">{[s.player, s.quality].filter(Boolean).join(" · ")}</p>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}

function UpcomingList({ status }: { status: AppStatus }) {
  const { t } = useTranslation("services")
  const items = status.upcoming ?? []
  if (items.length === 0) return null
  return (
    <div className="space-y-2">
      <h3 className="text-sm font-semibold">{t("apps.upcoming")}</h3>
      <ul className="space-y-1 text-xs">
        {items.map((u, i) => (
          <li key={i} className="flex justify-between gap-2">
            <span className="truncate" title={u.subtitle ? `${u.title} · ${u.subtitle}` : u.title}>
              {u.hasFile && <span className="text-emerald-600 dark:text-emerald-400">✓ </span>}
              <span className="font-medium">{u.title}</span>
              {u.subtitle && <span className="text-muted-foreground"> · {u.subtitle}</span>}
            </span>
            <span className="shrink-0 text-muted-foreground">{formatRelativeTime(u.date)}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}

function DiskList({ status }: { status: AppStatus }) {
  const { t } = useTranslation("services")
  const disks = status.disks ?? []
  if (disks.length === 0) return null
  return (
    <div className="space-y-2">
      <h3 className="text-sm font-semibold">{t("apps.disks")}</h3>
      <ul className="space-y-2">
        {disks.map(d => {
          const used = d.total > 0 ? (d.total - d.free) / d.total : 0
          return (
            <li key={d.path} className="space-y-1">
              <div className="flex justify-between gap-2 text-xs">
                <span className="truncate font-mono">{d.path}</span>
                <span className="shrink-0 text-muted-foreground">{t("apps.diskFree", { free: formatBytes(d.free), total: formatBytes(d.total) })}</span>
              </div>
              <Progress value={used * 100} className={cn("h-1.5", used > 0.9 && "[&>div]:bg-destructive")} />
            </li>
          )
        })}
      </ul>
    </div>
  )
}
