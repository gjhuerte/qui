/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { SearchInput } from "@/components/ui/SearchInput"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { useDateTimeFormatters } from "@/hooks/useDateTimeFormatters"
import { useGluetunLogs } from "@/hooks/useServices"
import { filterGluetunLogs, type GluetunLogFilter } from "@/lib/gluetun-logs"
import { cn, copyTextToClipboard } from "@/lib/utils"
import type { GluetunLogLevel } from "@/types"
import { AlertTriangle, Copy, Loader2, RefreshCw } from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

const TAIL_OPTIONS = [200, 500, 1000, 2000, 5000] as const
const FILTERS: GluetunLogFilter[] = ["all", "warn", "error"]

const levelClass: Record<GluetunLogLevel, string> = {
  debug: "text-muted-foreground",
  info: "text-sky-600 dark:text-sky-400",
  warn: "text-amber-600 dark:text-amber-400",
  error: "text-destructive",
  other: "text-muted-foreground",
}

export function GluetunLogs() {
  const { t } = useTranslation("services")
  const { formatISOTimestamp } = useDateTimeFormatters()
  const [tail, setTail] = useState<number>(500)
  const [filter, setFilter] = useState<GluetunLogFilter>("all")
  const [search, setSearch] = useState("")
  const [autoRefresh, setAutoRefresh] = useState(true)
  const { data, error, isLoading, isFetching, refetch } = useGluetunLogs(tail, autoRefresh)
  const scrollRef = useRef<HTMLDivElement>(null)

  const lines = useMemo(() => filterGluetunLogs(data?.lines ?? [], filter, search), [data, filter, search])
  const counts = useMemo(() => {
    const all = data?.lines ?? []
    return { error: all.filter(l => l.level === "error").length, warn: all.filter(l => l.level === "warn").length }
  }, [data])

  // Keep the newest line in view while the user is at the bottom.
  const stickToBottom = useRef(true)
  useEffect(() => {
    const el = scrollRef.current
    if (el && stickToBottom.current) el.scrollTop = el.scrollHeight
  }, [lines])

  const copy = async () => {
    const text = lines.map(l => [l.time, l.level.toUpperCase(), l.module ? `[${l.module}]` : "", l.message].filter(Boolean).join(" ")).join("\n")
    try {
      await copyTextToClipboard(text)
      toast.success(t("logs.copied"))
    } catch {
      toast.error(t("logs.copyFailed"))
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("logs.title")}</CardTitle>
        <CardDescription>{t("logs.description", { errors: counts.error, warnings: counts.warn })}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-center gap-3">
          <SearchInput
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t("logs.searchPlaceholder")}
            className="w-full sm:w-64"
          />
          <Select value={filter} onValueChange={(value) => setFilter(value as GluetunLogFilter)}>
            <SelectTrigger className="w-40" aria-label={t("logs.levelLabel")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {FILTERS.map(f => <SelectItem key={f} value={f}>{t(`logs.filters.${f}`)}</SelectItem>)}
            </SelectContent>
          </Select>
          <Select value={String(tail)} onValueChange={(value) => setTail(Number(value))}>
            <SelectTrigger className="w-36" aria-label={t("logs.tailLabel")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {TAIL_OPTIONS.map(n => <SelectItem key={n} value={String(n)}>{t("logs.tailOption", { count: n })}</SelectItem>)}
            </SelectContent>
          </Select>
          <div className="flex items-center gap-2">
            <Switch id="gluetun-logs-auto" checked={autoRefresh} onCheckedChange={setAutoRefresh} />
            <Label htmlFor="gluetun-logs-auto" className="font-normal">{t("logs.autoRefresh")}</Label>
          </div>
          <div className="ml-auto flex gap-2">
            <Button variant="outline" size="sm" onClick={copy} disabled={lines.length === 0}>
              <Copy className="mr-2 h-4 w-4" />
              {t("logs.copy")}
            </Button>
            <Button variant="outline" size="sm" onClick={() => refetch()} disabled={isFetching}>
              <RefreshCw className={cn("mr-2 h-4 w-4", isFetching && "animate-spin")} />
              {t("actions.refresh")}
            </Button>
          </div>
        </div>

        {error ? (
          <Alert variant="destructive">
            <AlertTriangle className="h-4 w-4" />
            <AlertTitle>{t("logs.loadFailed")}</AlertTitle>
            <AlertDescription>{error instanceof Error ? error.message : String(error)}</AlertDescription>
          </Alert>
        ) : isLoading ? (
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        ) : (
          <div
            ref={scrollRef}
            onScroll={(event) => {
              const el = event.currentTarget
              stickToBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40
            }}
            className="h-[60vh] overflow-auto rounded-md border bg-muted/30 p-3 font-mono text-xs leading-relaxed"
          >
            {lines.length === 0 ? (
              <p className="text-muted-foreground">{t("logs.empty")}</p>
            ) : lines.map((line, index) => (
              <div key={index} className="flex gap-2 whitespace-pre-wrap break-all">
                {line.time && <span className="shrink-0 text-muted-foreground">{formatISOTimestamp(line.time)}</span>}
                <span className={cn("w-12 shrink-0 font-semibold", levelClass[line.level])}>
                  {t([`logs.levels.${line.level}`, "logs.levels.other"])}
                </span>
                {line.module && <span className="shrink-0 text-muted-foreground">[{line.module}]</span>}
                <span className={line.level === "error" ? "text-destructive" : undefined}>{line.message}</span>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
