/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { describe, expect, it } from "vitest"

import { baseName, isRootDirectory, normalizeDirectoryPath, parentDirectory, splitBrowsePath } from "./paths"

describe("normalizeDirectoryPath", () => {
  it.each([
    ["", ""],
    ["/data", "/data/"],
    ["/data//", "/data/"],
    ["data", "/data/"],
    ["C:\\data", "C:\\data\\"],
    ["C:\\data\\", "C:\\data\\"],
    ["C:", "C:\\"],
    ["\\\\server\\share", "\\\\server\\share\\"],
    ["/data/ac\\dc", "/data/ac\\dc/"],
    ["/data/ac\\dc/", "/data/ac\\dc/"],
  ])("%j -> %j", (input, expected) => {
    expect(normalizeDirectoryPath(input)).toBe(expected)
  })
})

describe("parentDirectory", () => {
  it.each([
    ["/data/movies/", "/data/"],
    ["/data/movies", "/data/"],
    ["/data/", "/"],
    ["/", "/"],
    ["C:\\data\\movies\\", "C:\\data\\"],
    ["C:\\data\\", "C:\\"],
    ["C:\\", "C:\\"],
    ["\\\\server\\share\\", "\\\\server\\share\\"],
  ])("%j -> %j", (input, expected) => {
    expect(parentDirectory(input)).toBe(expected)
  })
})

describe("isRootDirectory", () => {
  it.each([
    ["/", true],
    ["C:\\", true],
    ["\\\\server\\share", true],
    ["/data", false],
    ["C:\\data", false],
  ])("%j -> %j", (input, expected) => {
    expect(isRootDirectory(input)).toBe(expected)
  })
})

describe("splitBrowsePath", () => {
  it.each([
    ["", { dir: "", filter: "" }],
    ["/data/", { dir: "/data/", filter: "" }],
    ["/data/mo", { dir: "/data/", filter: "mo" }],
    ["/da", { dir: "/", filter: "da" }],
    ["C:\\data\\mo", { dir: "C:\\data\\", filter: "mo" }],
  ])("%j", (input, expected) => {
    expect(splitBrowsePath(input)).toEqual(expected)
  })
})

describe("baseName", () => {
  it.each([
    ["/data/movies", "movies"],
    ["/data/movies/", "movies"],
    ["C:\\data\\movies", "movies"],
  ])("%j -> %j", (input, expected) => {
    expect(baseName(input)).toBe(expected)
  })
})
