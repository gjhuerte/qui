/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { api } from "@/lib/api"
import type { GluetunSettingsInput } from "@/types"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

export const seedboxKeys = {
  status: ["seedbox", "status"] as const,
}

export const gluetunKeys = {
  all: ["gluetun"] as const,
  status: ["gluetun", "status"] as const,
  logs: (tail: number) => ["gluetun", "logs", tail] as const,
  vpnSettings: ["gluetun", "vpn-settings"] as const,
  settings: ["gluetun", "settings"] as const,
}

export function useSeedboxStatus() {
  return useQuery({
    queryKey: seedboxKeys.status,
    queryFn: () => api.getSeedboxStatus(),
    refetchInterval: 30_000,
  })
}

/** Every seedbox write changes the status, so each mutation refreshes it. */
function useSeedboxMutation<TArgs, TResult>(fn: (args: TArgs) => Promise<TResult>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSettled: () => queryClient.invalidateQueries({ queryKey: seedboxKeys.status }),
  })
}

export function useSetSeedboxMamId() {
  return useSeedboxMutation((mamId: string) => api.setSeedboxMamId(mamId))
}

export function useClearSeedboxMamId() {
  return useSeedboxMutation(() => api.clearSeedboxMamId())
}

export function useUpdateSeedboxSettings() {
  return useSeedboxMutation((data: { enabled: boolean; intervalSeconds: number }) => api.updateSeedboxSettings(data))
}

export function useSyncSeedbox() {
  return useSeedboxMutation(() => api.syncSeedbox())
}

export function useGluetunStatus() {
  return useQuery({
    queryKey: gluetunKeys.status,
    queryFn: () => api.getGluetunStatus(),
    // Poll faster while a container restart runs, so its progress shows.
    refetchInterval: (query) => (query.state.data?.restart.running ? 3_000 : 15_000),
    retry: false,
  })
}

export function useGluetunLogs(tail: number, autoRefresh: boolean) {
  return useQuery({
    queryKey: gluetunKeys.logs(tail),
    queryFn: () => api.getGluetunLogs(tail),
    refetchInterval: autoRefresh ? 5_000 : false,
    retry: false,
  })
}

export function useGluetunVPNSettings() {
  return useQuery({
    queryKey: gluetunKeys.vpnSettings,
    queryFn: () => api.getGluetunVPNSettings(),
    retry: false,
  })
}

export function useGluetunSettings() {
  return useQuery({
    queryKey: gluetunKeys.settings,
    queryFn: () => api.getGluetunSettings(),
  })
}

function useGluetunMutation<TArgs, TResult>(fn: (args: TArgs) => Promise<TResult>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSettled: () => queryClient.invalidateQueries({ queryKey: gluetunKeys.all }),
  })
}

export function useUpdateGluetunSettings() {
  return useGluetunMutation((data: GluetunSettingsInput) => api.updateGluetunSettings(data))
}

export function useSetGluetunVPNStatus() {
  return useGluetunMutation((status: "running" | "stopped") => api.setGluetunVPNStatus(status))
}

export function useRestartGluetunVPN() {
  return useGluetunMutation(() => api.restartGluetunVPN())
}

export function useRestartGluetunContainer() {
  return useGluetunMutation((withDependents: boolean) => api.restartGluetunContainer(withDependents))
}
