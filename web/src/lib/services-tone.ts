/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { SeedboxContactState, SyncRunStatus } from "@/types"

export type Tone = "good" | "warn" | "bad" | "neutral"

export function vpnTone(status?: string): Tone {
  if (status === "running") return "good"
  if (status === "stopped") return "warn"
  return status ? "bad" : "neutral"
}

export function seedboxTone(state?: SeedboxContactState): Tone {
  switch (state) {
    case "updated":
    case "unchanged":
      return "good"
    case "waiting":
    case "throttled":
      return "warn"
    case "rejected":
    case "unreachable":
      return "bad"
    default:
      return "neutral"
  }
}

export function syncRunTone(status?: SyncRunStatus): Tone {
  switch (status) {
    case "success":
      return "good"
    case "partial":
      return "warn"
    case "failed":
      return "bad"
    default:
      return "neutral"
  }
}
