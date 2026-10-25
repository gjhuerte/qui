/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { api } from "@/lib/api"
import type { PlaylistAccount, PlaylistCompareRow, PlaylistLinkInput, PlaylistTrack } from "@/types"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

export const playlistKeys = {
  all: ["playlists"] as const,
  list: ["playlists", "list"] as const,
  detail: (id: number) => ["playlists", "detail", id] as const,
  services: ["playlists", "services"] as const,
  remote: (account: PlaylistAccount) => ["playlists", "remote", account.service, account.serverId ?? 0, account.accountId ?? ""] as const,
}

export function usePlaylists() {
  return useQuery({ queryKey: playlistKeys.list, queryFn: () => api.listPlaylists() })
}

export function usePlaylist(id: number | null) {
  return useQuery({ queryKey: playlistKeys.detail(id ?? 0), queryFn: () => api.getPlaylist(id!), enabled: id !== null })
}

export function usePlaylistServices() {
  return useQuery({ queryKey: playlistKeys.services, queryFn: () => api.getPlaylistServices() })
}

/** Playlists already on a service account; only fetched once the account is complete. */
export function useRemotePlaylists(account: PlaylistAccount | null) {
  return useQuery({
    queryKey: playlistKeys.remote(account ?? { service: "spotify" }),
    queryFn: () => api.listRemotePlaylists(account!),
    enabled: account !== null,
    retry: false,
  })
}

/** Every playlist write can change the list, the open playlist and the services. */
function usePlaylistMutation<TArgs, TResult>(fn: (args: TArgs) => Promise<TResult>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSettled: () => queryClient.invalidateQueries({ queryKey: playlistKeys.all }),
  })
}

export function useImportPlaylist() {
  return usePlaylistMutation((data: { text: string; name?: string; request?: string }) => api.importPlaylist(data))
}

export function useUpdatePlaylist() {
  return usePlaylistMutation(({ id, data }: { id: number; data: { name: string; description: string; tracks: PlaylistTrack[] } }) =>
    api.updatePlaylist(id, data))
}

export function useDeletePlaylist() {
  return usePlaylistMutation((id: number) => api.deletePlaylist(id))
}

export function useCreatePlaylistLink() {
  return usePlaylistMutation(({ playlistId, data }: { playlistId: number; data: PlaylistLinkInput }) => api.createPlaylistLink(playlistId, data))
}

export function useDeletePlaylistLink() {
  return usePlaylistMutation(({ playlistId, linkId }: { playlistId: number; linkId: number }) => api.deletePlaylistLink(playlistId, linkId))
}

export function useSyncPlaylistLink() {
  return usePlaylistMutation(({ playlistId, linkId }: { playlistId: number; linkId: number }) => api.syncPlaylistLink(playlistId, linkId))
}

/** Compare changes nothing, so it does not refresh any query. */
export function useComparePlaylistLink() {
  return useMutation({
    mutationFn: ({ playlistId, linkId }: { playlistId: number; linkId: number }) => api.comparePlaylistLink(playlistId, linkId),
  })
}

export function useApplyPlaylistCompare() {
  return usePlaylistMutation(({ playlistId, linkId, rows }: { playlistId: number; linkId: number; rows: PlaylistCompareRow[] }) =>
    api.applyPlaylistCompare(playlistId, linkId, rows))
}

export function useSaveSpotifyApp() {
  return usePlaylistMutation((data: { clientId: string; clientSecret: string }) => api.saveSpotifyApp(data))
}

export function useStartSpotifySignIn() {
  return useMutation({ mutationFn: () => api.startSpotifySignIn() })
}

export function useConnectSpotify() {
  return usePlaylistMutation((redirectUrl: string) => api.connectSpotify(redirectUrl))
}

export function useConnectYTMusic() {
  return usePlaylistMutation((headers: string) => api.connectYTMusic(headers))
}

export function useDisconnectPlaylistService() {
  return usePlaylistMutation((service: "spotify" | "ytmusic") => api.disconnectPlaylistService(service))
}
