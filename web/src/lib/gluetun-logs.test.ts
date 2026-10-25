/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { describe, expect, it } from "vitest"

import { filterGluetunLogs } from "@/lib/gluetun-logs"
import type { GluetunLogLine } from "@/types"

const lines: GluetunLogLine[] = [
  { level: "info", module: "wireguard", message: "Connecting to 1.2.3.4" },
  { level: "warn", module: "dns", message: "slow answer" },
  { level: "error", module: "vpn", message: "connection failed" },
  { level: "other", message: "=====" },
]

describe("filterGluetunLogs", () => {
  it("keeps every line for all levels", () => {
    expect(filterGluetunLogs(lines, "all", "")).toHaveLength(4)
  })

  it("keeps warnings and errors", () => {
    expect(filterGluetunLogs(lines, "warn", "").map(l => l.level)).toEqual(["warn", "error"])
  })

  it("keeps errors only", () => {
    expect(filterGluetunLogs(lines, "error", "").map(l => l.module)).toEqual(["vpn"])
  })

  it("searches messages and modules, ignoring case", () => {
    expect(filterGluetunLogs(lines, "all", "WIREGUARD")).toHaveLength(1)
    expect(filterGluetunLogs(lines, "all", "failed")).toHaveLength(1)
  })
})
