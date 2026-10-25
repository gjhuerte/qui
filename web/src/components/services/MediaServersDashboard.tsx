/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { InfoGrid, InfoItem, ToneBadge } from "@/components/services/shared"
import type { ServicesTab } from "@/components/services/tabs"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import { mediaSyncKeys, useMediaServerStatus, useMediaServers, useSyncPairs, useSyncRuns } from "@/hooks/useMediaSync"
import { api } from "@/lib/api"
import { formatRelativeTime } from "@/lib/dateTimeUtils"
import { latestRunByPair, pairLabel } from "@/lib/media-sync"
import { syncRunTone } from "@/lib/services-tone"
import { cn } from "@/lib/utils"
import type { MediaServerStatus } from "@/types"
import { useQueryClient } from "@tanstack/react-query"
import { ExternalLink, Loader2, Pause, Play, RefreshCw } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

export function MediaServersDashboard({ onNavigate }: { onNavigate: (tab: ServicesTab) => void }) {
  const { t } = useTranslation("services")
  const queryClient = useQueryClient()
  const { data, isLoading } = useMediaServerStatus()
  const [refreshing, setRefreshing] = useState(false)

  const refresh = async () => {
    setRefreshing(true)
    try {
      queryClient.setQueryData(mediaSyncKeys.status, await api.getMediaServerStatus(true))
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
          <CardTitle>{t("mediaSync.empty.title")}</CardTitle>
          <CardDescription>{t("mediaSync.empty.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button onClick={() => onNavigate("media-servers-manage")}>{t("mediaSync.actions.addServer")}</Button>
        </CardContent>
      </Card>
    )
  }

  const offline = data.filter(s => !s.online).length
  const streams = data.reduce((n, s) => n + s.sessions.length, 0)

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <ToneBadge tone={offline === 0 ? "good" : "bad"}>
          {offline === 0 ? t("mediaSync.summary.allOnline", { count: data.length }) : t("mediaSync.summary.offline", { count: offline })}
        </ToneBadge>
        <ToneBadge tone="neutral">{t("mediaSync.summary.streams", { count: streams })}</ToneBadge>
        <Button variant="ghost" size="sm" className="ml-auto" onClick={refresh} disabled={refreshing}>
          <RefreshCw className={cn("mr-2 h-4 w-4", refreshing && "animate-spin")} />
          {t("actions.refresh")}
        </Button>
      </div>
      <div className="grid items-start gap-4 xl:grid-cols-2">
        {data.map(status => <ServerCard key={status.server.id} status={status} />)}
      </div>
      <SyncSummaryCard onNavigate={onNavigate} />
    </div>
  )
}

function ServerCard({ status }: { status: MediaServerStatus }) {
  const { t } = useTranslation("services")
  const srv = status.server

  return (
    <Card className="min-w-0">
      <CardHeader>
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <CardTitle className="truncate">{srv.name}</CardTitle>
            <CardDescription>
              {t(`mediaSync.types.${srv.type}`)}
              {status.version && <span> · {t("apps.version", { version: status.version })}</span>}
              {status.serverName && <span> · {status.serverName}</span>}
              {status.platform && <span> · {status.platform}</span>}
            </CardDescription>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            <ToneBadge tone={status.online ? "good" : "bad"}>
              {status.online ? t("apps.online") : t("apps.offline")}
            </ToneBadge>
            <Button asChild variant="ghost" size="icon" className="h-8 w-8" aria-label={t("mediaSync.actions.open")}>
              <a href={srv.type === "plex" ? `${srv.baseUrl}/web` : srv.baseUrl} target="_blank" rel="noreferrer"><ExternalLink className="h-4 w-4" /></a>
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {!status.online ? (
          <p className="text-sm text-destructive break-words">{status.error}</p>
        ) : (
          <>
            <InfoGrid>
              <InfoItem label={t("mediaSync.status.movies")}>{status.movies.toLocaleString()}</InfoItem>
              <InfoItem label={t("mediaSync.status.series")}>{status.series.toLocaleString()}</InfoItem>
              <InfoItem label={t("mediaSync.status.episodes")}>{status.episodes.toLocaleString()}</InfoItem>
              {srv.type === "jellyfin" && <InfoItem label={t("mediaSync.status.users")}>{status.users}</InfoItem>}
              <InfoItem label={t("mediaSync.status.streams")}>{status.sessions.length}</InfoItem>
            </InfoGrid>
            {status.libraries.length > 0 && (
              <div className="space-y-1.5">
                <p className="text-xs text-muted-foreground">{t("mediaSync.status.libraries")}</p>
                <div className="flex flex-wrap gap-1.5">
                  {status.libraries.map(lib => (
                    <Badge key={lib.name} variant="secondary" className="font-normal">
                      {lib.name}
                      {lib.items !== undefined && <span className="ml-1 text-muted-foreground">{lib.items.toLocaleString()}</span>}
                    </Badge>
                  ))}
                </div>
              </div>
            )}
            <div className="space-y-2">
              <p className="text-xs text-muted-foreground">{t("mediaSync.status.nowPlaying")}</p>
              {status.sessions.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("mediaSync.status.nothingPlaying")}</p>
              ) : (
                <ul className="space-y-3">
                  {status.sessions.map((s, i) => (
                    <li key={i} className="space-y-1">
                      <div className="flex items-start justify-between gap-2 text-sm">
                        <div className="min-w-0">
                          <p className="truncate font-medium">{s.title}</p>
                          <p className="truncate text-xs text-muted-foreground">{s.user} · {s.client}</p>
                        </div>
                        <div className="flex shrink-0 items-center gap-1">
                          {s.transcoding && <ToneBadge tone="warn">{t("mediaSync.status.transcoding")}</ToneBadge>}
                          {s.state === "paused"
                            ? <Pause className="h-4 w-4 text-muted-foreground" aria-label={t("states.paused")} />
                            : <Play className="h-4 w-4 text-emerald-600 dark:text-emerald-400" aria-label={t("states.running")} />}
                        </div>
                      </div>
                      <Progress value={Math.round(s.progress * 100)} className="h-1.5" />
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}

function SyncSummaryCard({ onNavigate }: { onNavigate: (tab: ServicesTab) => void }) {
  const { t } = useTranslation("services")
  const { data: pairs } = useSyncPairs()
  const { data: servers } = useMediaServers()
  const { data: runs } = useSyncRuns()
  const latest = latestRunByPair(runs)

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle>{t("mediaSync.summary.syncTitle")}</CardTitle>
          <Button variant="outline" size="sm" onClick={() => onNavigate("media-sync-activity")}>{t("mediaSync.actions.openActivity")}</Button>
        </div>
        <CardDescription>{t("mediaSync.summary.syncDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {!pairs || pairs.length === 0 ? (
          <Button variant="link" className="h-auto px-0" onClick={() => onNavigate("media-sync")}>{t("mediaSync.pairs.none")}</Button>
        ) : (
          <ul className="divide-y">
            {pairs.map(pair => {
              const run = latest.get(pair.id)
              return (
                <li key={pair.id} className="flex flex-wrap items-center justify-between gap-2 py-2 first:pt-0 last:pb-0">
                  <span className="text-sm font-medium">{pairLabel(pair, servers)}</span>
                  <span className="flex items-center gap-2 text-xs text-muted-foreground">
                    {run ? (
                      <>
                        <ToneBadge tone={syncRunTone(run.status)}>{t([`mediaSync.runStatus.${run.status}`, "mediaSync.runStatus.failed"])}</ToneBadge>
                        {formatRelativeTime(run.startedAt)}
                      </>
                    ) : t("mediaSync.activity.never")}
                  </span>
                </li>
              )
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
