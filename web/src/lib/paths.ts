/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

/** Drive-letter and UNC paths, which is what a Windows qBittorrent returns. A backslash elsewhere is a valid Unix file name character. */
function isWindowsPath(path: string): boolean {
  return /^([a-zA-Z]:|\\\\)/.test(path)
}

export function pathSeparator(path: string): "/" | "\\" {
  return isWindowsPath(path) ? "\\" : "/"
}

/** Index of the last separator: "/" on every host, "\" too on a Windows path. */
export function lastSeparatorIndex(path: string): number {
  const slash = path.lastIndexOf("/")
  return isWindowsPath(path) ? Math.max(slash, path.lastIndexOf("\\")) : slash
}

export function endsWithSeparator(path: string): boolean {
  return path.length > 0 && lastSeparatorIndex(path) === path.length - 1
}

/** Normalizes a directory path for a consistent cache key: rooted, with one trailing separator. */
export function normalizeDirectoryPath(dirPath: string): string {
  if (!dirPath) return ""
  if (isWindowsPath(dirPath)) return dirPath.replace(/[\\/]*$/, "\\")
  return (dirPath.startsWith("/") ? dirPath : `/${dirPath}`).replace(/\/*$/, "/")
}

/** The last segment of a path, without a trailing separator. */
export function baseName(path: string): string {
  const trimmed = endsWithSeparator(path) ? path.slice(0, -1) : path
  return trimmed.slice(lastSeparatorIndex(trimmed) + 1)
}

/** True for "/", a drive root such as "C:\", and a bare UNC share root. */
export function isRootDirectory(dirPath: string): boolean {
  const dir = normalizeDirectoryPath(dirPath)
  return dir === "/" || /^[a-zA-Z]:\\$/.test(dir) || /^\\\\[^\\]+\\[^\\]+\\$/.test(dir)
}

/** The parent of a directory, with a trailing separator. A root is its own parent. */
export function parentDirectory(dirPath: string): string {
  const dir = normalizeDirectoryPath(dirPath)
  if (!dir || isRootDirectory(dir)) return dir
  const withoutTrailing = dir.slice(0, -1)
  return withoutTrailing.slice(0, lastSeparatorIndex(withoutTrailing) + 1) || "/"
}

/**
 * Splits a typed path into the directory to list and the name prefix to filter by:
 * "/data/mo" lists "/data/" filtered by "mo", and "/data/" lists "/data/" unfiltered.
 */
export function splitBrowsePath(path: string): { dir: string; filter: string } {
  if (!path) return { dir: "", filter: "" }
  if (endsWithSeparator(path)) return { dir: normalizeDirectoryPath(path), filter: "" }
  const index = lastSeparatorIndex(path)
  if (index === -1) return { dir: "/", filter: path }
  return { dir: normalizeDirectoryPath(path.slice(0, index + 1)), filter: path.slice(index + 1) }
}
