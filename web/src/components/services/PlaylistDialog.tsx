/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { PlaylistAccountFields } from "@/components/services/PlaylistAccountFields"
import { ToneBadge } from "@/components/services/shared"
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
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import {
  playlistKeys,
  useApplyPlaylistCompare,
  useCreatePlaylistLink,
  useDeletePlaylistLink,
  usePlaylist,
  usePlaylistServices,
  useRemotePlaylists,
  useSyncPlaylistLink,
  useUpdatePlaylist
} from "@/hooks/usePlaylists"
import { usePlaylistAccountChoice } from "@/hooks/usePlaylistAccount"
import { useMediaServers } from "@/hooks/useMediaSync"
import { api } from "@/lib/api"
import { formatRelativeTime } from "@/lib/dateTimeUtils"
import { applyComparePreset, compareChanges, compareStatus, type ComparePreset } from "@/lib/playlists"
import type { Tone } from "@/lib/services-tone"
import { PLAYLIST_SERVICES, type Playlist, type PlaylistAccount, type PlaylistCompareRow, type PlaylistLink, type PlaylistService, type PlaylistTrack } from "@/types"
import { useQuery } from "@tanstack/react-query"
import { GitCompare, Loader2, Plus, RefreshCw, Trash, X } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

export function PlaylistDialog({ id, onClose, onNavigate }: { id: number; onClose: () => void; onNavigate: (tab: ServicesTab) => void }) {
  const { t } = useTranslation("services")
  const { data: playlist, isLoading } = usePlaylist(id)
  const [adding, setAdding] = useState(false)
  const [comparing, setComparing] = useState<PlaylistLink | null>(null)

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-3xl">
        {isLoading || !playlist ? (
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>{playlist.name}</DialogTitle>
              <DialogDescription>{playlist.request ? t("playlists.detail.asked", { request: playlist.request }) : t("playlists.detail.tracks", { count: playlist.trackCount })}</DialogDescription>
            </DialogHeader>

            <section className="space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h3 className="font-medium">{t("playlists.detail.servicesTitle")}</h3>
                <Button size="sm" onClick={() => setAdding(true)}>
                  <Plus className="mr-2 h-4 w-4" />
                  {t("playlists.detail.addService")}
                </Button>
              </div>
              {playlist.links.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("playlists.detail.noServices")}</p>
              ) : (
                <ul className="divide-y rounded-md border">
                  {playlist.links.map(link => <LinkRow key={link.id} playlist={playlist} link={link} onCompare={() => setComparing(link)} />)}
                </ul>
              )}
            </section>

            <TracksEditor key={playlist.updatedAt} playlist={playlist} />
          </>
        )}
      </DialogContent>

      {adding && playlist && (
        <AddServiceDialog
          playlist={playlist}
          onClose={() => setAdding(false)}
          onCompare={(link) => setComparing(link)}
          onNavigate={(tab) => { onClose(); onNavigate(tab) }}
        />
      )}
      {comparing && playlist && <CompareDialog playlist={playlist} link={comparing} onClose={() => setComparing(null)} />}
    </Dialog>
  )
}

function linkTone(link: PlaylistLink): Tone {
  if (!link.remoteId) return "neutral"
  return link.missing > 0 ? "warn" : "good"
}

function LinkRow({ playlist, link, onCompare }: { playlist: Playlist; link: PlaylistLink; onCompare: () => void }) {
  const { t } = useTranslation("services")
  const { data: servers } = useMediaServers()
  const sync = useSyncPlaylistLink()
  const remove = useDeletePlaylistLink()
  const server = servers?.find(s => s.id === link.serverId)
  const where = [server?.name, link.accountName].filter(Boolean).join(" · ")

  const runSync = () => sync.mutate({ playlistId: playlist.id, linkId: link.id }, {
    onSuccess: (res) => {
      const text = t("playlists.toasts.synced", { count: res.link.matched, service: t(`playlists.services.${link.service}`) })
      if (res.missing.length > 0) toast.warning(`${text} ${t("playlists.toasts.missing", { count: res.missing.length })}`)
      else toast.success(text)
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

  return (
    <li className="flex flex-wrap items-start justify-between gap-3 p-3">
      <div className="min-w-0 space-y-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-medium">{t(`playlists.services.${link.service}`)}</span>
          {where && <span className="text-sm text-muted-foreground">{where}</span>}
          <ToneBadge tone={linkTone(link)}>
            {!link.remoteId ? t("playlists.detail.notCreated") : t("playlists.detail.matched", { matched: link.matched, total: link.matched + link.missing })}
          </ToneBadge>
        </div>
        <p className="text-xs text-muted-foreground">
          {link.remoteName && <span>{t("playlists.detail.remoteName", { name: link.remoteName })} · </span>}
          {link.lastSyncedAt ? t("playlists.detail.lastSynced", { when: formatRelativeTime(link.lastSyncedAt) }) : t("playlists.detail.neverSynced")}
        </p>
        {link.message && <p className="text-xs text-muted-foreground break-words">{link.message}</p>}
      </div>
      <div className="flex shrink-0 items-center gap-1">
        <Button variant="outline" size="sm" onClick={runSync} disabled={sync.isPending}>
          {sync.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <RefreshCw className="mr-2 h-4 w-4" />}
          {t("playlists.detail.sync")}
        </Button>
        <Button variant="outline" size="sm" onClick={onCompare} disabled={!link.remoteId || sync.isPending}>
          <GitCompare className="mr-2 h-4 w-4" />
          {t("playlists.detail.compare")}
        </Button>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8" aria-label={t("playlists.detail.removeService")}>
              <Trash className="h-4 w-4" />
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t("playlists.detail.removeTitle", { service: t(`playlists.services.${link.service}`) })}</AlertDialogTitle>
              <AlertDialogDescription>{t("playlists.detail.removeDescription")}</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t("common:actions.cancel")}</AlertDialogCancel>
              <AlertDialogAction onClick={() => remove.mutate({ playlistId: playlist.id, linkId: link.id }, { onError: (err) => toast.error(errorMessage(err)) })}>
                {t("playlists.detail.removeService")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </li>
  )
}

function TracksEditor({ playlist }: { playlist: Playlist }) {
  const { t } = useTranslation("services")
  const update = useUpdatePlaylist()
  const [name, setName] = useState(playlist.name)
  const [description, setDescription] = useState(playlist.description)
  const [tracks, setTracks] = useState<PlaylistTrack[]>(playlist.tracks ?? [])
  const changed = name !== playlist.name || description !== playlist.description || tracks.length !== (playlist.tracks ?? []).length

  const save = () => update.mutate({ id: playlist.id, data: { name, description, tracks } }, {
    onSuccess: () => toast.success(t("playlists.toasts.saved")),
    onError: (err) => toast.error(errorMessage(err)),
  })

  return (
    <section className="space-y-3 border-t pt-4">
      <h3 className="font-medium">{t("playlists.detail.tracksTitle")}</h3>
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="playlist-edit-name">{t("playlists.list.name")}</Label>
          <Input id="playlist-edit-name" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="playlist-edit-description">{t("playlists.detail.description")}</Label>
          <Textarea id="playlist-edit-description" rows={1} value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
      </div>
      <div className="max-h-80 overflow-y-auto rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">#</TableHead>
              <TableHead>{t("playlists.detail.title")}</TableHead>
              <TableHead>{t("playlists.detail.artist")}</TableHead>
              <TableHead>{t("playlists.detail.album")}</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {tracks.map((track, i) => (
              <TableRow key={`${i}-${track.artist}-${track.title}`}>
                <TableCell className="font-mono text-xs text-muted-foreground">{i + 1}</TableCell>
                <TableCell className="text-sm">{track.title}</TableCell>
                <TableCell className="text-sm">{track.artist}</TableCell>
                <TableCell className="text-xs text-muted-foreground">{track.album || "—"}</TableCell>
                <TableCell>
                  <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setTracks(prev => prev.filter((_, j) => j !== i))} aria-label={t("playlists.detail.removeTrack")}>
                    <X className="h-4 w-4" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">{t("playlists.detail.saveHint")}</p>
        <Button onClick={save} disabled={!changed || update.isPending || name.trim() === ""}>
          {update.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {t("apps.actions.save")}
        </Button>
      </div>
    </section>
  )
}

const MEDIA_SERVER_SERVICES = ["jellyfin", "plex"] as const

function AddServiceDialog({ playlist, onClose, onCompare, onNavigate }: {
  playlist: Playlist
  onClose: () => void
  onCompare: (link: PlaylistLink) => void
  onNavigate: (tab: ServicesTab) => void
}) {
  const { t } = useTranslation("services")
  const { data: status } = usePlaylistServices()
  const create = useCreatePlaylistLink()
  const sync = useSyncPlaylistLink()
  const [service, setService] = useState<PlaylistService>("jellyfin")
  const [mode, setMode] = useState<"new" | "existing">("new")
  const [remoteId, setRemoteId] = useState<string | undefined>(undefined)
  const onServer = service === "jellyfin" || service === "plex"
  const choice = usePlaylistAccountChoice(onServer ? [service] : MEDIA_SERVER_SERVICES)

  const connected = (s: PlaylistService) => s === "spotify" ? status?.spotify.connected : s === "ytmusic" ? status?.ytmusic.connected : true
  const account: PlaylistAccount | null = onServer
    ? (choice.server && choice.account ? { service, serverId: choice.server.id, accountId: choice.account.id } : null)
    : (connected(service) ? { service } : null)
  const remote = useRemotePlaylists(mode === "existing" ? account : null)
  const picked = remote.data?.find(p => p.id === remoteId)
  const busy = create.isPending || sync.isPending

  const submit = () => {
    if (!account) return
    create.mutate({
      playlistId: playlist.id,
      data: { ...account, accountName: onServer ? choice.account?.name : undefined, remoteId: picked?.id, remoteName: picked?.name },
    }, {
      onSuccess: (link) => {
        if (mode === "existing") {
          onClose()
          onCompare(link)
          return
        }
        sync.mutate({ playlistId: playlist.id, linkId: link.id }, {
          onSuccess: (res) => {
            const text = t("playlists.toasts.synced", { count: res.link.matched, service: t(`playlists.services.${service}`) })
            if (res.missing.length > 0) toast.warning(`${text} ${t("playlists.toasts.missing", { count: res.missing.length })}`)
            else toast.success(text)
            onClose()
          },
          onError: (err) => {
            toast.error(errorMessage(err))
            onClose()
          },
        })
      },
      onError: (err) => toast.error(errorMessage(err)),
    })
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("playlists.add.title")}</DialogTitle>
          <DialogDescription>{t("playlists.add.description", { name: playlist.name })}</DialogDescription>
        </DialogHeader>
        <div className="space-y-5">
          <div className="space-y-2">
            <Label htmlFor="playlist-add-service">{t("playlists.add.service")}</Label>
            <Select value={service} onValueChange={(v) => { setService(v as PlaylistService); setRemoteId(undefined) }}>
              <SelectTrigger id="playlist-add-service">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PLAYLIST_SERVICES.map(s => (
                  <SelectItem key={s} value={s}>
                    {t(`playlists.services.${s}`)}
                    {!connected(s) && <span className="ml-1 text-xs text-muted-foreground">{t("playlists.add.notConnected")}</span>}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {onServer ? (
            choice.servers.length === 0 && !choice.loadingServers ? (
              <p className="text-sm text-muted-foreground">
                {t("playlists.add.noServer", { service: t(`playlists.services.${service}`) })}{" "}
                <Button variant="link" className="h-auto p-0" onClick={() => onNavigate("media-servers-manage")}>{t("mediaSync.actions.addServer")}</Button>
              </p>
            ) : (
              <PlaylistAccountFields choice={choice} idPrefix="playlist-add" />
            )
          ) : !connected(service) ? (
            <p className="text-sm text-muted-foreground">
              {t("playlists.add.connectFirst", { service: t(`playlists.services.${service}`) })}{" "}
              <Button variant="link" className="h-auto p-0" onClick={() => onNavigate("playlist-services")}>{t("playlists.add.openServices")}</Button>
            </p>
          ) : (
            <p className="text-sm text-muted-foreground">
              {t("playlists.add.signedInAs", { name: service === "spotify" ? status?.spotify.accountName : status?.ytmusic.accountName })}
            </p>
          )}

          <RadioGroup value={mode} onValueChange={(v) => setMode(v as "new" | "existing")} className="space-y-3">
            <div className="flex items-start gap-3">
              <RadioGroupItem value="new" id="playlist-add-new" className="mt-0.5" />
              <div className="space-y-0.5">
                <Label htmlFor="playlist-add-new" className="cursor-pointer font-medium">{t("playlists.add.newLabel")}</Label>
                <p className="text-xs text-muted-foreground">{t("playlists.add.newDescription")}</p>
              </div>
            </div>
            <div className="flex items-start gap-3">
              <RadioGroupItem value="existing" id="playlist-add-existing" className="mt-0.5" />
              <div className="flex-1 space-y-2">
                <div className="space-y-0.5">
                  <Label htmlFor="playlist-add-existing" className="cursor-pointer font-medium">{t("playlists.add.existingLabel")}</Label>
                  <p className="text-xs text-muted-foreground">{t("playlists.add.existingDescription")}</p>
                </div>
                {mode === "existing" && account && (
                  remote.isLoading ? (
                    <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                  ) : remote.error ? (
                    <p className="text-sm text-destructive break-words">{remote.error.message}</p>
                  ) : (remote.data ?? []).length === 0 ? (
                    <p className="text-sm text-muted-foreground">{t("playlists.add.noRemote")}</p>
                  ) : (
                    <Select value={remoteId} onValueChange={setRemoteId}>
                      <SelectTrigger aria-label={t("playlists.add.existingLabel")}>
                        <SelectValue placeholder={t("playlists.add.pickPlaylist")} />
                      </SelectTrigger>
                      <SelectContent>
                        {(remote.data ?? []).map(p => (
                          <SelectItem key={p.id} value={p.id}>
                            {p.name}{p.count > 0 && <span className="ml-1 text-xs text-muted-foreground">{t("playlists.detail.tracks", { count: p.count })}</span>}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )
                )}
              </div>
            </div>
          </RadioGroup>
        </div>
        <DialogFooter>
          <Button onClick={submit} disabled={busy || !account || (mode === "existing" && !picked)}>
            {busy && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {mode === "existing" ? t("playlists.add.linkAndCompare") : t("playlists.add.createAndSync")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

const COMPARE_PRESETS: ComparePreset[] = ["everything", "local", "remote", "both"]

const STATUS_TONES: Record<ReturnType<typeof compareStatus>, Tone> = {
  both: "good",
  localOnly: "neutral",
  remoteOnly: "neutral",
  notFound: "warn",
}

function CompareDialog({ playlist, link, onClose }: { playlist: Playlist; link: PlaylistLink; onClose: () => void }) {
  const { t } = useTranslation("services")
  // A read-only POST; the key sits under the feature prefix so writes refresh it.
  const { data, isLoading, error, dataUpdatedAt } = useQuery({
    queryKey: [...playlistKeys.all, "compare", playlist.id, link.id],
    queryFn: () => api.comparePlaylistLink(playlist.id, link.id),
    retry: false,
    staleTime: 0,
    gcTime: 0,
  })
  const service = t(`playlists.services.${link.service}`)

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>{t("playlists.compare.title", { service })}</DialogTitle>
          <DialogDescription>{t("playlists.compare.description", { service, name: link.remoteName || playlist.name })}</DialogDescription>
        </DialogHeader>
        {isLoading ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="h-5 w-5 animate-spin" />
            {t("playlists.compare.loading", { service })}
          </div>
        ) : error ? (
          <p className="text-sm text-destructive break-words">{error.message}</p>
        ) : data ? (
          <CompareEditor key={dataUpdatedAt} playlist={playlist} link={link} initial={data} service={service} onDone={onClose} />
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function CompareEditor({ playlist, link, initial, service, onDone }: {
  playlist: Playlist
  link: PlaylistLink
  initial: PlaylistCompareRow[]
  service: string
  onDone: () => void
}) {
  const { t } = useTranslation("services")
  const apply = useApplyPlaylistCompare()
  const [rows, setRows] = useState(initial)
  const changes = compareChanges(rows)
  const nothing = changes.addToService + changes.removeFromService + changes.addToPlaylist + changes.removeFromPlaylist === 0

  const submit = () => apply.mutate({ playlistId: playlist.id, linkId: link.id, rows }, {
    onSuccess: () => {
      toast.success(t("playlists.toasts.applied", { service }))
      onDone()
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-sm text-muted-foreground">{t("playlists.compare.keep")}</span>
        {COMPARE_PRESETS.map(preset => (
          <Button key={preset} variant="outline" size="sm" onClick={() => setRows(prev => applyComparePreset(prev, preset))}>
            {t(`playlists.compare.presets.${preset}`, { service })}
          </Button>
        ))}
      </div>
      <div className="max-h-[50vh] overflow-y-auto rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">{t("playlists.compare.keepColumn")}</TableHead>
              <TableHead>{t("playlists.detail.title")}</TableHead>
              <TableHead>{t("playlists.detail.artist")}</TableHead>
              <TableHead>{t("playlists.compare.where")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map(row => {
              const status = compareStatus(row)
              return (
                <TableRow key={row.key} className={row.keep ? undefined : "opacity-50"}>
                  <TableCell>
                    <Checkbox
                      checked={row.keep}
                      onCheckedChange={(checked) => setRows(prev => prev.map(r => r.key === row.key ? { ...r, keep: checked === true } : r))}
                      aria-label={t("playlists.compare.keepColumn")}
                    />
                  </TableCell>
                  <TableCell className="text-sm">{row.title}</TableCell>
                  <TableCell className="text-sm">{row.artist}</TableCell>
                  <TableCell>
                    <ToneBadge tone={STATUS_TONES[status]}>{t(`playlists.compare.status.${status}`, { service })}</ToneBadge>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>
      <p className="text-sm text-muted-foreground">
        {nothing ? t("playlists.compare.noChanges") : t("playlists.compare.changes", { ...changes, service })}
      </p>
      <DialogFooter>
        <Button onClick={submit} disabled={apply.isPending || nothing}>
          {apply.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {t("playlists.compare.apply")}
        </Button>
      </DialogFooter>
    </div>
  )
}
