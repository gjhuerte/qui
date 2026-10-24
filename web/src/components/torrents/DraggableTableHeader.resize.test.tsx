/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { cleanup, fireEvent, render } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"

import { DraggableTableHeader } from "./DraggableTableHeader"
import type { TorrentTableHeader } from "./tanstackTableFeatures"

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }))
vi.mock("@dnd-kit/sortable", () => ({
  useSortable: () => ({ attributes: {}, listeners: {}, setNodeRef: () => {}, transform: null, transition: undefined, isDragging: false }),
}))
vi.mock("./ColumnFilterPopover", () => ({ ColumnFilterPopover: () => null }))

afterEach(cleanup)

function fakeHeader(id: string, resize: (event: unknown) => void) {
  const column = {
    id,
    columnDef: { header: id, minSize: 20 },
    getCanResize: () => true,
    getIsResizing: () => false,
    getIsSorted: () => false,
    getCanSort: () => false,
    getToggleSortingHandler: () => undefined,
    resetSize: vi.fn(),
  }
  return {
    id,
    column,
    isPlaceholder: true,
    getSize: () => 120,
    getResizeHandler: () => resize,
    getContext: () => ({ table: { getColumn: () => null, setColumnSizing: vi.fn() } }),
  } as unknown as TorrentTableHeader
}

const rightHandle = (container: HTMLElement) => container.querySelector(".cursor-col-resize:not([data-resize-zone])") as HTMLElement
const leftZone = (container: HTMLElement) => container.querySelector("[data-resize-zone=\"previous\"]") as HTMLElement | null

describe("DraggableTableHeader resize grab zones", () => {
  it("resizes its own column from the wider zone on its right edge", () => {
    const resize = vi.fn()
    const { container } = render(<DraggableTableHeader header={fakeHeader("size", resize)} />)
    expect(rightHandle(container).className).toContain("w-3")
    fireEvent.mouseDown(rightHandle(container))
    expect(resize).toHaveBeenCalledTimes(1)
  })

  it("resizes the previous column from the zone on its left edge", () => {
    const own = vi.fn()
    const previous = vi.fn()
    const { container } = render(<DraggableTableHeader header={fakeHeader("size", own)} previousHeader={fakeHeader("name", previous)} />)
    fireEvent.mouseDown(leftZone(container)!)
    expect(previous).toHaveBeenCalledTimes(1)
    expect(own).not.toHaveBeenCalled()
  })

  it("has no left zone for the first column or after a stretched column", () => {
    const first = render(<DraggableTableHeader header={fakeHeader("name", vi.fn())} />)
    expect(leftZone(first.container)).toBeNull()
    cleanup()
    const afterStretched = render(<DraggableTableHeader header={fakeHeader("size", vi.fn())} previousHeader={fakeHeader("name", vi.fn())} previousStretched />)
    expect(leftZone(afterStretched.container)).toBeNull()
  })
})
