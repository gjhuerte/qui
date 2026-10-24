/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { afterEach, describe, expect, it } from "vitest"
import { getIncognitoTags, getLinuxCategory, getLinuxIsoName, getLinuxSavePath, getLinuxTrackerDomain } from "./incognito"

afterEach(() => {
  document.documentElement.removeAttribute("data-theme")
})

const HASH = "abcdef1234567890abcdef1234567890abcdef12"

describe("incognito flavors", () => {
  it("serves linux vocabulary by default", () => {
    expect(getLinuxIsoName(HASH)).toMatch(/\.iso$/)
    expect(getLinuxTrackerDomain(HASH)).toBeTruthy()
    expect(getLinuxSavePath(HASH)).toBeTruthy()
    expect(getIncognitoTags().length).toBeGreaterThan(0)
  })

  it("is deterministic per hash", () => {
    expect(getLinuxIsoName(HASH)).toBe(getLinuxIsoName(HASH))
    expect(getLinuxCategory(HASH)).toBe(getLinuxCategory(HASH))
  })
})
