/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { getColumnType, type ColumnFilter } from "@/lib/column-filter-utils"
import { columnLabel } from "@/lib/torrent-table/column-label"
import { useSortable } from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"
import { flexRender } from "@tanstack/react-table"
import { ChevronDown, ChevronUp } from "lucide-react"
import type { MouseEvent as ReactMouseEvent } from "react"
import { ColumnFilterPopover } from "./ColumnFilterPopover"
import { useMediaQuery } from "@/hooks/useMediaQuery"
import type { ViewMode } from "@/hooks/usePersistedCompactViewState"
import type { TorrentTableHeader } from "./tanstackTableFeatures"

{/* Auto-fit width for column headers on double-click*/}
const TORRENT_ROW_CELL_MEASURE = "data-torrent-column-measure"

function escapeColumnIdForSelector(columnId: string): string {
  const cssEscape = (globalThis as unknown as { CSS?: { escape?: (id: string) => string } }).CSS?.escape
  return typeof cssEscape === "function" ? cssEscape(columnId) : columnId.replace(/\\/g, "\\\\").replace(/"/g, "\\\"")
}

function measureNaturalOuterWidth(source: HTMLElement): number {
  const body = source.ownerDocument.body
  if (!body) {
    return source.scrollWidth
  }

  const clone = source.cloneNode(true) as HTMLElement
  clone.style.cssText = "position:absolute;visibility:hidden;pointer-events:none;left:-99999px;top:0;width:max-content;max-width:none;min-width:min-content;height:auto;flex-shrink:0;flex-grow:0;box-sizing:border-box"

  body.appendChild(clone)
  const rect = clone.getBoundingClientRect()
  const widthPx = Math.max(clone.scrollWidth, rect.width)
  body.removeChild(clone)
  return widthPx
}

function measureTorrentColumnFitWidth(gridRoot: HTMLElement, columnId: string): number | null {
  const nodes = gridRoot.querySelectorAll<HTMLElement>(
    `[${TORRENT_ROW_CELL_MEASURE}="${escapeColumnIdForSelector(columnId)}"]`
  )
  if (nodes.length === 0) {
    return null
  }

  let maxContent = 0
  nodes.forEach((el) => {
    maxContent = Math.max(maxContent, measureNaturalOuterWidth(el))
  })

  return maxContent
}
{/* Auto-fit width for column headers on double-click end*/}

interface DraggableTableHeaderProps {
  header: TorrentTableHeader
  columnFilters?: ColumnFilter[]
  viewMode?: ViewMode
  onFilterChange?: (columnId: string, filter: ColumnFilter | null) => void
  stretched?: boolean
  // The header to the left, whose resize edge this header's left grab zone extends.
  previousHeader?: TorrentTableHeader
  previousStretched?: boolean
}

// Double-click on a resize edge: fit the column to its widest cell.
function fitColumnToContent(event: ReactMouseEvent<HTMLElement>, header: TorrentTableHeader) {
  const { column } = header
  const table = header.getContext().table
  event.preventDefault()
  event.stopPropagation()
  const gridRoot = event.currentTarget.closest("[role=\"grid\"]") as HTMLElement | null
  const measured = gridRoot ? measureTorrentColumnFitWidth(gridRoot, column.id) : null
  if (measured === null) {
    column.resetSize()
    return
  }
  const minSize = column.columnDef.minSize ?? 20
  const maxSize = column.columnDef.maxSize ?? Number.MAX_SAFE_INTEGER
  const nextWidth = Math.min(Math.max(Math.ceil(measured), minSize), maxSize)
  table.setColumnSizing(prev => ({
    ...prev,
    [column.id]: nextWidth,
  }))
}

export function DraggableTableHeader({ header, columnFilters = [], viewMode = "normal", onFilterChange, stretched = false, previousHeader, previousStretched = false }: DraggableTableHeaderProps) {
  const { column } = header

  const isSelectHeader = column.id === "select"
  const isPriorityHeader = column.id === "priority"
  const isTrackerIconHeader = column.id === "tracker_icon"
  const isStatusIconHeader = column.id === "status_icon"
  const isCompactHeader = isTrackerIconHeader || isStatusIconHeader

  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({
    id: column.id,
    disabled: column.id === "select",
  })
  const table = header.getContext().table
  const trackerColumn = isTrackerIconHeader ? table.getColumn("tracker") : null

  // A stretched column's width is derived, so resizing it would only store an invisible value.
  const canResize = column.getCanResize() && !stretched
  // The cell clips its children, so the grab zone of the edge on the left of
  // this cell lives here and drives the previous column.
  const canResizePrevious = !!previousHeader && previousHeader.column.getCanResize() && !previousStretched
  const shouldShowSeparator = canResize || stretched || column.columnDef.enableResizing === false
  const shouldShowSortIndicator = !isSelectHeader && column.getIsSorted() && (isPriorityHeader || !isCompactHeader)
  const canSort = column.getCanSort() || (!!trackerColumn && trackerColumn.getCanSort())
  const toggleSortingHandler = column.getToggleSortingHandler()
  const trackerToggleHandler = trackerColumn?.getToggleSortingHandler()
  const columnHasActiveFilter = columnFilters.some(f => f.columnId === column.id)
  const primaryInputCanHover = useMediaQuery("(hover: hover)")
  const hideFilterUntilHover = primaryInputCanHover && !columnHasActiveFilter
  const columnFilterIconVisibilityClassName = hideFilterUntilHover ? "hidden group-hover:block focus-within:block [&:has(button[data-state=open])]:block" : undefined
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.8 : 1,
    position: "relative" as const,
    width: header.getSize(),
    flexShrink: 0,
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      className="group overflow-hidden"
    >
      <div
        className={`${viewMode === "dense" ? "px-2 h-7 text-xs" : "px-3 h-10 text-sm"} text-left font-medium text-muted-foreground flex items-center ${canSort ? "cursor-pointer select-none" : ""
        } ${column.id !== "select" ? "cursor-grab active:cursor-grabbing" : ""
        }`}
        onClick={event => {
          if (column.id === "select" || !canSort) {
            return
          }

          if (isTrackerIconHeader && trackerToggleHandler) {
            trackerToggleHandler(event)
            return
          }

          if (toggleSortingHandler) {
            toggleSortingHandler(event)
          }
        }}
        {...(column.id !== "select" ? attributes : {})}
        {...(column.id !== "select" ? listeners : {})}
      >
        {/* Header content */}
        <div
          className={`flex items-center ${isCompactHeader ? "gap-0" : "gap-1"} flex-1 min-w-0 ${isSelectHeader || isCompactHeader ? "justify-center" : ""
          }`}
        >
          <span
            className={`whitespace-nowrap ${!isPriorityHeader && !isCompactHeader ? "overflow-hidden flex-1 min-w-0" : ""
            } ${isCompactHeader ? "flex items-center w-full justify-center" : ""
            } ${isSelectHeader ? "flex items-center justify-center" : ""}`}
          >
            {header.isPlaceholder ? null : flexRender(
              column.columnDef.header,
              header.getContext()
            )}
          </span>
          {/* Column filter button - only show for filterable columns */}
          {!isSelectHeader && !isPriorityHeader && !isTrackerIconHeader && !isStatusIconHeader && onFilterChange && (
            <span className={columnFilterIconVisibilityClassName}>
              <ColumnFilterPopover
                columnId={column.id}
                columnName={columnLabel(column)}
                columnType={getColumnType(column.id)}
                currentFilter={columnFilters.find(f => f.columnId === column.id)}
                onApply={(filter) => onFilterChange(column.id, filter)}
              />
            </span>
          )}
          {shouldShowSortIndicator && (
            column.getIsSorted() === "asc" ? (
              <ChevronUp className={`h-4 w-4 flex-shrink-0${isPriorityHeader ? " ml-1 mr-1" : ""}`} />
            ) : (
              <ChevronDown className={`h-4 w-4 flex-shrink-0${isPriorityHeader ? " ml-1 mr-1" : ""}`} />
            )
          )}
        </div>
      </div>

      {/* Grab zone for the edge on the left: resizes the previous column */}
      {canResizePrevious && previousHeader && (
        <div
          aria-hidden="true"
          data-resize-zone="previous"
          onMouseDown={previousHeader.getResizeHandler()}
          onTouchStart={previousHeader.getResizeHandler()}
          onDoubleClick={(event) => fitColumnToContent(event, previousHeader)}
          className="absolute left-0 top-0 z-10 h-full w-2 cursor-col-resize select-none touch-none"
        />
      )}

      {/* Resize handle */}
      {shouldShowSeparator && (
        <div
          onMouseDown={canResize ? header.getResizeHandler() : undefined}
          onTouchStart={canResize ? header.getResizeHandler() : undefined}
          onDoubleClick={(event) => {
            if (canResize) fitColumnToContent(event, header)
          }}
          // 12px of grab area left of the line, plus the 8px zone in the next cell.
          className={`absolute right-0 top-0 z-10 h-full w-3 select-none group/resize flex justify-end ${canResize ? "cursor-col-resize touch-none" : "pointer-events-none"
          }`}
        >
          <div
            className={`h-full w-px ${canResize && column.getIsResizing() ? "bg-primary" : canResize ? "bg-border group-hover/resize:bg-primary/50" : "bg-border"
            }`}
          />
        </div>
      )}
    </div>
  )
}
