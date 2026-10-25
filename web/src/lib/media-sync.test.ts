/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { autoMatchUsers, latestRunByPair, pairLabel } from "@/lib/media-sync"
import type { MediaServer, SyncPair, SyncRun } from "@/types"
import { describe, expect, it } from "vitest"

const user = (id: string, name: string) => ({ id, name, admin: false })

describe("autoMatchUsers", () => {
  it("maps accounts whose names match, ignoring case and punctuation", () => {
    const maps = autoMatchUsers(
      [user("a1", "Ann"), user("a2", "Bob Smith"), user("a3", "Kids")],
      [user("b1", "bob.smith"), user("b2", "ANN"), user("b3", "guest")],
      []
    )
    expect(maps).toEqual([
      { userAId: "a1", userAName: "Ann", userBId: "b2", userBName: "ANN" },
      { userAId: "a2", userAName: "Bob Smith", userBId: "b1", userBName: "bob.smith" },
    ])
  })

  it("keeps existing maps and skips accounts already mapped", () => {
    const current = [{ userAId: "a1", userAName: "Ann", userBId: "b9", userBName: "annie" }]
    const maps = autoMatchUsers([user("a1", "Ann"), user("a2", "Bob")], [user("b2", "Ann"), user("b9", "Bob")], current)
    expect(maps).toEqual(current)
  })
})

describe("pairLabel", () => {
  const servers = [{ id: 1, name: "Jellyfin" }, { id: 2, name: "Plex" }] as MediaServer[]
  it("shows the direction between the server names", () => {
    expect(pairLabel({ serverAId: 1, serverBId: 2, direction: "a_to_b" } as SyncPair, servers)).toBe("Jellyfin → Plex")
    expect(pairLabel({ serverAId: 1, serverBId: 3, direction: "both" } as SyncPair, servers)).toBe("Jellyfin ⇄ #3")
  })
})

describe("latestRunByPair", () => {
  it("keeps the first run seen for each pair", () => {
    const runs = [{ id: 3, pairId: 1 }, { id: 2, pairId: 2 }, { id: 1, pairId: 1 }] as SyncRun[]
    const latest = latestRunByPair(runs)
    expect(latest.get(1)?.id).toBe(3)
    expect(latest.get(2)?.id).toBe(2)
  })
})
