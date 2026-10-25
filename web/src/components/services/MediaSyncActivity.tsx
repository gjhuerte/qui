/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { RunFooter } from "@/components/cross-seed/RunFooter"
import { InfoGrid, InfoItem, ToneBadge } from "@/components/services/shared"
import type { ServicesTab } from "@/components/services/tabs"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useDateTimeFormatters } from "@/hooks/useDateTimeFormatters"
import { useCancelSyncPair, useMediaServers, useRunSyncPair, useSyncPairs, useSyncRun, useSyncRuns, useSyncUserMaps } from "@/hooks/useMediaSync"
import { formatRelativeTime } from "@/lib/dateTimeUtils"
import { latestRunByPair, pairLabel } from "@/lib/media-sync"
import { mediaSyncRunState } from "@/lib/run-state"
import { syncRunTone } from "@/lib/services-tone"
import type { MediaServer, SyncPair, SyncRun } from "@/types"
import { List, Loader2, RefreshCw } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

export function MediaSyncActivity({ onNavigate }: { onNavigate: (tab: ServicesTab) => void }) {
  const { t } = useTranslation("services")
  const { data: pairs, isLoading } = useSyncPairs()
  const { data: servers } = useMediaServers()
  const { data: runs } = useSyncRuns()
  const [openRun, setOpenRun] = useState<SyncRun | null>(null)
  const latest = latestRunByPair(runs)

  if (isLoading) {
    return <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
  }
  if (!pairs || pairs.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("mediaSync.activity.title")}</CardTitle>
          <CardDescription>{t("mediaSync.accounts.noPairs")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button onClick={() => onNavigate("media-sync")}>{t("mediaSync.actions.addPair")}</Button>
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="space-y-6">
      <div className="grid items-start gap-4 xl:grid-cols-2">
        {pairs.map(pair => <PairRunCard key={pair.id} pair={pair} servers={servers} latest={latest.get(pair.id)} onOpenRun={setOpenRun} />)}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("mediaSync.activity.logTitle")}</CardTitle>
          <CardDescription>{t("mediaSync.activity.logDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          {!runs || runs.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("mediaSync.activity.never")}</p>
          ) : (
            <RunTable runs={runs} pairs={pairs} servers={servers} onOpen={setOpenRun} />
          )}
        </CardContent>
      </Card>

      {openRun && <RunDialog run={openRun} pairs={pairs} servers={servers} onClose={() => setOpenRun(null)} />}
    </div>
  )
}

function PairRunCard({ pair, servers, latest, onOpenRun }: {
  pair: SyncPair
  servers: MediaServer[] | undefined
  latest: SyncRun | undefined
  onOpenRun: (run: SyncRun) => void
}) {
  const { t } = useTranslation("services")
  const { data: maps } = useSyncUserMaps(pair.id)
  const run = useRunSyncPair()
  const cancel = useCancelSyncPair()
  const state = mediaSyncRunState({
    running: latest?.status === "running",
    cancelPending: cancel.isPending,
    startPending: run.isPending,
    hasMaps: (maps?.length ?? 0) > 0,
  }, t)

  return (
    <Card className="min-w-0">
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle>{pairLabel(pair, servers)}</CardTitle>
          {latest && (
            <ToneBadge tone={syncRunTone(latest.status)}>
              {t([`mediaSync.runStatus.${latest.status}`, "mediaSync.runStatus.failed"])}
            </ToneBadge>
          )}
        </div>
        <CardDescription>
          {pair.enabled ? t("mediaSync.activity.every", { count: pair.intervalMinutes }) : t("mediaSync.activity.manualOnly")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {latest ? (
          <>
            <InfoGrid>
              <InfoItem label={t("mediaSync.activity.lastRun")}>{formatRelativeTime(latest.startedAt)}</InfoItem>
              <InfoItem label={t("mediaSync.activity.markedOn", { server: servers?.find(s => s.id === pair.serverAId)?.name ?? "A" })}>{latest.markedA}</InfoItem>
              <InfoItem label={t("mediaSync.activity.markedOn", { server: servers?.find(s => s.id === pair.serverBId)?.name ?? "B" })}>{latest.markedB}</InfoItem>
              <InfoItem label={t("mediaSync.activity.notFound")}>{latest.notFound}</InfoItem>
            </InfoGrid>
            {latest.message && <p className="text-sm text-muted-foreground break-words">{latest.message}</p>}
            {latest.status !== "running" && (
              <Button variant="link" size="sm" className="h-auto px-0" onClick={() => onOpenRun(latest)}>{t("mediaSync.activity.details")}</Button>
            )}
          </>
        ) : (
          <p className="text-sm text-muted-foreground">{t("mediaSync.activity.never")}</p>
        )}
      </CardContent>
      <RunFooter
        state={state}
        runLabel={t("mediaSync.actions.syncNow")}
        runIcon={<RefreshCw className="mr-2 h-4 w-4" />}
        onRun={() => run.mutate(pair.id, {
          onSuccess: () => toast.success(t("mediaSync.toasts.started")),
          onError: (err) => toast.error(errorMessage(err)),
        })}
        onCancel={() => cancel.mutate(pair.id, { onError: (err) => toast.error(errorMessage(err)) })}
        cancelDialog={{ title: t("mediaSync.cancelDialog.title"), description: t("mediaSync.cancelDialog.description") }}
      >
        {null}
      </RunFooter>
    </Card>
  )
}

function RunTable({ runs, pairs, servers, onOpen }: {
  runs: SyncRun[]
  pairs: SyncPair[]
  servers: MediaServer[] | undefined
  onOpen: (run: SyncRun) => void
}) {
  const { t } = useTranslation("services")
  const { formatISOTimestamp } = useDateTimeFormatters()
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t("mediaSync.activity.time")}</TableHead>
          <TableHead>{t("mediaSync.activity.pair")}</TableHead>
          <TableHead>{t("mediaSync.activity.source")}</TableHead>
          <TableHead>{t("mediaSync.activity.result")}</TableHead>
          <TableHead className="text-right">{t("mediaSync.activity.marked")}</TableHead>
          <TableHead className="text-right">{t("mediaSync.activity.notFound")}</TableHead>
          <TableHead>{t("mediaSync.activity.message")}</TableHead>
          <TableHead className="w-12" pinEnd />
        </TableRow>
      </TableHeader>
      <TableBody>
        {runs.map(run => {
          const pair = pairs.find(p => p.id === run.pairId)
          return (
            <TableRow key={run.id}>
              <TableCell className="whitespace-nowrap text-xs">{formatISOTimestamp(run.startedAt)}</TableCell>
              <TableCell className="whitespace-nowrap text-xs">{pair ? pairLabel(pair, servers) : `#${run.pairId}`}</TableCell>
              <TableCell className="text-xs">{t([`mediaSync.sources.${run.source}`, "mediaSync.sources.manual"])}</TableCell>
              <TableCell>
                <ToneBadge tone={syncRunTone(run.status)}>{t([`mediaSync.runStatus.${run.status}`, "mediaSync.runStatus.failed"])}</ToneBadge>
              </TableCell>
              <TableCell className="text-right font-mono text-xs">{run.markedA + run.markedB}</TableCell>
              <TableCell className="text-right font-mono text-xs">{run.notFound}</TableCell>
              <TableCell className="max-w-72 truncate text-xs" title={run.message}>{run.message || "—"}</TableCell>
              <TableCell className="text-right" pinEnd>
                <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => onOpen(run)} disabled={run.status === "running"} aria-label={t("mediaSync.activity.details")}>
                  <List className="h-4 w-4" />
                </Button>
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}

function RunDialog({ run, pairs, servers, onClose }: {
  run: SyncRun
  pairs: SyncPair[]
  servers: MediaServer[] | undefined
  onClose: () => void
}) {
  const { t } = useTranslation("services")
  const { formatISOTimestamp } = useDateTimeFormatters()
  const { data, isLoading } = useSyncRun(run.id)
  const pair = pairs.find(p => p.id === run.pairId)
  const target = (side: "a" | "b") => {
    const id = side === "a" ? pair?.serverAId : pair?.serverBId
    return servers?.find(s => s.id === id)?.name ?? side.toUpperCase()
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{pair ? pairLabel(pair, servers) : `#${run.pairId}`}</DialogTitle>
          <DialogDescription>
            {formatISOTimestamp(run.startedAt)} · {t([`mediaSync.runStatus.${run.status}`, "mediaSync.runStatus.failed"])}
          </DialogDescription>
        </DialogHeader>
        {run.message && <p className="text-sm text-muted-foreground break-words">{run.message}</p>}
        {isLoading || !data ? (
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        ) : !data.items || data.items.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("mediaSync.activity.nothingMarked")}</p>
        ) : (
          <div className="max-h-[60vh] overflow-y-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("mediaSync.activity.titleColumn")}</TableHead>
                  <TableHead>{t("mediaSync.activity.account")}</TableHead>
                  <TableHead>{t("mediaSync.activity.markedOnColumn")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.items.map((item, i) => (
                  <TableRow key={i}>
                    <TableCell className="text-xs">
                      {item.title}
                      {item.error && <p className="text-destructive">{item.error}</p>}
                    </TableCell>
                    <TableCell className="text-xs">{item.userName}</TableCell>
                    <TableCell className="text-xs">{target(item.target)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {data.markedA + data.markedB + data.failed > data.items.length && (
              <p className="mt-2 text-xs text-muted-foreground">{t("mediaSync.activity.truncated", { count: data.items.length })}</p>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
