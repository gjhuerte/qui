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
import {
  useAppInstances,
  useDeleteAppInstance,
  useSaveAppInstance,
  useTestAppInstance
} from "@/hooks/useServices"
import { APP_TYPES, type AppInstance, type AppInstanceInput, type AppStatus, type AppType } from "@/types"
import { Loader2, Pencil, Plus, Trash } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

/** Default ports, used as the URL placeholder for each type. */
const DEFAULT_URLS: Record<AppType, string> = {
  sonarr: "http://192.168.1.10:8989",
  radarr: "http://192.168.1.10:7878",
  prowlarr: "http://192.168.1.10:9696",
  bazarr: "http://192.168.1.10:6767",
  tautulli: "http://192.168.1.10:8181",
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

export function AppsManage() {
  const { t } = useTranslation("services")
  const { data: instances, isLoading } = useAppInstances()
  const save = useSaveAppInstance()
  const remove = useDeleteAppInstance()
  // undefined: dialog closed; null: adding; an instance: editing.
  const [editing, setEditing] = useState<AppInstance | null | undefined>(undefined)
  const [deleting, setDeleting] = useState<AppInstance | null>(null)

  const toggle = (inst: AppInstance, enabled: boolean) => {
    save.mutate({ id: inst.id, data: { type: inst.type, name: inst.name, baseUrl: inst.baseUrl, apiKey: "", enabled } }, {
      onError: (err) => toast.error(errorMessage(err)),
    })
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <CardTitle>{t("apps.manage.title")}</CardTitle>
            <Button size="sm" onClick={() => setEditing(null)}>
              <Plus className="mr-2 h-4 w-4" />
              {t("apps.actions.add")}
            </Button>
          </div>
          <CardDescription>{t("apps.manage.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          ) : !instances || instances.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("apps.manage.none")}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("apps.form.name")}</TableHead>
                  <TableHead>{t("apps.form.type")}</TableHead>
                  <TableHead>{t("apps.form.url")}</TableHead>
                  <TableHead>{t("apps.form.enabled")}</TableHead>
                  <TableHead className="w-24" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {instances.map(inst => (
                  <TableRow key={inst.id}>
                    <TableCell className="font-medium">{inst.name}</TableCell>
                    <TableCell>{t(`apps.types.${inst.type}`)}</TableCell>
                    <TableCell className="max-w-56 truncate font-mono text-xs">{inst.baseUrl}</TableCell>
                    <TableCell>
                      <Switch checked={inst.enabled} onCheckedChange={(checked) => toggle(inst, checked)} aria-label={t("apps.form.enabled")} />
                    </TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setEditing(inst)} aria-label={t("apps.actions.edit")}>
                        <Pencil className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setDeleting(inst)} aria-label={t("apps.actions.delete")}>
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

      <HelpCard />

      {editing !== undefined && (
        <AppDialog instance={editing} onClose={() => setEditing(undefined)} />
      )}

      <AlertDialog open={deleting !== null} onOpenChange={(open) => !open && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("apps.deleteDialog.title", { name: deleting?.name })}</AlertDialogTitle>
            <AlertDialogDescription>{t("apps.deleteDialog.description")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common:actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleting && remove.mutate(deleting.id, {
                onSuccess: () => toast.success(t("apps.toasts.deleted")),
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

function AppDialog({ instance, onClose }: { instance: AppInstance | null; onClose: () => void }) {
  const { t } = useTranslation("services")
  const save = useSaveAppInstance()
  const test = useTestAppInstance()
  const [type, setType] = useState<AppType>(instance?.type ?? "sonarr")
  const [name, setName] = useState(instance?.name ?? "")
  const [baseUrl, setBaseUrl] = useState(instance?.baseUrl ?? "")
  const [apiKey, setApiKey] = useState("")
  const [enabled, setEnabled] = useState(instance?.enabled ?? true)
  const [result, setResult] = useState<AppStatus | null>(null)

  const input = (): AppInstanceInput => ({ type, name: name || t(`apps.types.${type}`), baseUrl, apiKey, enabled })

  const runTest = () => {
    setResult(null)
    test.mutate({ ...input(), id: instance?.id }, {
      onSuccess: setResult,
      onError: (err) => toast.error(errorMessage(err)),
    })
  }

  const submit = () => {
    save.mutate({ id: instance?.id, data: input() }, {
      onSuccess: () => {
        toast.success(t(instance ? "apps.toasts.updated" : "apps.toasts.added"))
        onClose()
      },
      onError: (err) => toast.error(errorMessage(err)),
    })
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{instance ? t("apps.form.editTitle") : t("apps.form.addTitle")}</DialogTitle>
          <DialogDescription>{t(`apps.form.keyHelp.${type}`)}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="app-type">{t("apps.form.type")}</Label>
              <Select value={type} onValueChange={(value) => { setType(value as AppType); setResult(null) }}>
                <SelectTrigger id="app-type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {APP_TYPES.map(a => <SelectItem key={a} value={a}>{t(`apps.types.${a}`)}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="app-name">{t("apps.form.name")}</Label>
              <Input id="app-name" value={name} onChange={(e) => setName(e.target.value)} placeholder={t(`apps.types.${type}`)} />
            </div>
          </div>
          <div className="space-y-2">
            <div className="flex items-center gap-1.5">
              <Label htmlFor="app-url">{t("apps.form.url")}</Label>
              <FieldHelp>{t("apps.form.urlHelp")}</FieldHelp>
            </div>
            <Input id="app-url" value={baseUrl} onChange={(e) => { setBaseUrl(e.target.value); setResult(null) }} placeholder={DEFAULT_URLS[type]} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="app-key">{t("apps.form.apiKey")}</Label>
            <Input
              id="app-key"
              type="password"
              value={apiKey}
              onChange={(e) => { setApiKey(e.target.value); setResult(null) }}
              placeholder={instance ? t("apps.form.apiKeySaved") : ""}
              autoComplete="off"
              data-1p-ignore="true"
            />
          </div>
          <div className="flex items-center justify-between gap-3">
            <Label htmlFor="app-enabled" className="font-medium">{t("apps.form.enabled")}</Label>
            <Switch id="app-enabled" checked={enabled} onCheckedChange={setEnabled} />
          </div>
          {result && (
            <div className="rounded-md border p-3 text-sm">
              <ToneBadge tone={result.online ? "good" : "bad"}>
                {result.online ? t("apps.form.testOk", { version: result.version ?? "—" }) : t("apps.form.testFailed")}
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
          <Button onClick={submit} disabled={save.isPending || baseUrl.trim() === "" || (!instance && apiKey.trim() === "")}>
            {save.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {t("apps.actions.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function HelpCard() {
  const { t } = useTranslation("services")
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("apps.help.title")}</CardTitle>
        <CardDescription>{t("apps.help.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
          <li>{t("apps.help.sameNetwork")}</li>
          <li>{t("apps.help.lan")}</li>
          <li>{t("apps.help.names")}</li>
          <li>{t("apps.help.urlBase")}</li>
        </ul>
      </CardContent>
    </Card>
  )
}
