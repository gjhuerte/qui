/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import type { PlaylistAccountChoice } from "@/hooks/usePlaylistAccount"
import { Loader2 } from "lucide-react"
import { useTranslation } from "react-i18next"

/** The server and account pickers of a Jellyfin or Plex account. */
export function PlaylistAccountFields({ choice, idPrefix }: { choice: PlaylistAccountChoice; idPrefix: string }) {
  const { t } = useTranslation("services")
  const { servers, server, users, account } = choice

  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <div className="space-y-2">
        <Label htmlFor={`${idPrefix}-server`}>{t("playlists.account.server")}</Label>
        <Select value={server ? String(server.id) : undefined} onValueChange={(v) => choice.selectServer(Number(v))}>
          <SelectTrigger id={`${idPrefix}-server`}>
            <SelectValue placeholder={t("playlists.account.noServer")} />
          </SelectTrigger>
          <SelectContent>
            {servers.map(s => {
              const type = t(`mediaSync.types.${s.type}`)
              return <SelectItem key={s.id} value={String(s.id)}>{s.name === type ? s.name : `${s.name} (${type})`}</SelectItem>
            })}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label htmlFor={`${idPrefix}-account`}>{t("playlists.account.account")}</Label>
        {users.isLoading ? (
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        ) : users.error ? (
          <p className="text-sm text-destructive break-words">{users.error.message}</p>
        ) : (
          <Select value={account?.id} onValueChange={choice.selectAccount} disabled={!server}>
            <SelectTrigger id={`${idPrefix}-account`}>
              <SelectValue placeholder={t("mediaSync.accounts.pick")} />
            </SelectTrigger>
            <SelectContent>
              {(users.data ?? []).map(u => (
                <SelectItem key={u.id} value={u.id}>
                  {u.name}{u.admin && <span className="ml-1 text-xs text-muted-foreground">{t("mediaSync.accounts.admin")}</span>}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </div>
    </div>
  )
}
