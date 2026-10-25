/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { ServicesTab } from "@/components/services/tabs"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { SaveFooter } from "@/components/ui/save-footer"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { useDeleteSyncPair, useMediaServers, useSaveSyncPair, useSyncPairs } from "@/hooks/useMediaSync"
import { pairLabel } from "@/lib/media-sync"
import { SYNC_DIRECTIONS, type MediaServer, type SyncDirection, type SyncPair } from "@/types"
import { Loader2, Plus, Trash } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

const MIN_INTERVAL = 5
const MAX_INTERVAL = 7 * 24 * 60

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

export function MediaSyncPairs({ onNavigate }: { onNavigate: (tab: ServicesTab) => void }) {
  const { t } = useTranslation("services")
  const { data: servers, isLoading: loadingServers } = useMediaServers()
  const { data: pairs, isLoading: loadingPairs } = useSyncPairs()

  if (loadingServers || loadingPairs) {
    return <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
  }

  if (!servers || servers.length < 2) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("mediaSync.pairs.needServersTitle")}</CardTitle>
          <CardDescription>{t("mediaSync.pairs.needServers")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button onClick={() => onNavigate("media-servers-manage")}>{t("mediaSync.actions.addServer")}</Button>
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="space-y-6">
      {pairs?.map(pair => (
        <PairCard key={`${pair.id}-${pair.updatedAt}`} pair={pair} servers={servers} onNavigate={onNavigate} />
      ))}
      <AddPairCard servers={servers} first={!pairs || pairs.length === 0} />
    </div>
  )
}

function serverName(servers: MediaServer[], id: number): string {
  return servers.find(s => s.id === id)?.name ?? `#${id}`
}

function DirectionOptions({ id, value, onChange, nameA, nameB }: {
  id: string
  value: SyncDirection
  onChange: (value: SyncDirection) => void
  nameA: string
  nameB: string
}) {
  const { t } = useTranslation("services")
  return (
    <RadioGroup value={value} onValueChange={(v) => onChange(v as SyncDirection)} className="space-y-3">
      {SYNC_DIRECTIONS.map(direction => (
        <div key={direction} className="flex items-start gap-3">
          <RadioGroupItem value={direction} id={`${id}-${direction}`} className="mt-0.5" />
          <div className="flex-1 space-y-0.5">
            <Label htmlFor={`${id}-${direction}`} className="cursor-pointer font-medium">
              {t(`mediaSync.directions.${direction}.label`, { a: nameA, b: nameB })}
            </Label>
            <p className="text-xs text-muted-foreground">{t(`mediaSync.directions.${direction}.description`, { a: nameA, b: nameB })}</p>
          </div>
        </div>
      ))}
    </RadioGroup>
  )
}

function PairCard({ pair, servers, onNavigate }: { pair: SyncPair; servers: MediaServer[]; onNavigate: (tab: ServicesTab) => void }) {
  const { t } = useTranslation("services")
  const save = useSaveSyncPair()
  const remove = useDeleteSyncPair()
  const [direction, setDirection] = useState(pair.direction)
  const [enabled, setEnabled] = useState(pair.enabled)
  const [minutes, setMinutes] = useState(String(pair.intervalMinutes))
  const nameA = serverName(servers, pair.serverAId)
  const nameB = serverName(servers, pair.serverBId)

  const submit = () => {
    const interval = Number(minutes)
    if (!Number.isInteger(interval) || interval < MIN_INTERVAL || interval > MAX_INTERVAL) {
      toast.error(t("mediaSync.pairs.invalidInterval", { min: MIN_INTERVAL, max: MAX_INTERVAL }))
      return
    }
    save.mutate({ id: pair.id, data: { serverAId: pair.serverAId, serverBId: pair.serverBId, direction, enabled, intervalMinutes: interval } }, {
      onSuccess: () => toast.success(t("mediaSync.toasts.pairSaved")),
      onError: (err) => toast.error(errorMessage(err)),
    })
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle>{pairLabel({ ...pair, direction }, servers)}</CardTitle>
          <div className="flex items-center gap-1">
            <Button variant="outline" size="sm" onClick={() => onNavigate("media-sync-accounts")}>{t("mediaSync.actions.mapAccounts")}</Button>
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="ghost" size="icon" className="h-8 w-8" aria-label={t("mediaSync.actions.deletePair")}>
                  <Trash className="h-4 w-4" />
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t("mediaSync.pairs.deleteTitle", { pair: pairLabel(pair, servers) })}</AlertDialogTitle>
                  <AlertDialogDescription>{t("mediaSync.pairs.deleteDescription")}</AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t("common:actions.cancel")}</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={() => remove.mutate(pair.id, {
                      onSuccess: () => toast.success(t("mediaSync.toasts.pairDeleted")),
                      onError: (err) => toast.error(errorMessage(err)),
                    })}
                  >
                    {t("apps.actions.delete")}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        </div>
        <CardDescription>{t("mediaSync.pairs.description", { a: nameA, b: nameB })}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <DirectionOptions id={`pair-${pair.id}`} value={direction} onChange={setDirection} nameA={nameA} nameB={nameB} />
        <div className="space-y-4 border-t pt-4">
          <div className="flex items-center justify-between gap-3">
            <div className="space-y-0.5">
              <Label htmlFor={`pair-${pair.id}-enabled`} className="font-medium">{t("mediaSync.pairs.schedule")}</Label>
              <p className="text-xs text-muted-foreground">{t("mediaSync.pairs.scheduleHelp")}</p>
            </div>
            <Switch id={`pair-${pair.id}-enabled`} checked={enabled} onCheckedChange={setEnabled} />
          </div>
          <div className="max-w-xs space-y-2">
            <Label htmlFor={`pair-${pair.id}-interval`}>{t("mediaSync.pairs.interval")}</Label>
            <Input
              id={`pair-${pair.id}-interval`}
              type="number"
              min={MIN_INTERVAL}
              max={MAX_INTERVAL}
              value={minutes}
              disabled={!enabled}
              onChange={(e) => setMinutes(e.target.value)}
            />
          </div>
        </div>
      </CardContent>
      <SaveFooter pending={save.isPending} onSave={submit} label={t("apps.actions.save")} />
    </Card>
  )
}

function AddPairCard({ servers, first }: { servers: MediaServer[]; first: boolean }) {
  const { t } = useTranslation("services")
  const save = useSaveSyncPair()
  const [serverA, setServerA] = useState(String(servers[0].id))
  const [serverB, setServerB] = useState(String(servers[1].id))
  const [direction, setDirection] = useState<SyncDirection>("both")
  const same = serverA === serverB

  const submit = () => {
    save.mutate({ data: { serverAId: Number(serverA), serverBId: Number(serverB), direction, enabled: true, intervalMinutes: 60 } }, {
      onSuccess: () => toast.success(t("mediaSync.toasts.pairAdded")),
      onError: (err) => toast.error(errorMessage(err)),
    })
  }

  const picker = (id: string, label: string, value: string, onChange: (v: string) => void) => (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger id={id}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {servers.map(s => {
            const type = t(`mediaSync.types.${s.type}`)
            return <SelectItem key={s.id} value={String(s.id)}>{s.name === type ? s.name : `${s.name} (${type})`}</SelectItem>
          })}
        </SelectContent>
      </Select>
    </div>
  )

  return (
    <Card>
      <CardHeader>
        <CardTitle>{first ? t("mediaSync.pairs.firstTitle") : t("mediaSync.pairs.addTitle")}</CardTitle>
        <CardDescription>{t("mediaSync.pairs.addDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="grid gap-4 sm:grid-cols-2">
          {picker("pair-new-a", t("mediaSync.pairs.serverA"), serverA, setServerA)}
          {picker("pair-new-b", t("mediaSync.pairs.serverB"), serverB, setServerB)}
        </div>
        {same ? (
          <p className="text-sm text-destructive">{t("mediaSync.pairs.sameServer")}</p>
        ) : (
          <DirectionOptions
            id="pair-new"
            value={direction}
            onChange={setDirection}
            nameA={serverName(servers, Number(serverA))}
            nameB={serverName(servers, Number(serverB))}
          />
        )}
      </CardContent>
      <CardFooter className="flex justify-end">
        <Button className="min-h-11 md:min-h-9" onClick={submit} disabled={save.isPending || same}>
          {save.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Plus className="mr-2 h-4 w-4" />}
          {t("mediaSync.actions.addPair")}
        </Button>
      </CardFooter>
    </Card>
  )
}
