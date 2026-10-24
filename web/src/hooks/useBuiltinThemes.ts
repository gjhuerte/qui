/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { registerBuiltinThemes, getThemeById, getDefaultTheme, type Theme } from "@/config/themes"
import { parseThemeCSS } from "@/utils/themeParser"
import { setTheme } from "@/utils/theme"
import type { BuiltinTheme } from "@/types"

function toTheme(entry: BuiltinTheme): Theme | null {
  const parsed = parseThemeCSS(entry.css)
  if (!parsed) {
    console.warn(`Failed to parse built-in theme: ${entry.id}`)
    return null
  }
  return {
    id: entry.id,
    name: parsed.metadata.name,
    description: parsed.metadata.description,
    lightOnly: parsed.metadata.lightOnly,
    variations: parsed.variations,
    cssVars: parsed.cssVars,
  }
}

/**
 * Registers a fetched theme payload in the client registry and re-applies the
 * stored selection: it may only just have become resolvable, and the boot
 * paint may have used a stale cached copy of the theme.
 */
export function applyBuiltinThemesPayload(payload: { themes: BuiltinTheme[] }): void {
  registerBuiltinThemes(payload.themes.map(toTheme).filter((t): t is Theme => t !== null))

  // A fresh profile has no stored selection; its boot paint used the bundled
  // fallback, whose values can lag the server's default theme, so the fetched
  // default repaints it.
  const targetId = localStorage.getItem("color-theme") ?? getDefaultTheme().id
  if (getThemeById(targetId)) {
    // System change: restoring existing state must not be pushed.
    void setTheme(targetId, undefined, undefined, true)
  }
}

/**
 * Query for the built-in theme list. Public endpoint, so it also themes the
 * login page. Registration happens inside the queryFn so the registry is
 * populated before any observer re-renders on the committed data; an effect
 * would leave the first isSuccess render reading the pre-registration array.
 */
export function useBuiltinThemes() {
  const query = useQuery({
    queryKey: ["builtin-themes"],
    queryFn: async ({ signal }) => {
      const payload = await api.getBuiltinThemes(signal)
      applyBuiltinThemesPayload(payload)
      return payload
    },
    // The built-in themes are embedded in the binary, so they change only on an update.
    staleTime: Infinity,
    retry: 1,
  })

  return { data: query.data, isSuccess: query.isSuccess, isError: query.isError }
}
