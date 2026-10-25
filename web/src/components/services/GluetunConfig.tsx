/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useGluetunStatus, useGluetunVPNSettings } from "@/hooks/useServices"
import { Loader2 } from "lucide-react"
import { useTranslation } from "react-i18next"

export function GluetunConfig() {
  return (
    <div className="space-y-6">
      <EnvironmentCard />
      <VPNSettingsCard />
    </div>
  )
}

function EnvironmentCard() {
  const { t } = useTranslation("services")
  const { data: status, isLoading } = useGluetunStatus()
  const container = status?.docker.container

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("config.environment.title")}</CardTitle>
        <CardDescription>{t("config.environment.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        ) : !container ? (
          <p className="text-sm text-muted-foreground">{status?.docker.error ?? t("config.environment.unavailable")}</p>
        ) : container.environment.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("config.environment.empty")}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("config.environment.name")}</TableHead>
                <TableHead>{t("config.environment.value")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {container.environment.map(env => (
                <TableRow key={env.name}>
                  <TableCell className="font-mono text-xs">{env.name}</TableCell>
                  <TableCell className="font-mono text-xs break-all">
                    {env.redacted ? <Badge variant="secondary">{t("config.redacted")}</Badge> : env.value}
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

function VPNSettingsCard() {
  const { t } = useTranslation("services")
  const { data, error, isLoading } = useGluetunVPNSettings()

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("config.vpnSettings.title")}</CardTitle>
        <CardDescription>{t("config.vpnSettings.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        ) : error ? (
          <Alert variant="destructive">
            <AlertDescription>{error instanceof Error ? error.message : String(error)}</AlertDescription>
          </Alert>
        ) : (
          <pre className="max-h-[60vh] overflow-auto rounded-md border bg-muted/30 p-3 font-mono text-xs">
            {JSON.stringify(data, null, 2)}
          </pre>
        )}
      </CardContent>
    </Card>
  )
}
