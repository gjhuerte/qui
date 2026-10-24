/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Accordion } from "@/components/ui/accordion"
import { FilterViewsSection } from "@/components/torrents/FilterViewsSection"
import { makeFilters } from "@/test/mockFilters"
import type { FilterView, TorrentFilters } from "@/types"
import { cleanup, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"

// Stable singletons: fresh objects per render loop effects (see web/AGENTS.md).
// updateMutate runs onSuccess itself so the toast, and therefore Undo, is reachable.
const mocks = vi.hoisted(() => ({
  updateMutate: vi.fn((_payload: unknown, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.()),
  createMutate: vi.fn(),
  deleteMutate: vi.fn(),
  toastSuccess: vi.fn(),
  views: [] as unknown[],
  search: {} as { q?: string },
  sorting: [] as { id: string; desc: boolean }[],
  columnFilters: [] as unknown[],
  setSorting: vi.fn(),
  setColumnFilters: vi.fn(),
  navigateWithSearch: vi.fn(),
}))

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => vi.fn(),
  useSearch: () => mocks.search,
}))
vi.mock("@/lib/router-search", () => ({ navigateWithSearch: mocks.navigateWithSearch }))
vi.mock("@/hooks/usePersistedColumnSorting", () => ({
  usePersistedColumnSorting: () => [mocks.sorting, mocks.setSorting],
}))
vi.mock("@/hooks/usePersistedColumnFilters", () => ({
  usePersistedColumnFilters: () => [mocks.columnFilters, mocks.setColumnFilters],
}))

vi.mock("@/hooks/useFilterViews", () => ({
  useFilterViews: () => ({ data: mocks.views }),
  useCreateFilterView: () => ({ mutate: mocks.createMutate, isPending: false }),
  useUpdateFilterView: () => ({ mutate: mocks.updateMutate, isPending: false }),
  useDeleteFilterView: () => ({ mutate: mocks.deleteMutate, isPending: false }),
}))

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock("sonner", () => ({ toast: { success: mocks.toastSuccess, error: vi.fn() } }))

const SAVED: FilterView = {
  id: 7,
  name: "Movies",
  filters: makeFilters({ status: ["downloading"] }),
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
}

function renderSection(selectedFilters: TorrentFilters, hasActiveFilters: boolean, views: FilterView[] = [SAVED], onApply = vi.fn()) {
  mocks.views = views
  return render(
    <Accordion type="multiple" defaultValue={["views"]}>
      <FilterViewsSection
        instanceId={1}
        selectedFilters={selectedFilters}
        onApply={onApply}
        hasActiveFilters={hasActiveFilters}
        triggerClassName=""
        contentClassName=""
        itemClassName=""
      />
    </Accordion>
  )
}

async function openViewMenu() {
  await userEvent.click(screen.getByLabelText("filterSidebar.views.actions"))
  return screen.getByText("filterSidebar.views.updateFilters")
}

function resetTableState() {
  mocks.search = {}
  mocks.sorting = []
  mocks.columnFilters = []
}

describe("FilterViewsSection update filters", () => {
  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
    resetTableState()
  })

  it("sends the current selection as a normalized snapshot, keeping the name", async () => {
    renderSection(makeFilters({ status: ["stalledUP", "downloading"] }), true)

    await userEvent.click(await openViewMenu())

    expect(mocks.updateMutate).toHaveBeenCalledTimes(1)
    const [payload] = mocks.updateMutate.mock.calls[0] as [{ id: number; data: { name: string; filters: TorrentFilters } }]
    expect(payload.id).toBe(7)
    expect(payload.data.name).toBe("Movies")
    // Sorted, not raw: a view written by update must compare equal to one
    // written by create for the same selection. Normalization itself is
    // covered by lib/filter-views.test.ts.
    expect(payload.data.filters.status).toEqual(["downloading", "stalledUP"])
  })

  it("undo writes the replaced filters back", async () => {
    renderSection(makeFilters({ status: ["stalledUP"] }), true)

    await userEvent.click(await openViewMenu())

    const [, toastOptions] = mocks.toastSuccess.mock.calls[0] as [string, { action: { onClick: () => void } }]
    toastOptions.action.onClick()

    expect(mocks.updateMutate).toHaveBeenCalledTimes(2)
    const [undone] = mocks.updateMutate.mock.calls[1] as [{ data: { name: string; filters: unknown } }]
    expect(undone.data.name).toBe("Movies")
    expect(undone.data.filters).toEqual(SAVED.filters)
  })

  it("does nothing when no filters are selected", async () => {
    renderSection(makeFilters(), false)

    await userEvent.click(await openViewMenu())

    expect(mocks.updateMutate).not.toHaveBeenCalled()
  })
})

const SIZE_FILTER = { columnId: "size", operation: "gt", value: "1", sizeUnit: "GiB" }

describe("FilterViewsSection search, sorting, and column filters", () => {
  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
    resetTableState()
  })

  it("saves the search, the sort, and the column filters with the sidebar filters", async () => {
    mocks.search = { q: "  ubuntu " }
    mocks.sorting = [{ id: "size", desc: true }]
    mocks.columnFilters = [SIZE_FILTER]
    renderSection(makeFilters({ status: ["downloading"] }), true)

    await userEvent.click(await openViewMenu())

    const [payload] = mocks.updateMutate.mock.calls[0] as [{ data: { filters: Record<string, unknown> } }]
    expect(payload.data.filters.search).toBe("ubuntu")
    expect(payload.data.filters.sorting).toEqual([{ id: "size", desc: true }])
    expect(payload.data.filters.columnFilters).toEqual([SIZE_FILTER])
  })

  it("can save a view with only a search", async () => {
    mocks.search = { q: "ubuntu" }
    renderSection(makeFilters(), false)

    await userEvent.click(await openViewMenu())

    expect(mocks.updateMutate).toHaveBeenCalledTimes(1)
  })

  it("restores the search, the sort, and the column filters of a view", async () => {
    mocks.search = { q: "old", modal: "x" } as { q?: string }
    const view: FilterView = { ...SAVED, filters: { ...SAVED.filters, search: "ubuntu", sorting: [{ id: "name", desc: false }], columnFilters: [] } as FilterView["filters"] }
    const onApply = vi.fn()
    renderSection(makeFilters(), false, [view], onApply)

    await userEvent.click(screen.getByText("Movies"))

    expect(onApply).toHaveBeenCalledTimes(1)
    expect(mocks.setSorting).toHaveBeenCalledWith([{ id: "name", desc: false }])
    expect(mocks.setColumnFilters).toHaveBeenCalledWith([])
    const [{ search }] = mocks.navigateWithSearch.mock.calls[0] as [{ search: Record<string, unknown> }]
    expect(search).toEqual({ q: "ubuntu", modal: "x" })
  })

  it("leaves search, sort, and column filters alone for a view saved before they were saved", async () => {
    renderSection(makeFilters(), false)

    await userEvent.click(screen.getByText("Movies"))

    expect(mocks.setSorting).not.toHaveBeenCalled()
    expect(mocks.setColumnFilters).not.toHaveBeenCalled()
    expect(mocks.navigateWithSearch).not.toHaveBeenCalled()
  })
})
