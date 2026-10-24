/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { ColumnFilter } from "@/lib/column-filter-utils"
import type { SavedViewState, TorrentFilters } from "@/types"
import type { SortingState } from "@tanstack/react-table"

const FILTER_ARRAY_KEYS = [
  "status",
  "excludeStatus",
  "categories",
  "excludeCategories",
  "tags",
  "excludeTags",
  "trackers",
  "excludeTrackers",
] as const

/**
 * Normalizes a filter object into the snapshot a saved view stores: every array
 * key present and sorted, `expr` normalized, and the derived `expandedCategories`
 * / `expandedExcludeCategories` fields dropped (FilterSidebar recomputes those).
 *
 * The result is also what applying a view feeds back in, so missing keys reset
 * instead of merging with whatever was selected before.
 */
export function toViewFilters(filters: Partial<TorrentFilters> | undefined): TorrentFilters {
  const normalized = { expr: filters?.expr || "" } as TorrentFilters
  for (const key of FILTER_ARRAY_KEYS) {
    normalized[key] = [...(filters?.[key] ?? [])].sort()
  }
  return normalized
}

/** Deep-equality of two filter selections, ignoring array order and expanded* fields. */
export function filterViewsEqual(a: Partial<TorrentFilters> | undefined, b: Partial<TorrentFilters> | undefined): boolean {
  // ponytail: key order is fixed by toViewFilters, so stringify is a safe deep compare
  return JSON.stringify(toViewFilters(a)) === JSON.stringify(toViewFilters(b))
}

/** The selection after the user clears every sidebar filter. */
export const EMPTY_SIDEBAR_FILTERS: TorrentFilters = toViewFilters(undefined)

/** Reports whether any sidebar filter is set. A blank expression filters nothing. */
export function hasSidebarFilters(filters: Partial<TorrentFilters>): boolean {
  return FILTER_ARRAY_KEYS.some(key => (filters[key]?.length ?? 0) > 0) || Boolean(filters.expr?.trim())
}

/** The table state that a view saves next to the sidebar filters. */
export interface ViewTableState {
  search: string
  sorting: SortingState
  columnFilters: ColumnFilter[]
}

const normalizeTableState = (state: Partial<ViewTableState>): ViewTableState => ({
  search: state.search?.trim() ?? "",
  sorting: (state.sorting ?? []).map(({ id, desc }) => ({ id, desc })),
  columnFilters: state.columnFilters ?? [],
})

/** The snapshot a view saves: the sidebar filters plus search, sorting, and column filters. */
export function toViewSnapshot(filters: Partial<TorrentFilters> | undefined, table: ViewTableState): SavedViewState {
  return { ...toViewFilters(filters), ...normalizeTableState(table) }
}

/**
 * Views saved before search, sorting, and column filters were part of a view
 * carry none of them. Applying such a view leaves those settings alone.
 */
export function viewHasTableState(view: SavedViewState): boolean {
  return view.search !== undefined || view.sorting !== undefined || view.columnFilters !== undefined
}

/** The table state of a view, with the missing parts cleared. */
export function viewTableStateOf(view: SavedViewState): ViewTableState {
  return normalizeTableState(view)
}

/** Reports whether the current filters, search, sort, and column filters match a saved view. */
export function viewMatches(view: SavedViewState, filters: Partial<TorrentFilters>, table: ViewTableState): boolean {
  if (!filterViewsEqual(filters, view)) return false
  if (!viewHasTableState(view)) return true
  return JSON.stringify(viewTableStateOf(view)) === JSON.stringify(normalizeTableState(table))
}

/** Reports whether the search, sort, or column filters hold anything worth saving. */
export function hasTableState(table: ViewTableState): boolean {
  return table.search.trim() !== "" || table.sorting.length > 0 || table.columnFilters.length > 0
}
