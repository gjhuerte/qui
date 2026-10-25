/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useMediaServers, useMediaServerUsers } from "@/hooks/useMediaSync"
import type { MediaServerType } from "@/types"
import { useState } from "react"

/**
 * Picks a Jellyfin or Plex server and an account on it. Until the user
 * chooses, the first server and its first admin account are picked.
 */
export function usePlaylistAccountChoice(types: readonly MediaServerType[]) {
  const { data: allServers, isLoading: loadingServers } = useMediaServers()
  const servers = (allServers ?? []).filter(s => s.enabled && types.includes(s.type))
  const [pickedServer, setPickedServer] = useState<number | undefined>(undefined)
  const server = servers.find(s => s.id === pickedServer) ?? servers[0]

  const users = useMediaServerUsers(server?.id)
  const [pickedAccount, setPickedAccount] = useState<string | undefined>(undefined)
  const list = users.data ?? []
  const account = list.find(u => u.id === pickedAccount) ?? list.find(u => u.admin) ?? list[0]

  return {
    servers,
    loadingServers,
    server,
    selectServer: (id: number) => {
      setPickedServer(id)
      setPickedAccount(undefined)
    },
    users,
    account,
    selectAccount: setPickedAccount,
  }
}

export type PlaylistAccountChoice = ReturnType<typeof usePlaylistAccountChoice>
