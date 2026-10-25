/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { InfoGrid, InfoItem, ToneBadge } from "@/components/services/shared"
import { seedboxTone } from "@/lib/services-tone"
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
import { SaveFooter } from "@/components/ui/save-footer"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import {
  useClearSeedboxMamId,
  useSeedboxStatus,
  useSetSeedboxMamId,
  useSyncSeedbox,
  useUpdateSeedboxSettings
} from "@/hooks/useServices"
import { useDateTimeFormatters } from "@/hooks/useDateTimeFormatters"
import { formatRelativeTime } from "@/lib/dateTimeUtils"
import type { SeedboxContact, SeedboxStatus } from "@/types"
import type { TFunction } from "i18next"
import { Loader2, RefreshCw, Trash } from "lucide-react"
import { useState } from "react"
import { Trans, useTranslation } from "react-i18next"
import { toast } from "sonner"

/** One sentence for a toast after a request to MAM. */
function describeContact(t: TFunction<"services">, contact: SeedboxContact): string {
  return t([`seedbox.outcomes.${contact.state}`, "seedbox.outcomes.rejected"], { ip: contact.ip ?? "—", message: contact.message ?? "" })
}

function notify(t: TFunction<"services">, contact: SeedboxContact) {
  const text = describeContact(t, contact)
  if (seedboxTone(contact.state) === "bad") toast.error(text)
  else toast.success(text)
}

export function SeedboxOverview() {
  const { data: status, isLoading } = useSeedboxStatus()
  if (isLoading || !status) {
    return <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
  }
  return (
    <div className="space-y-6">
      <StatusCard status={status} />
      <SessionCard status={status} />
      <ScheduleCard key={`${status.enabled}-${status.intervalSeconds}`} status={status} />
    </div>
  )
}

function StatusCard({ status }: { status: SeedboxStatus }) {
  const { t } = useTranslation("services")
  const { formatISOTimestamp } = useDateTimeFormatters()
  const sync = useSyncSeedbox()
  const latest = status.latest

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle>{t("seedbox.status.title")}</CardTitle>
          {latest && (
            <ToneBadge tone={seedboxTone(latest.state)}>
              {t([`seedbox.states.${latest.state}`, "seedbox.states.rejected"])}
            </ToneBadge>
          )}
        </div>
        <CardDescription>{t("seedbox.status.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {latest ? (
          <InfoGrid>
            <InfoItem label={t("seedbox.status.ip")} mono>{latest.ip || "—"}</InfoItem>
            <InfoItem label={t("seedbox.status.asn")}>
              {latest.asn ? t("seedbox.asn", { asn: latest.asn }) : "—"}{latest.asOrg && <span className="text-muted-foreground"> · {latest.asOrg}</span>}
            </InfoItem>
            <InfoItem label={t("seedbox.status.lastCheck")}>
              <span title={formatISOTimestamp(latest.createdAt)}>{formatRelativeTime(latest.createdAt)}</span>
            </InfoItem>
            <InfoItem label={t("seedbox.status.nextCheck")}>
              {status.nextContactAt ? formatRelativeTime(status.nextContactAt) : t("seedbox.status.paused")}
            </InfoItem>
            <InfoItem label={t("seedbox.status.answer")}>
              {latest.message || "—"}{latest.httpStatus ? <span className="text-muted-foreground"> {t("seedbox.httpStatus", { status: latest.httpStatus })}</span> : null}
            </InfoItem>
          </InfoGrid>
        ) : (
          <p className="text-sm text-muted-foreground">{t("seedbox.status.never")}</p>
        )}
        {latest?.state === "rejected" && <p className="text-sm text-destructive">{t("seedbox.status.rejectedHelp")}</p>}
        {latest?.state === "unreachable" && <p className="text-sm text-destructive">{t("seedbox.status.unreachableHelp")}</p>}
      </CardContent>
      <CardFooter>
        <Button
          variant="outline"
          size="sm"
          disabled={sync.isPending}
          onClick={() => sync.mutate(undefined, {
            onSuccess: (contact) => notify(t, contact),
            onError: (err) => toast.error(err instanceof Error ? err.message : String(err)),
          })}
        >
          {sync.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <RefreshCw className="mr-2 h-4 w-4" />}
          {t("seedbox.actions.updateNow")}
        </Button>
      </CardFooter>
    </Card>
  )
}

function SessionCard({ status }: { status: SeedboxStatus }) {
  const { t } = useTranslation("services")
  const [mamId, setMamId] = useState("")
  const save = useSetSeedboxMamId()
  const clear = useClearSeedboxMamId()

  const submit = () => {
    save.mutate(mamId, {
      onSuccess: (contact) => {
        setMamId("")
        notify(t, contact)
      },
      onError: (err) => toast.error(err instanceof Error ? err.message : String(err)),
    })
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle>{t("seedbox.session.title")}</CardTitle>
          <ToneBadge tone={status.mamId.configured ? "good" : "warn"}>
            {status.mamId.configured ? t("seedbox.session.configured", { masked: status.mamId.masked }) : t("seedbox.session.missing")}
          </ToneBadge>
        </div>
        <CardDescription>{t("seedbox.session.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <ol className="list-decimal space-y-1 pl-5 text-sm text-muted-foreground">
          <li>
            <Trans
              t={t}
              i18nKey="seedbox.session.steps.open"
              components={{ mam: <a href="https://www.myanonamouse.net/preferences/index.php?view=security" target="_blank" rel="noreferrer" className="font-medium text-primary underline-offset-4 hover:underline" /> }}
            />
          </li>
          <li>{t("seedbox.session.steps.ip", { ip: status.latest?.ip ?? "—" })}</li>
          <li>{t("seedbox.session.steps.asn")}</li>
          <li>{t("seedbox.session.steps.dynamic")}</li>
          <li>{t("seedbox.session.steps.paste")}</li>
        </ol>
        <div className="space-y-2">
          <Label htmlFor="seedbox-mam-id">{status.mamId.configured ? t("seedbox.session.replaceLabel") : t("seedbox.session.label")}</Label>
          <Textarea
            id="seedbox-mam-id"
            rows={3}
            value={mamId}
            onChange={(e) => setMamId(e.target.value)}
            placeholder={t("seedbox.session.placeholder")}
            autoComplete="off"
            spellCheck={false}
            className="font-mono text-xs"
          />
        </div>
      </CardContent>
      <CardFooter className="flex justify-between gap-2">
        {status.mamId.configured ? (
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="outline" size="sm" disabled={clear.isPending}>
                <Trash className="mr-2 h-4 w-4" />
                {t("seedbox.actions.remove")}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{t("seedbox.removeDialog.title")}</AlertDialogTitle>
                <AlertDialogDescription>{t("seedbox.removeDialog.description")}</AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>{t("common:actions.cancel")}</AlertDialogCancel>
                <AlertDialogAction
                  onClick={() => clear.mutate(undefined, {
                    onSuccess: () => toast.success(t("seedbox.toasts.removed")),
                    onError: (err) => toast.error(err instanceof Error ? err.message : String(err)),
                  })}
                >
                  {t("seedbox.actions.remove")}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        ) : <span />}
        <Button size="sm" onClick={submit} disabled={save.isPending || mamId.trim() === ""}>
          {save.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {t("seedbox.actions.saveAndUpdate")}
        </Button>
      </CardFooter>
    </Card>
  )
}

function ScheduleCard({ status }: { status: SeedboxStatus }) {
  const { t } = useTranslation("services")
  const update = useUpdateSeedboxSettings()
  const [enabled, setEnabled] = useState(status.enabled)
  const [minutes, setMinutes] = useState(String(Math.round(status.intervalSeconds / 60)))

  const save = () => {
    const value = Number(minutes)
    if (!Number.isInteger(value) || value < 1 || value > 1440) {
      toast.error(t("seedbox.schedule.invalid"))
      return
    }
    update.mutate({ enabled, intervalSeconds: value * 60 }, {
      onSuccess: () => toast.success(t("seedbox.toasts.scheduleSaved")),
      onError: (err) => toast.error(err instanceof Error ? err.message : String(err)),
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("seedbox.schedule.title")}</CardTitle>
        <CardDescription>{t("seedbox.schedule.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center justify-between gap-3">
          <div className="space-y-0.5">
            <Label htmlFor="seedbox-enabled" className="font-medium">{t("seedbox.schedule.enabled")}</Label>
            <p className="text-xs text-muted-foreground">{t("seedbox.schedule.enabledHelp")}</p>
          </div>
          <Switch id="seedbox-enabled" checked={enabled} onCheckedChange={setEnabled} />
        </div>
        <div className="max-w-xs space-y-2">
          <Label htmlFor="seedbox-interval">{t("seedbox.schedule.interval")}</Label>
          <Input
            id="seedbox-interval"
            type="number"
            min={1}
            max={1440}
            value={minutes}
            disabled={!enabled}
            onChange={(e) => setMinutes(e.target.value)}
          />
        </div>
      </CardContent>
      <SaveFooter pending={update.isPending} onSave={save} label={t("seedbox.schedule.save")} />
    </Card>
  )
}
