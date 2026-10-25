/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { InstanceFilter } from "@/lib/media-library"
import { useState } from "react"

export const PAGE_SIZE = 100

/** Shared filter state; any change goes back to the first page. */
export function useLibraryControls<F extends string>(initial: F) {
  const [search, setSearch] = useState("")
  const [filter, setFilter] = useState<F>(initial)
  const [instance, setInstance] = useState<InstanceFilter>("all")
  const [page, setPage] = useState(0)
  return {
    search,
    onSearch: (value: string) => {
      setSearch(value)
      setPage(0)
    },
    filter,
    onFilter: (value: F) => {
      setFilter(value)
      setPage(0)
    },
    instance,
    onInstance: (value: InstanceFilter) => {
      setInstance(value)
      setPage(0)
    },
    page,
    onPage: setPage,
  }
}

/** Fetches the library again past the server cache. */
export function useLibraryRefresh(load: () => Promise<unknown>) {
  const [refreshing, setRefreshing] = useState(false)
  const refresh = async () => {
    setRefreshing(true)
    try {
      await load()
    } finally {
      setRefreshing(false)
    }
  }
  return { refreshing, refresh }
}
