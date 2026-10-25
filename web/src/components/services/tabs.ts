/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { z } from "zod"

// i18n-namespace: services

/** Nav sections in display order. */
export const SERVICES_NAV_GROUPS = [
  { labelKey: "nav.apps", tabs: ["apps", "apps-issues", "apps-movies", "apps-series", "apps-subtitles", "apps-manage"] },
  { labelKey: "nav.mediaSync", tabs: ["media-servers", "media-servers-manage", "media-sync", "media-sync-accounts", "media-sync-activity"] },
  { labelKey: "nav.playlists", tabs: ["playlists", "playlist-services"] },
  { labelKey: "nav.gluetun", tabs: ["gluetun", "gluetun-logs", "gluetun-config", "gluetun-connection"] },
  { labelKey: "nav.seedbox", tabs: ["seedbox", "seedbox-activity"] },
] as const

export type ServicesTab = (typeof SERVICES_NAV_GROUPS)[number]["tabs"][number]

/** Tab values as they appear in the URL search parameter. The cast gives z.enum the tuple it needs. */
export const SERVICES_TABS = SERVICES_NAV_GROUPS.flatMap(group => group.tabs) as [ServicesTab, ...ServicesTab[]]

export const DEFAULT_SERVICES_TAB: ServicesTab = "apps"

/** An unknown tab value drops to undefined, and the route then shows the default tab. */
export const servicesSearchSchema = z.object({
  tab: z.enum(SERVICES_TABS).optional().catch(undefined),
})
