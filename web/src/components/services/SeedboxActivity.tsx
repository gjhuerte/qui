/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { seedboxTone } from "@/lib/services-tone"
import { ToneBadge } from "@/components/services/shared"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useDateTimeFormatters } from "@/hooks/useDateTimeFormatters"
import { useSeedboxStatus } from "@/hooks/useServices"
import { Loader2 } from "lucide-react"
import { useTranslation } from "react-i18next"

export function SeedboxActivity() {
  const { t } = useTranslation("services")
  const { formatISOTimestamp } = useDateTimeFormatters()
  const { data: status, isLoading } = useSeedboxStatus()

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("seedbox.activity.title")}</CardTitle>
        <CardDescription>{t("seedbox.activity.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading || !status ? (
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        ) : status.contacts.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("seedbox.status.never")}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("seedbox.activity.time")}</TableHead>
                <TableHead>{t("seedbox.activity.source")}</TableHead>
                <TableHead>{t("seedbox.activity.result")}</TableHead>
                <TableHead>{t("seedbox.activity.ip")}</TableHead>
                <TableHead>{t("seedbox.activity.answer")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {status.contacts.map(contact => (
                <TableRow key={contact.id}>
                  <TableCell className="whitespace-nowrap text-xs">{formatISOTimestamp(contact.createdAt)}</TableCell>
                  <TableCell className="text-xs">{t([`seedbox.sources.${contact.source}`, "seedbox.sources.manual"])}</TableCell>
                  <TableCell>
                    <ToneBadge tone={seedboxTone(contact.state)}>
                      {t([`seedbox.states.${contact.state}`, "seedbox.states.rejected"])}
                    </ToneBadge>
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    {contact.ip || "—"}
                    {contact.asn ? <span className="text-muted-foreground"> · {t("seedbox.asn", { asn: contact.asn })}</span> : null}
                  </TableCell>
                  <TableCell className="text-xs">
                    {contact.message || "—"}
                    {contact.httpStatus ? <span className="text-muted-foreground"> ({contact.httpStatus})</span> : null}
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
