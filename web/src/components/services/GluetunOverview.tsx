/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { InfoGrid, InfoItem, ToneBadge } from "@/components/services/shared"
import { vpnTone, type Tone } from "@/lib/services-tone"
import type { ServicesTab } from "@/components/services/tabs"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useDateTimeFormatters } from "@/hooks/useDateTimeFormatters"
import {
  useGluetunStatus,
  useRestartGluetunContainer,
  useRestartGluetunVPN,
  useSetGluetunVPNStatus
} from "@/hooks/useServices"
import { formatRelativeTime } from "@/lib/dateTimeUtils"
import { formatBytes } from "@/lib/utils"
import type { GluetunStatus } from "@/types"
import { AlertTriangle, Loader2, Play, Power, RefreshCw, RotateCw, Square } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

function healthTone(status?: string): Tone {
  if (status === "healthy") return "good"
  if (status === "starting") return "warn"
  return status ? "bad" : "neutral"
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}

export function GluetunOverview({ onNavigate }: { onNavigate: (tab: ServicesTab) => void }) {
  const { t } = useTranslation("services")
  const { data: status, isLoading, isFetching, refetch, error } = useGluetunStatus()

  if (isLoading) {
    return <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
  }
  if (error || !status) {
    return (
      <Alert variant="destructive">
        <AlertTriangle className="h-4 w-4" />
        <AlertTitle>{t("gluetun.loadFailed")}</AlertTitle>
        <AlertDescription>{errorMessage(error)}</AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="space-y-6">
      <RestartProgress status={status} />
      {(!status.control.reachable || !status.docker.available) && (
        <Alert className="border-border rounded-xl bg-card">
          <AlertTriangle className="h-4 w-4 text-amber-600 dark:text-amber-400" />
          <AlertTitle>{t("gluetun.partialTitle")}</AlertTitle>
          <AlertDescription className="space-y-1">
            {!status.control.reachable && <p>{t("gluetun.controlUnreachable", { error: status.control.error })}</p>}
            {!status.docker.available && <p>{t("gluetun.dockerUnavailable", { error: status.docker.error })}</p>}
            <p>
              <button type="button" className="font-medium text-primary underline-offset-4 hover:underline" onClick={() => onNavigate("gluetun-connection")}>
                {t("gluetun.openConnection")}
              </button>
            </p>
          </AlertDescription>
        </Alert>
      )}
      <VPNCard status={status} refreshing={isFetching} onRefresh={() => refetch()} />
      {status.docker.container && <ContainerCard status={status} />}
      {status.docker.available && <DependentsCard status={status} />}
    </div>
  )
}

function RestartProgress({ status }: { status: GluetunStatus }) {
  const { t } = useTranslation("services")
  const restart = status.restart
  if (!restart.startedAt) return null

  // Hide a finished restart after ten minutes.
  if (!restart.running && restart.finishedAt && Date.now() - new Date(restart.finishedAt).getTime() > 10 * 60_000) {
    return null
  }

  if (restart.running) {
    return (
      <Alert className="border-border rounded-xl bg-card">
        <Loader2 className="h-4 w-4 animate-spin" />
        <AlertTitle>{t("gluetun.restart.runningTitle")}</AlertTitle>
        <AlertDescription>
          {t([`gluetun.restart.steps.${restart.step ?? "gluetun"}`, "gluetun.restart.steps.gluetun"])}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <Alert variant={restart.error ? "destructive" : "default"} className="rounded-xl">
      <RotateCw className="h-4 w-4" />
      <AlertTitle>{restart.error ? t("gluetun.restart.failedTitle") : t("gluetun.restart.doneTitle")}</AlertTitle>
      <AlertDescription>
        {restart.error ?? t("gluetun.restart.doneDescription", { containers: (restart.containers ?? []).join(", ") })}
      </AlertDescription>
    </Alert>
  )
}

function VPNCard({ status, refreshing, onRefresh }: { status: GluetunStatus; refreshing: boolean; onRefresh: () => void }) {
  const { t } = useTranslation("services")
  const control = status.control
  const ip = control.publicIp
  const setVPN = useSetGluetunVPNStatus()
  const restartVPN = useRestartGluetunVPN()
  const location = [ip?.city, ip?.region, ip?.country].filter(Boolean).join(", ")
  const vpnRunning = control.vpnStatus === "running"

  const changeVPN = () => {
    const next = vpnRunning ? "stopped" : "running"
    setVPN.mutate(next, {
      onSuccess: () => toast.success(t(next === "running" ? "gluetun.toasts.vpnStarted" : "gluetun.toasts.vpnStopped")),
      onError: (err) => toast.error(errorMessage(err)),
    })
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle>{t("gluetun.vpn.title")}</CardTitle>
          <ToneBadge tone={control.reachable ? vpnTone(control.vpnStatus) : "bad"}>
            {control.reachable ? t([`states.${control.vpnStatus}`, "states.unknown"]) : t("states.unreachable")}
          </ToneBadge>
        </div>
        <CardDescription>{t("gluetun.vpn.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <InfoGrid>
          <InfoItem label={t("gluetun.vpn.publicIp")} mono>{ip?.ip || "—"}</InfoItem>
          <InfoItem label={t("gluetun.vpn.location")}>{location || "—"}</InfoItem>
          <InfoItem label={t("gluetun.vpn.organization")}>{ip?.organization || "—"}</InfoItem>
          <InfoItem label={t("gluetun.vpn.forwardedPort")} mono>
            {control.forwardedPorts.length > 0 ? control.forwardedPorts.join(", ") : "—"}
          </InfoItem>
          <InfoItem label={t("gluetun.vpn.dns")}>
            {control.dnsStatus ? t([`states.${control.dnsStatus}`, "states.unknown"]) : "—"}
          </InfoItem>
          <InfoItem label={t("gluetun.vpn.timezone")}>{ip?.timezone || "—"}</InfoItem>
          {ip?.hostname && <InfoItem label={t("gluetun.vpn.hostname")} mono>{ip.hostname}</InfoItem>}
          {ip?.location && <InfoItem label={t("gluetun.vpn.coordinates")} mono>{ip.location}</InfoItem>}
        </InfoGrid>
        {control.reachable && control.error && (
          <p className="mt-4 text-xs text-muted-foreground">{t("gluetun.vpn.partialErrors", { error: control.error })}</p>
        )}
      </CardContent>
      <CardFooter className="flex flex-wrap gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={!control.reachable || restartVPN.isPending}
          onClick={() => restartVPN.mutate(undefined, {
            onSuccess: () => toast.success(t("gluetun.toasts.vpnRestarted")),
            onError: (err) => toast.error(errorMessage(err)),
          })}
        >
          {restartVPN.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <RotateCw className="mr-2 h-4 w-4" />}
          {t("gluetun.actions.restartVpn")}
        </Button>
        <Button variant="outline" size="sm" disabled={!control.reachable || setVPN.isPending} onClick={changeVPN}>
          {vpnRunning ? <Square className="mr-2 h-4 w-4" /> : <Play className="mr-2 h-4 w-4" />}
          {vpnRunning ? t("gluetun.actions.stopVpn") : t("gluetun.actions.startVpn")}
        </Button>
        <RestartContainerButton status={status} />
        <Button variant="ghost" size="sm" className="ml-auto" disabled={refreshing} onClick={onRefresh}>
          <RefreshCw className={refreshing ? "mr-2 h-4 w-4 animate-spin" : "mr-2 h-4 w-4"} />
          {t("actions.refresh")}
        </Button>
      </CardFooter>
    </Card>
  )
}

function RestartContainerButton({ status }: { status: GluetunStatus }) {
  const { t } = useTranslation("services")
  const [open, setOpen] = useState(false)
  const [withDependents, setWithDependents] = useState(true)
  const restart = useRestartGluetunContainer()
  const dependents = status.docker.dependents

  const confirm = () => {
    restart.mutate(withDependents, {
      onSuccess: () => {
        setOpen(false)
        toast.success(t("gluetun.toasts.restartStarted"))
      },
      onError: (err) => toast.error(errorMessage(err)),
    })
  }

  return (
    <>
      <Button
        variant="outline"
        size="sm"
        disabled={!status.docker.available || status.restart.running}
        onClick={() => setOpen(true)}
      >
        <Power className="mr-2 h-4 w-4" />
        {t("gluetun.actions.restartContainer")}
      </Button>
      <AlertDialog open={open} onOpenChange={setOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("gluetun.restartDialog.title")}</AlertDialogTitle>
            <AlertDialogDescription>{t("gluetun.restartDialog.description")}</AlertDialogDescription>
          </AlertDialogHeader>
          {dependents.length > 0 && (
            <div className="space-y-2">
              <div className="flex items-start gap-2">
                <Checkbox
                  id="gluetun-restart-dependents"
                  checked={withDependents}
                  onCheckedChange={(checked) => setWithDependents(checked === true)}
                />
                <Label htmlFor="gluetun-restart-dependents" className="font-normal leading-snug">
                  {t("gluetun.restartDialog.withDependents", { containers: dependents.map(d => d.name).join(", ") })}
                </Label>
              </div>
              <p className="text-xs text-muted-foreground">{t("gluetun.restartDialog.dependentsHelp")}</p>
            </div>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={restart.isPending}>{t("common:actions.cancel")}</AlertDialogCancel>
            <Button onClick={confirm} disabled={restart.isPending}>
              {restart.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t("gluetun.restartDialog.confirm")}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

function ContainerCard({ status }: { status: GluetunStatus }) {
  const { t } = useTranslation("services")
  const { formatISOTimestamp } = useDateTimeFormatters()
  const container = status.docker.container!
  const stats = status.docker.stats
  const health = container.health

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle>{t("gluetun.container.title")}</CardTitle>
          <div className="flex gap-2">
            <ToneBadge tone={container.running ? "good" : "bad"}>
              {t([`states.${container.state}`, "states.unknown"])}
            </ToneBadge>
            {health && (
              <ToneBadge tone={healthTone(health.status)}>
                {t([`states.${health.status}`, "states.unknown"])}
              </ToneBadge>
            )}
          </div>
        </div>
        <CardDescription className="font-mono">{container.image}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <InfoGrid>
          <InfoItem label={t("gluetun.container.name")} mono>{container.name} ({container.id})</InfoItem>
          <InfoItem label={t("gluetun.container.started")}>
            {container.startedAt ? `${formatRelativeTime(container.startedAt)} · ${formatISOTimestamp(container.startedAt)}` : "—"}
          </InfoItem>
          <InfoItem label={t("gluetun.container.restartCount")}>{container.restartCount}</InfoItem>
          <InfoItem label={t("gluetun.container.restartPolicy")}>{container.restartPolicy || "—"}</InfoItem>
          <InfoItem label={t("gluetun.container.cpu")}>
            {stats ? `${stats.cpuPercent.toFixed(1)}%` : "—"}
            {container.cpuLimit > 0 && <span className="text-muted-foreground"> {t("gluetun.container.cpuLimit", { count: container.cpuLimit })}</span>}
          </InfoItem>
          <InfoItem label={t("gluetun.container.memory")}>
            {stats ? formatBytes(stats.memoryUsage) : "—"}
            {(container.memoryLimit > 0 || (stats && stats.memoryLimit > 0)) && (
              <span className="text-muted-foreground"> / {formatBytes(container.memoryLimit || stats?.memoryLimit || 0)}</span>
            )}
          </InfoItem>
          <InfoItem label={t("gluetun.container.network")}>
            {stats ? t("gluetun.container.networkValue", { rx: formatBytes(stats.rxBytes), tx: formatBytes(stats.txBytes) }) : "—"}
          </InfoItem>
          <InfoItem label={t("gluetun.container.processes")}>{stats ? stats.pids : "—"}</InfoItem>
          {container.capAdd.length > 0 && (
            <InfoItem label={t("gluetun.container.capabilities")} mono>{container.capAdd.join(", ")}</InfoItem>
          )}
          {container.error && <InfoItem label={t("gluetun.container.error")}>{container.error}</InfoItem>}
        </InfoGrid>

        {container.ports.length > 0 && (
          <div className="space-y-2">
            <h3 className="text-sm font-semibold">{t("gluetun.container.ports")}</h3>
            <div className="flex flex-wrap gap-2">
              {container.ports.map(port => (
                <span key={`${port.container}-${port.host}`} className="rounded-md border px-2 py-1 font-mono text-xs">
                  {port.host} → {port.container}
                </span>
              ))}
            </div>
          </div>
        )}

        {health && health.checks.length > 0 && (
          <div className="space-y-2">
            <h3 className="text-sm font-semibold">
              {t("gluetun.container.healthChecks")}
              {health.failingStreak > 0 && (
                <span className="ml-2 font-normal text-destructive">{t("gluetun.container.failingStreak", { count: health.failingStreak })}</span>
              )}
            </h3>
            <ul className="space-y-1 text-xs">
              {health.checks.map(check => (
                <li key={check.start} className="flex gap-2">
                  <span className={check.exitCode === 0 ? "text-emerald-600 dark:text-emerald-400" : "text-destructive"}>
                    {check.exitCode === 0 ? "✓" : "✗"}
                  </span>
                  <span className="shrink-0 text-muted-foreground">{formatRelativeTime(check.start)}</span>
                  <span className="truncate font-mono" title={check.output}>{check.output.split("\n")[0]}</span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function DependentsCard({ status }: { status: GluetunStatus }) {
  const { t } = useTranslation("services")
  const dependents = status.docker.dependents

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("gluetun.dependents.title")}</CardTitle>
        <CardDescription>{t("gluetun.dependents.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {dependents.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("gluetun.dependents.none")}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("gluetun.dependents.name")}</TableHead>
                <TableHead>{t("gluetun.dependents.image")}</TableHead>
                <TableHead>{t("gluetun.dependents.state")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {dependents.map(dependent => {
                const uptime = dependent.status
                return (
                  <TableRow key={dependent.id}>
                    <TableCell className="font-medium">{dependent.name}</TableCell>
                    <TableCell className="font-mono text-xs">{dependent.image}</TableCell>
                    <TableCell>
                      <ToneBadge tone={dependent.state === "running" ? "good" : "bad"}>
                        {t([`states.${dependent.state}`, "states.unknown"])}
                      </ToneBadge>
                      <span className="ml-2 text-xs text-muted-foreground">{uptime}</span>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}
