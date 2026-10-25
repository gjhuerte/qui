/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { ToneBadge } from "@/components/services/shared"
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
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { FieldHelp } from "@/components/ui/field-help"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useDeleteMediaServer, useMediaServers, useSaveMediaServer, useTestMediaServer } from "@/hooks/useMediaSync"
import { MEDIA_SERVER_TYPES, type MediaServer, type MediaServerInput, type MediaServerStatus, type MediaServerType } from "@/types"
import { Loader2, Pencil, Plus, Trash } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

/** Default ports, used as the URL placeholder for each type. */
const DEFAULT_URLS: Record<MediaServerType, string> = {
  jellyfin: "http://192.168.1.10:8096",
  plex: "http://192.168.1.10:32400",
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

export function MediaServersManage() {
  const { t } = useTranslation("services")
  const { data: servers, isLoading } = useMediaServers()
  const save = useSaveMediaServer()
  const remove = useDeleteMediaServer()
  // undefined: dialog closed; null: adding; a server: editing.
  const [editing, setEditing] = useState<MediaServer | null | undefined>(undefined)
  const [deleting, setDeleting] = useState<MediaServer | null>(null)

  const toggle = (srv: MediaServer, enabled: boolean) => {
    save.mutate({ id: srv.id, data: { type: srv.type, name: srv.name, baseUrl: srv.baseUrl, token: "", enabled } }, {
      onError: (err) => toast.error(errorMessage(err)),
    })
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <CardTitle>{t("mediaSync.servers.title")}</CardTitle>
            <Button size="sm" onClick={() => setEditing(null)}>
              <Plus className="mr-2 h-4 w-4" />
              {t("mediaSync.actions.addServer")}
            </Button>
          </div>
          <CardDescription>{t("mediaSync.servers.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          ) : !servers || servers.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("mediaSync.servers.none")}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("apps.form.name")}</TableHead>
                  <TableHead>{t("mediaSync.form.type")}</TableHead>
                  <TableHead>{t("apps.form.url")}</TableHead>
                  <TableHead>{t("apps.form.enabled")}</TableHead>
                  <TableHead className="w-24" pinEnd />
                </TableRow>
              </TableHeader>
              <TableBody>
                {servers.map(srv => (
                  <TableRow key={srv.id}>
                    <TableCell className="font-medium">{srv.name}</TableCell>
                    <TableCell>{t(`mediaSync.types.${srv.type}`)}</TableCell>
                    <TableCell className="max-w-56 truncate font-mono text-xs">{srv.baseUrl}</TableCell>
                    <TableCell>
                      <Switch checked={srv.enabled} onCheckedChange={(checked) => toggle(srv, checked)} aria-label={t("apps.form.enabled")} />
                    </TableCell>
                    <TableCell className="text-right" pinEnd>
                      <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setEditing(srv)} aria-label={t("apps.actions.edit")}>
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setDeleting(srv)} aria-label={t("apps.actions.delete")}>
                        <Trash className="h-4 w-4" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("mediaSync.help.title")}</CardTitle>
          <CardDescription>{t("mediaSync.help.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
            <li>{t("mediaSync.form.tokenHelp.jellyfin")}</li>
            <li>{t("mediaSync.form.tokenHelp.plex")}</li>
            <li>{t("mediaSync.help.plexUsers")}</li>
            <li>{t("mediaSync.help.ids")}</li>
          </ul>
        </CardContent>
      </Card>

      {editing !== undefined && <ServerDialog server={editing} onClose={() => setEditing(undefined)} />}

      <AlertDialog open={deleting !== null} onOpenChange={(open) => !open && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("apps.deleteDialog.title", { name: deleting?.name })}</AlertDialogTitle>
            <AlertDialogDescription>{t("mediaSync.servers.deleteDescription")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common:actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleting && remove.mutate(deleting.id, {
                onSuccess: () => toast.success(t("mediaSync.toasts.serverDeleted")),
                onError: (err) => toast.error(errorMessage(err)),
              })}
            >
              {t("apps.actions.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function ServerDialog({ server, onClose }: { server: MediaServer | null; onClose: () => void }) {
  const { t } = useTranslation("services")
  const save = useSaveMediaServer()
  const test = useTestMediaServer()
  const [type, setType] = useState<MediaServerType>(server?.type ?? "jellyfin")
  const [name, setName] = useState(server?.name ?? "")
  const [baseUrl, setBaseUrl] = useState(server?.baseUrl ?? "")
  const [token, setToken] = useState("")
  const [enabled, setEnabled] = useState(server?.enabled ?? true)
  const [result, setResult] = useState<MediaServerStatus | null>(null)

  const input = (): MediaServerInput => ({ type, name: name || t(`mediaSync.types.${type}`), baseUrl, token, enabled })

  const runTest = () => {
    setResult(null)
    test.mutate({ ...input(), id: server?.id }, {
      onSuccess: setResult,
      onError: (err) => toast.error(errorMessage(err)),
    })
  }

  const submit = () => {
    save.mutate({ id: server?.id, data: input() }, {
      onSuccess: () => {
        toast.success(t(server ? "mediaSync.toasts.serverUpdated" : "mediaSync.toasts.serverAdded"))
        onClose()
      },
      onError: (err) => toast.error(errorMessage(err)),
    })
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{server ? t("mediaSync.form.editTitle") : t("mediaSync.form.addTitle")}</DialogTitle>
          <DialogDescription>{t(`mediaSync.form.tokenHelp.${type}`)}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="media-server-type">{t("mediaSync.form.type")}</Label>
              <Select value={type} onValueChange={(value) => { setType(value as MediaServerType); setResult(null) }}>
                <SelectTrigger id="media-server-type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {MEDIA_SERVER_TYPES.map(s => <SelectItem key={s} value={s}>{t(`mediaSync.types.${s}`)}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="media-server-name">{t("apps.form.name")}</Label>
              <Input id="media-server-name" value={name} onChange={(e) => setName(e.target.value)} placeholder={t(`mediaSync.types.${type}`)} />
            </div>
          </div>
          <div className="space-y-2">
            <div className="flex items-center gap-1.5">
              <Label htmlFor="media-server-url">{t("apps.form.url")}</Label>
              <FieldHelp>{t("mediaSync.form.urlHelp")}</FieldHelp>
            </div>
            <Input id="media-server-url" value={baseUrl} onChange={(e) => { setBaseUrl(e.target.value); setResult(null) }} placeholder={DEFAULT_URLS[type]} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="media-server-token">{t(`mediaSync.form.token.${type}`)}</Label>
            <Input
              id="media-server-token"
              type="password"
              value={token}
              onChange={(e) => { setToken(e.target.value); setResult(null) }}
              placeholder={server ? t("mediaSync.form.tokenSaved") : ""}
              autoComplete="off"
              data-1p-ignore="true"
            />
          </div>
          <div className="flex items-center justify-between gap-3">
            <Label htmlFor="media-server-enabled" className="font-medium">{t("apps.form.enabled")}</Label>
            <Switch id="media-server-enabled" checked={enabled} onCheckedChange={setEnabled} />
          </div>
          {result && (
            <div className="rounded-md border p-3 text-sm">
              <ToneBadge tone={result.online ? "good" : "bad"}>
                {result.online
                  ? t("mediaSync.form.testOk", { name: result.serverName || "—", version: result.version ?? "—" })
                  : t("apps.form.testFailed")}
              </ToneBadge>
              {!result.online && <p className="mt-2 break-words text-destructive">{result.error}</p>}
            </div>
          )}
        </div>
        <DialogFooter className="gap-2 sm:justify-between">
          <Button variant="outline" onClick={runTest} disabled={test.isPending || baseUrl.trim() === ""}>
            {test.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {t("apps.actions.test")}
          </Button>
          <Button onClick={submit} disabled={save.isPending || baseUrl.trim() === "" || (!server && token.trim() === "")}>
            {save.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {t("apps.actions.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
