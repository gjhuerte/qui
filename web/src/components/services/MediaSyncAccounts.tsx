/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { ServicesTab } from "@/components/services/tabs"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useMediaServers, useMediaServerUsers, useSaveSyncUserMaps, useSyncPairs, useSyncUserMaps } from "@/hooks/useMediaSync"
import { autoMatchUsers, DIRECTION_ARROWS, pairLabel } from "@/lib/media-sync"
import type { MediaServer, MediaServerUser, SyncPair, SyncUserMap } from "@/types"
import { Loader2, Plus, Trash, Wand2 } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

export function MediaSyncAccounts({ onNavigate }: { onNavigate: (tab: ServicesTab) => void }) {
  const { t } = useTranslation("services")
  const { data: pairs, isLoading } = useSyncPairs()
  const { data: servers } = useMediaServers()
  const [selected, setSelected] = useState<number | undefined>(undefined)

  if (isLoading) {
    return <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
  }
  if (!pairs || pairs.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("mediaSync.accounts.title")}</CardTitle>
          <CardDescription>{t("mediaSync.accounts.noPairs")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button onClick={() => onNavigate("media-sync")}>{t("mediaSync.actions.addPair")}</Button>
        </CardContent>
      </Card>
    )
  }

  const pair = pairs.find(p => p.id === selected) ?? pairs[0]

  return (
    <div className="space-y-6">
      {pairs.length > 1 && (
        <div className="max-w-sm space-y-2">
          <Label htmlFor="sync-accounts-pair">{t("mediaSync.accounts.pair")}</Label>
          <Select value={String(pair.id)} onValueChange={(v) => setSelected(Number(v))}>
            <SelectTrigger id="sync-accounts-pair">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {pairs.map(p => <SelectItem key={p.id} value={String(p.id)}>{pairLabel(p, servers)}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
      )}
      <PairAccounts key={pair.id} pair={pair} servers={servers ?? []} />
    </div>
  )
}

function PairAccounts({ pair, servers }: { pair: SyncPair; servers: MediaServer[] }) {
  const { data: maps, isLoading } = useSyncUserMaps(pair.id)
  if (isLoading || !maps) {
    return <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
  }
  return <MapEditor pair={pair} servers={servers} initial={maps} />
}

/** A row being edited; an empty ID means not picked yet. */
type Row = SyncUserMap & { key: number }

function MapEditor({ pair, servers, initial }: { pair: SyncPair; servers: MediaServer[]; initial: SyncUserMap[] }) {
  const { t } = useTranslation("services")
  const save = useSaveSyncUserMaps()
  const usersA = useMediaServerUsers(pair.serverAId)
  const usersB = useMediaServerUsers(pair.serverBId)
  const [nextKey, setNextKey] = useState(initial.length)
  const [rows, setRows] = useState<Row[]>(() => initial.map((m, i) => ({ ...m, key: i })))
  const nameA = servers.find(s => s.id === pair.serverAId)?.name ?? `#${pair.serverAId}`
  const nameB = servers.find(s => s.id === pair.serverBId)?.name ?? `#${pair.serverBId}`

  const addRow = () => {
    setRows(prev => [...prev, { key: nextKey, userAId: "", userAName: "", userBId: "", userBName: "" }])
    setNextKey(k => k + 1)
  }

  const matchByName = () => {
    const current = rows.filter(r => r.userAId && r.userBId)
    const matched = autoMatchUsers(usersA.data ?? [], usersB.data ?? [], current)
    const added = matched.length - current.length
    setRows(matched.map((m, i) => ({ ...m, key: nextKey + i })))
    setNextKey(k => k + matched.length)
    toast.info(t("mediaSync.accounts.matched", { count: added }))
  }

  const update = (key: number, side: "a" | "b", user: MediaServerUser) => {
    setRows(prev => prev.map(r => {
      if (r.key !== key) return r
      return side === "a" ? { ...r, userAId: user.id, userAName: user.name } : { ...r, userBId: user.id, userBName: user.name }
    }))
  }

  const submit = () => {
    if (rows.some(r => !r.userAId || !r.userBId)) {
      toast.error(t("mediaSync.accounts.incomplete"))
      return
    }
    save.mutate({ pairId: pair.id, maps: rows.map(({ key: _key, ...m }) => m) }, {
      onSuccess: () => toast.success(t("mediaSync.toasts.mapsSaved")),
      onError: (err) => toast.error(err instanceof Error ? err.message : String(err)),
    })
  }

  const loadingUsers = usersA.isLoading || usersB.isLoading

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle>{t("mediaSync.accounts.title")}</CardTitle>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" onClick={matchByName} disabled={loadingUsers || !usersA.data || !usersB.data}>
              <Wand2 className="mr-2 h-4 w-4" />
              {t("mediaSync.accounts.matchByName")}
            </Button>
            <Button variant="outline" size="sm" onClick={addRow}>
              <Plus className="mr-2 h-4 w-4" />
              {t("mediaSync.accounts.addRow")}
            </Button>
          </div>
        </div>
        <CardDescription>{t("mediaSync.accounts.description", { a: nameA, b: nameB })}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {usersA.error && <p className="text-sm text-destructive break-words">{t("mediaSync.accounts.usersFailed", { server: nameA, error: usersA.error.message })}</p>}
        {usersB.error && <p className="text-sm text-destructive break-words">{t("mediaSync.accounts.usersFailed", { server: nameB, error: usersB.error.message })}</p>}
        {loadingUsers ? (
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        ) : rows.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("mediaSync.accounts.none")}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("mediaSync.accounts.accountOn", { server: nameA })}</TableHead>
                <TableHead className="w-10 text-center" />
                <TableHead>{t("mediaSync.accounts.accountOn", { server: nameB })}</TableHead>
                <TableHead className="w-12" pinEnd />
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map(row => (
                <TableRow key={row.key}>
                  <TableCell>
                    <UserPicker
                      label={t("mediaSync.accounts.accountOn", { server: nameA })}
                      users={usersA.data ?? []}
                      value={row.userAId}
                      storedName={row.userAName}
                      onChange={(u) => update(row.key, "a", u)}
                    />
                  </TableCell>
                  <TableCell className="text-center text-muted-foreground">{DIRECTION_ARROWS[pair.direction]}</TableCell>
                  <TableCell>
                    <UserPicker
                      label={t("mediaSync.accounts.accountOn", { server: nameB })}
                      users={usersB.data ?? []}
                      value={row.userBId}
                      storedName={row.userBName}
                      onChange={(u) => update(row.key, "b", u)}
                    />
                  </TableCell>
                  <TableCell className="text-right" pinEnd>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8"
                      onClick={() => setRows(prev => prev.filter(r => r.key !== row.key))}
                      aria-label={t("mediaSync.accounts.removeRow")}
                    >
                      <Trash className="h-4 w-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
      <CardFooter className="flex justify-end">
        <Button className="min-h-11 md:min-h-9" onClick={submit} disabled={save.isPending}>
          {save.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {t("apps.actions.save")}
        </Button>
      </CardFooter>
    </Card>
  )
}

function UserPicker({ label, users, value, storedName, onChange }: {
  label: string
  users: MediaServerUser[]
  value: string
  storedName: string
  onChange: (user: MediaServerUser) => void
}) {
  const { t } = useTranslation("services")
  // A mapped account that left the server still shows under its saved name.
  const options = value && !users.some(u => u.id === value)
    ? [...users, { id: value, name: t("mediaSync.accounts.missingUser", { name: storedName }), admin: false }]
    : users
  return (
    <Select value={value || undefined} onValueChange={(id) => { const u = options.find(o => o.id === id); if (u) onChange(u) }}>
      <SelectTrigger className="min-w-40" aria-label={label}>
        <SelectValue placeholder={t("mediaSync.accounts.pick")} />
      </SelectTrigger>
      <SelectContent>
        {options.map(u => (
          <SelectItem key={u.id} value={u.id}>
            {u.name}{u.admin && <span className="ml-1 text-xs text-muted-foreground">{t("mediaSync.accounts.admin")}</span>}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
