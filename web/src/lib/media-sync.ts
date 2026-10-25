/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { MediaServer, MediaServerUser, SyncDirection, SyncPair, SyncRun, SyncUserMap } from "@/types"

export const DIRECTION_ARROWS: Record<SyncDirection, string> = {
  both: "⇄",
  a_to_b: "→",
  b_to_a: "←",
}

/** "Jellyfin ⇄ Plex", with "#id" for a server that no longer exists. */
export function pairLabel(pair: SyncPair, servers: MediaServer[] | undefined): string {
  const name = (id: number) => servers?.find(s => s.id === id)?.name ?? `#${id}`
  return `${name(pair.serverAId)} ${DIRECTION_ARROWS[pair.direction]} ${name(pair.serverBId)}`
}

/** The newest run of each pair. Runs arrive newest first. */
export function latestRunByPair(runs: SyncRun[] | undefined): Map<number, SyncRun> {
  const latest = new Map<number, SyncRun>()
  for (const run of runs ?? []) {
    if (!latest.has(run.pairId)) latest.set(run.pairId, run)
  }
  return latest
}

function nameKey(name: string): string {
  return name.toLowerCase().replace(/[^\p{L}\p{N}]/gu, "")
}

/**
 * Adds a map for each account on server A whose name matches an account on
 * server B, ignoring case, spaces and punctuation. Accounts already mapped
 * on either side are left alone.
 */
export function autoMatchUsers(usersA: MediaServerUser[], usersB: MediaServerUser[], current: SyncUserMap[]): SyncUserMap[] {
  const usedA = new Set(current.map(m => m.userAId))
  const usedB = new Set(current.map(m => m.userBId))
  const byName = new Map<string, MediaServerUser>()
  for (const b of usersB) {
    const key = nameKey(b.name)
    if (key && !usedB.has(b.id) && !byName.has(key)) byName.set(key, b)
  }
  const added: SyncUserMap[] = []
  for (const a of usersA) {
    if (usedA.has(a.id)) continue
    const b = byName.get(nameKey(a.name))
    if (!b) continue
    byName.delete(nameKey(a.name))
    added.push({ userAId: a.id, userAName: a.name, userBId: b.id, userBName: b.name })
  }
  return [...current, ...added]
}
