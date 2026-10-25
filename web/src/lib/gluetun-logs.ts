/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { GluetunLogLine } from "@/types"

export type GluetunLogFilter = "all" | "warn" | "error"

/** Keeps lines at or above the chosen level whose module or message contains the search text. */
export function filterGluetunLogs(lines: GluetunLogLine[], filter: GluetunLogFilter, search: string): GluetunLogLine[] {
  const needle = search.trim().toLowerCase()
  return lines.filter(line => {
    if (filter === "error" && line.level !== "error") return false
    if (filter === "warn" && line.level !== "warn" && line.level !== "error") return false
    if (!needle) return true
    return line.message.toLowerCase().includes(needle) || (line.module?.toLowerCase().includes(needle) ?? false)
  })
}
