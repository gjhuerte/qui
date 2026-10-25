/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { ServicesTab } from "@/components/services/tabs"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { SearchInput } from "@/components/ui/SearchInput"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { formatRelativeTime } from "@/lib/dateTimeUtils"
import { PAGE_SIZE } from "@/hooks/useLibraryControls"
import type { InstanceFilter } from "@/lib/media-library"
import { cn } from "@/lib/utils"
import type { AppLibrary } from "@/types"
import { AlertTriangle, ChevronLeft, ChevronRight, Loader2, RefreshCw } from "lucide-react"
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"

interface LibraryShellProps<T, F extends string> {
  title: string
  description: string
  library?: AppLibrary<T>
  isLoading: boolean
  error: unknown
  refreshing: boolean
  onRefresh: () => void
  /** Shown when no instance of the app is enabled. */
  noInstancesText: string
  onNavigate: (tab: ServicesTab) => void
  search: string
  onSearch: (value: string) => void
  filter: F
  filters: readonly { value: F; label: string }[]
  onFilter: (value: F) => void
  instance: InstanceFilter
  onInstance: (value: InstanceFilter) => void
  shown: number
  page: number
  onPage: (page: number) => void
  children: ReactNode
}

export function LibraryShell<T, F extends string>(props: LibraryShellProps<T, F>) {
  const { t } = useTranslation("services")
  const { library, isLoading, error, page, shown } = props
  const pages = Math.max(1, Math.ceil(shown / PAGE_SIZE))

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle>{props.title}</CardTitle>
          <Button variant="ghost" size="sm" onClick={props.onRefresh} disabled={props.refreshing || isLoading}>
            <RefreshCw className={cn("mr-2 h-4 w-4", props.refreshing && "animate-spin")} />
            {t("actions.refresh")}
          </Button>
        </div>
        <CardDescription>
          {props.description}
          {library && <span> {t("library.loaded", { when: formatRelativeTime(library.loadedAt) })}</span>}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {isLoading ? (
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        ) : error ? (
          <Alert variant="destructive">
            <AlertTriangle className="h-4 w-4" />
            <AlertDescription>{error instanceof Error ? error.message : String(error)}</AlertDescription>
          </Alert>
        ) : !library || library.instances.length === 0 ? (
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">{props.noInstancesText}</p>
            <Button size="sm" onClick={() => props.onNavigate("apps-manage")}>{t("apps.actions.add")}</Button>
          </div>
        ) : (
          <>
            {library.errors.length > 0 && (
              <Alert className="rounded-xl">
                <AlertTriangle className="h-4 w-4 text-amber-600 dark:text-amber-400" />
                <AlertTitle>{t("library.partialTitle")}</AlertTitle>
                <AlertDescription>
                  {library.errors.map(e => <p key={e.id}><span className="font-medium">{e.name}</span>: {e.error}</p>)}
                </AlertDescription>
              </Alert>
            )}
            <div className="flex flex-wrap items-center gap-3">
              <SearchInput
                value={props.search}
                onChange={(event) => props.onSearch(event.target.value)}
                onClear={() => props.onSearch("")}
                placeholder={t("library.search")}
                className="w-full sm:w-64"
              />
              <Select value={props.filter} onValueChange={(value) => props.onFilter(value as F)}>
                <SelectTrigger className="w-52" aria-label={t("library.filter")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {props.filters.map(f => <SelectItem key={f.value} value={f.value}>{f.label}</SelectItem>)}
                </SelectContent>
              </Select>
              {library.instances.length > 1 && (
                <Select value={String(props.instance)} onValueChange={(value) => props.onInstance(value === "all" ? "all" : Number(value))}>
                  <SelectTrigger className="w-44" aria-label={t("library.instance")}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t("library.allInstances")}</SelectItem>
                    {library.instances.map(i => <SelectItem key={i.id} value={String(i.id)}>{i.name}</SelectItem>)}
                  </SelectContent>
                </Select>
              )}
              <span className="ml-auto text-sm text-muted-foreground">
                {t("library.count", { shown, total: library.items.length })}
              </span>
            </div>
            {props.children}
            {pages > 1 && (
              <div className="flex items-center justify-end gap-2">
                <Button variant="outline" size="icon" className="h-8 w-8" disabled={page === 0} onClick={() => props.onPage(page - 1)} aria-label={t("library.previous")}>
                  <ChevronLeft className="h-4 w-4" />
                </Button>
                <span className="text-sm text-muted-foreground">{t("library.page", { page: page + 1, pages })}</span>
                <Button variant="outline" size="icon" className="h-8 w-8" disabled={page >= pages - 1} onClick={() => props.onPage(page + 1)} aria-label={t("library.next")}>
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
