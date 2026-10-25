/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { PlaylistAccountFields } from "@/components/services/PlaylistAccountFields"
import { PlaylistDialog } from "@/components/services/PlaylistDialog"
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
  AlertDialogTitle
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import { useImportPlaylist, useDeletePlaylist, usePlaylists } from "@/hooks/usePlaylists"
import { usePlaylistAccountChoice } from "@/hooks/usePlaylistAccount"
import { api } from "@/lib/api"
import { formatRelativeTime } from "@/lib/dateTimeUtils"
import { buildPlaylistPrompt } from "@/lib/playlists"
import { copyTextToClipboard } from "@/lib/utils"
import type { Playlist } from "@/types"
import { Copy, Download, ListMusic, Loader2, Sparkles, Trash } from "lucide-react"
import { useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

const SERVER_TYPES = ["jellyfin", "plex"] as const

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

export function PlaylistsPage({ onNavigate }: { onNavigate: (tab: ServicesTab) => void }) {
  const [openId, setOpenId] = useState<number | null>(null)
  return (
    <div className="space-y-6">
      <CreateCard onNavigate={onNavigate} onCreated={setOpenId} />
      <PlaylistsTable onOpen={setOpenId} />
      {openId !== null && <PlaylistDialog id={openId} onClose={() => setOpenId(null)} onNavigate={onNavigate} />}
    </div>
  )
}

function Step({ n, title, description, children }: { n: number; title: string; description: string; children: ReactNode }) {
  return (
    <section className="flex gap-4">
      <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary text-sm font-semibold text-primary-foreground">{n}</span>
      <div className="min-w-0 flex-1 space-y-3">
        <div className="space-y-0.5">
          <h3 className="font-medium leading-7">{title}</h3>
          <p className="text-sm text-muted-foreground">{description}</p>
        </div>
        {children}
      </div>
    </section>
  )
}

function CreateCard({ onNavigate, onCreated }: { onNavigate: (tab: ServicesTab) => void; onCreated: (id: number) => void }) {
  const { t } = useTranslation("services")
  const choice = usePlaylistAccountChoice(SERVER_TYPES)
  const importPlaylist = useImportPlaylist()
  const [exporting, setExporting] = useState(false)
  const [request, setRequest] = useState("")
  const [count, setCount] = useState("25")
  const [onlyLibrary, setOnlyLibrary] = useState(true)
  const [answer, setAnswer] = useState("")
  const [name, setName] = useState("")

  const hasServer = choice.servers.length > 0
  const prompt = buildPlaylistPrompt({ request, count: Number(count) || 25, onlyLibrary: hasServer && onlyLibrary }, t)

  const exportLibrary = async () => {
    if (!choice.server || !choice.account) return
    setExporting(true)
    try {
      const { text, filename } = await api.exportMusicLibrary({ service: choice.server.type, serverId: choice.server.id, accountId: choice.account.id })
      const url = URL.createObjectURL(new Blob([text], { type: "text/plain;charset=utf-8" }))
      const a = document.createElement("a")
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
      toast.success(t("playlists.create.exported", { size: Math.max(1, Math.round(text.length / 1024)) }))
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setExporting(false)
    }
  }

  const copyPrompt = async () => {
    try {
      await copyTextToClipboard(prompt)
      toast.success(t("playlists.create.promptCopied"))
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  const create = () => {
    importPlaylist.mutate({ text: answer, name, request }, {
      onSuccess: (p) => {
        toast.success(t("playlists.create.created", { name: p.name, count: p.trackCount }))
        setAnswer("")
        setName("")
        onCreated(p.id)
      },
      onError: (err) => toast.error(errorMessage(err)),
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><Sparkles className="h-5 w-5" />{t("playlists.create.title")}</CardTitle>
        <CardDescription>{t("playlists.create.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-8">
        <Step n={1} title={t("playlists.create.step1Title")} description={t("playlists.create.step1Description")}>
          {choice.loadingServers ? (
            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          ) : !hasServer ? (
            <p className="text-sm text-muted-foreground">
              {t("playlists.create.noServer")}{" "}
              <Button variant="link" className="h-auto p-0" onClick={() => onNavigate("media-servers-manage")}>{t("mediaSync.actions.addServer")}</Button>
            </p>
          ) : (
            <>
              <PlaylistAccountFields choice={choice} idPrefix="playlist-export" />
              <Button variant="outline" onClick={exportLibrary} disabled={exporting || !choice.account}>
                {exporting ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Download className="mr-2 h-4 w-4" />}
                {t("playlists.create.export")}
              </Button>
            </>
          )}
        </Step>

        <Step n={2} title={t("playlists.create.step2Title")} description={t("playlists.create.step2Description")}>
          <div className="space-y-2">
            <Label htmlFor="playlist-request">{t("playlists.create.request")}</Label>
            <Textarea
              id="playlist-request"
              rows={3}
              value={request}
              onChange={(e) => setRequest(e.target.value)}
              placeholder={t("playlists.create.requestPlaceholder")}
            />
          </div>
          <div className="flex flex-wrap items-end gap-6">
            <div className="w-32 space-y-2">
              <Label htmlFor="playlist-count">{t("playlists.create.count")}</Label>
              <Input id="playlist-count" type="number" min={1} max={500} value={count} onChange={(e) => setCount(e.target.value)} />
            </div>
            <div className="flex items-center gap-3 pb-2">
              <Switch id="playlist-only-library" checked={hasServer && onlyLibrary} onCheckedChange={setOnlyLibrary} disabled={!hasServer} />
              <Label htmlFor="playlist-only-library">{t("playlists.create.onlyLibrary")}</Label>
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="playlist-prompt">{t("playlists.create.prompt")}</Label>
            <Textarea id="playlist-prompt" rows={9} readOnly value={prompt} className="font-mono text-xs" />
          </div>
          <Button variant="outline" onClick={copyPrompt}>
            <Copy className="mr-2 h-4 w-4" />
            {t("playlists.create.copyPrompt")}
          </Button>
        </Step>

        <Step n={3} title={t("playlists.create.step3Title")} description={t("playlists.create.step3Description")}>
          <div className="space-y-2">
            <Label htmlFor="playlist-answer">{t("playlists.create.answer")}</Label>
            <Textarea
              id="playlist-answer"
              rows={8}
              value={answer}
              onChange={(e) => setAnswer(e.target.value)}
              placeholder={t("playlists.create.answerPlaceholder")}
              className="font-mono text-xs"
              spellCheck={false}
            />
          </div>
          <div className="max-w-sm space-y-2">
            <Label htmlFor="playlist-name">{t("playlists.create.name")}</Label>
            <Input id="playlist-name" value={name} onChange={(e) => setName(e.target.value)} placeholder={t("playlists.create.namePlaceholder")} />
          </div>
          <Button onClick={create} disabled={importPlaylist.isPending || answer.trim() === ""}>
            {importPlaylist.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <ListMusic className="mr-2 h-4 w-4" />}
            {t("playlists.create.submit")}
          </Button>
        </Step>
      </CardContent>
    </Card>
  )
}

function PlaylistsTable({ onOpen }: { onOpen: (id: number) => void }) {
  const { t } = useTranslation("services")
  const { data: playlists, isLoading } = usePlaylists()
  const remove = useDeletePlaylist()
  const [deleting, setDeleting] = useState<Playlist | null>(null)

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("playlists.list.title")}</CardTitle>
        <CardDescription>{t("playlists.list.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        ) : !playlists || playlists.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("playlists.list.none")}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("playlists.list.name")}</TableHead>
                <TableHead className="text-right">{t("playlists.list.tracks")}</TableHead>
                <TableHead>{t("playlists.list.services")}</TableHead>
                <TableHead>{t("playlists.list.created")}</TableHead>
                <TableHead className="w-24" pinEnd />
              </TableRow>
            </TableHeader>
            <TableBody>
              {playlists.map(p => (
                <TableRow key={p.id} className="cursor-pointer" onClick={() => onOpen(p.id)}>
                  <TableCell>
                    <p className="font-medium">{p.name}</p>
                    {p.description && <p className="max-w-md truncate text-xs text-muted-foreground">{p.description}</p>}
                  </TableCell>
                  <TableCell className="text-right font-mono text-xs">{p.trackCount}</TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {p.links.length === 0 ? (
                        <span className="text-xs text-muted-foreground">{t("playlists.list.notSynced")}</span>
                      ) : p.links.map(l => (
                        <ToneBadge key={l.id} tone={!l.remoteId ? "neutral" : l.missing > 0 ? "warn" : "good"}>
                          {t(`playlists.services.${l.service}`)}
                        </ToneBadge>
                      ))}
                    </div>
                  </TableCell>
                  <TableCell className="whitespace-nowrap text-xs">{formatRelativeTime(p.createdAt)}</TableCell>
                  <TableCell className="text-right" pinEnd>
                    <div className="flex justify-end" onClick={(e) => e.stopPropagation()}>
                      <Button variant="ghost" size="sm" onClick={() => onOpen(p.id)}>{t("playlists.list.open")}</Button>
                      <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setDeleting(p)} aria-label={t("playlists.list.delete")}>
                        <Trash className="h-4 w-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>

      <AlertDialog open={deleting !== null} onOpenChange={(open) => !open && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("apps.deleteDialog.title", { name: deleting?.name })}</AlertDialogTitle>
            <AlertDialogDescription>{t("playlists.list.deleteDescription")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common:actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleting && remove.mutate(deleting.id, {
                onSuccess: () => toast.success(t("playlists.toasts.deleted")),
                onError: (err) => toast.error(errorMessage(err)),
              })}
            >
              {t("apps.actions.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  )
}
