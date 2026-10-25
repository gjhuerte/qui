/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { ToneBadge } from "@/components/services/shared"
import type { ServicesTab } from "@/components/services/tabs"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { appKeys, useAppStatus } from "@/hooks/useServices"
import { api } from "@/lib/api"
import { collectIssues } from "@/lib/media-library"
import { cn } from "@/lib/utils"
import { useQueryClient } from "@tanstack/react-query"
import { CheckCircle2, ExternalLink, Loader2, RefreshCw } from "lucide-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"

export function AppsIssues({ onNavigate }: { onNavigate: (tab: ServicesTab) => void }) {
  const { t } = useTranslation("services")
  const queryClient = useQueryClient()
  const { data, isLoading } = useAppStatus()
  const [refreshing, setRefreshing] = useState(false)
  const issues = useMemo(() => collectIssues(data ?? []), [data])

  const refresh = async () => {
    setRefreshing(true)
    try {
      queryClient.setQueryData(appKeys.status, await api.getAppStatus(true))
    } finally {
      setRefreshing(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle>{t("issues.title")}</CardTitle>
          <Button variant="ghost" size="sm" onClick={refresh} disabled={refreshing}>
            <RefreshCw className={cn("mr-2 h-4 w-4", refreshing && "animate-spin")} />
            {t("actions.refresh")}
          </Button>
        </div>
        <CardDescription>{t("issues.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        ) : !data || data.length === 0 ? (
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">{t("apps.empty.description")}</p>
            <Button size="sm" onClick={() => onNavigate("apps-manage")}>{t("apps.actions.add")}</Button>
          </div>
        ) : issues.length === 0 ? (
          <p className="flex items-center gap-2 text-sm text-emerald-700 dark:text-emerald-400">
            <CheckCircle2 className="h-4 w-4" />
            {t("issues.none", { count: data.length })}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-28">{t("issues.columns.severity")}</TableHead>
                <TableHead className="w-40">{t("issues.columns.app")}</TableHead>
                <TableHead>{t("issues.columns.issue")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {issues.map((issue, i) => (
                <TableRow key={i}>
                  <TableCell>
                    <ToneBadge tone={issue.severity === "error" ? "bad" : "warn"}>{t(`issues.severity.${issue.severity}`)}</ToneBadge>
                  </TableCell>
                  <TableCell>
                    <a href={issue.app.baseUrl} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 font-medium hover:underline">
                      {issue.app.name}
                      <ExternalLink className="h-3 w-3 opacity-60" />
                    </a>
                    <p className="text-xs text-muted-foreground">{t(`apps.types.${issue.app.type}`)}</p>
                  </TableCell>
                  <TableCell>
                    <p className="text-xs font-medium text-muted-foreground">{t(`issues.kinds.${issue.kind}`)}</p>
                    <p className="text-sm break-words">{issue.title}</p>
                    {issue.detail && <p className="text-xs text-muted-foreground break-words">{issue.detail}</p>}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}
