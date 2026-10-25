/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { api } from "@/lib/api"
import type { MediaServerInput, SyncPairInput, SyncUserMap } from "@/types"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

export const mediaSyncKeys = {
  all: ["media-sync"] as const,
  status: ["media-sync", "status"] as const,
  servers: ["media-sync", "servers"] as const,
  users: (serverId: number) => ["media-sync", "users", serverId] as const,
  pairs: ["media-sync", "pairs"] as const,
  maps: (pairId: number) => ["media-sync", "maps", pairId] as const,
  runs: ["media-sync", "runs"] as const,
  run: (id: number) => ["media-sync", "run", id] as const,
}

export function useMediaServerStatus() {
  return useQuery({
    queryKey: mediaSyncKeys.status,
    queryFn: () => api.getMediaServerStatus(),
    refetchInterval: 30_000,
  })
}

export function useMediaServers() {
  return useQuery({ queryKey: mediaSyncKeys.servers, queryFn: () => api.listMediaServers() })
}

export function useMediaServerUsers(serverId: number | undefined) {
  return useQuery({
    queryKey: mediaSyncKeys.users(serverId ?? 0),
    queryFn: () => api.listMediaServerUsers(serverId!),
    enabled: serverId !== undefined,
    retry: false,
  })
}

export function useSyncPairs() {
  return useQuery({ queryKey: mediaSyncKeys.pairs, queryFn: () => api.listSyncPairs() })
}

export function useSyncUserMaps(pairId: number | undefined) {
  return useQuery({
    queryKey: mediaSyncKeys.maps(pairId ?? 0),
    queryFn: () => api.getSyncUserMaps(pairId!),
    enabled: pairId !== undefined,
  })
}

/** Polls fast while a run is in progress, so the log shows its outcome soon. */
export function useSyncRuns() {
  return useQuery({
    queryKey: mediaSyncKeys.runs,
    queryFn: () => api.listSyncRuns(),
    refetchInterval: (query) => (query.state.data?.some(r => r.status === "running") ? 3_000 : 30_000),
  })
}

export function useSyncRun(id: number | null) {
  return useQuery({
    queryKey: mediaSyncKeys.run(id ?? 0),
    queryFn: () => api.getSyncRun(id!),
    enabled: id !== null,
  })
}

function useMediaSyncMutation<TArgs, TResult>(fn: (args: TArgs) => Promise<TResult>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSettled: () => queryClient.invalidateQueries({ queryKey: mediaSyncKeys.all }),
  })
}

export function useSaveMediaServer() {
  return useMediaSyncMutation(({ id, data }: { id?: number; data: MediaServerInput }) =>
    id ? api.updateMediaServer(id, data) : api.createMediaServer(data))
}

export function useDeleteMediaServer() {
  return useMediaSyncMutation((id: number) => api.deleteMediaServer(id))
}

export function useTestMediaServer() {
  return useMutation({ mutationFn: (data: MediaServerInput & { id?: number }) => api.testMediaServer(data) })
}

export function useSaveSyncPair() {
  return useMediaSyncMutation(({ id, data }: { id?: number; data: SyncPairInput }) =>
    id ? api.updateSyncPair(id, data) : api.createSyncPair(data))
}

export function useDeleteSyncPair() {
  return useMediaSyncMutation((id: number) => api.deleteSyncPair(id))
}

export function useSaveSyncUserMaps() {
  return useMediaSyncMutation(({ pairId, maps }: { pairId: number; maps: SyncUserMap[] }) => api.setSyncUserMaps(pairId, maps))
}

export function useRunSyncPair() {
  return useMediaSyncMutation((pairId: number) => api.runSyncPair(pairId))
}

export function useCancelSyncPair() {
  return useMediaSyncMutation((pairId: number) => api.cancelSyncPair(pairId))
}
