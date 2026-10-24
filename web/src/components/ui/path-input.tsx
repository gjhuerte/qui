/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import * as PopoverPrimitive from "@radix-ui/react-popover"
import { CornerLeftUp, File, Folder, Loader2 } from "lucide-react"
import { forwardRef, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react"
import type { ComponentProps, FocusEvent, KeyboardEvent, MouseEvent } from "react"
import { useTranslation } from "react-i18next"

import { Input } from "@/components/ui/input"
import { useDirectoryContent } from "@/hooks/useDirectoryContent"
import { useInstanceCapabilities } from "@/hooks/useInstanceCapabilities"
import { useInstancePreferences } from "@/hooks/useInstancePreferences"
import { useIncognitoMode } from "@/lib/incognito"
import { baseName, isRootDirectory, normalizeDirectoryPath, parentDirectory, pathSeparator, splitBrowsePath } from "@/lib/paths"
import { cn } from "@/lib/utils"

// A flat directory with thousands of entries would render one button per entry.
const MAX_ENTRIES = 100

type Entry = { kind: "parent" | "dir" | "file"; path: string }

export type PathInputProps = Omit<ComponentProps<"input">, "value" | "onChange" | "type"> & {
  /** The qBittorrent instance whose file system the picker lists. */
  instanceId: number
  value: string
  onValueChange: (value: string) => void
  /** Also list files. Picking a file closes the list instead of opening the folder. */
  includeFiles?: boolean
  /** The folder to list while the field is empty. Defaults to the default save path of the instance. */
  rootPath?: string
}

/**
 * A path field with a folder browser, like Flood: a click opens a list of the
 * folders in the typed directory, a folder opens the next level, and ".." goes up.
 * Typing filters the list by name. qBittorrent 5.0 and later list the folders;
 * for an older qBittorrent, qui lists them itself when the instance has local or
 * SSH filesystem access. Without either, it is a plain text field.
 *
 * The list renders in a portal, so a dialog body that scrolls does not clip it.
 */
export const PathInput = forwardRef<HTMLInputElement, PathInputProps>(function PathInput(
  { instanceId, value, onValueChange, includeFiles = false, rootPath, onFocus, onClick, onBlur, onKeyDown, className, disabled, ...inputProps },
  forwardedRef
) {
  const { t } = useTranslation("common")
  const [incognitoMode] = useIncognitoMode()
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  useImperativeHandle(forwardedRef, () => inputRef.current as HTMLInputElement)
  // Set by a press inside the list: a press on its header or scrollbar can blur
  // the field, and that blur must not close the list.
  const pressInListRef = useRef(false)

  const [open, setOpen] = useState(false)
  const [highlighted, setHighlighted] = useState(-1)
  // True after the user types. Only a typed filter preselects a match: a value
  // that the form filled in, such as the current location, must not turn Enter
  // into "open this folder".
  const [typing, setTyping] = useState(false)

  const { data: capabilities } = useInstanceCapabilities(instanceId > 0 ? instanceId : null)
  const supported = Boolean(capabilities?.supportsFolderBrowsing ?? capabilities?.supportsPathAutocomplete) && instanceId > 0
  const active = supported && open && !disabled
  const { preferences } = useInstancePreferences(instanceId, { enabled: active && !value && !rootPath })

  const { dir: typedDir, filter } = splitBrowsePath(value)
  const dir = typedDir || normalizeDirectoryPath(rootPath || preferences?.save_path || "/")

  const dirs = useDirectoryContent(instanceId, dir, { enabled: active })
  const files = useDirectoryContent(instanceId, dir, { enabled: active && includeFiles, mode: "files" })
  const loading = dirs.isLoading || (includeFiles && files.isLoading)

  const entries = useMemo<Entry[]>(() => {
    const prefix = filter.toLowerCase()
    const matches = (path: string) => !prefix || baseName(path).toLowerCase().startsWith(prefix)
    const listed: Entry[] = [
      ...(dirs.data ?? []).filter(matches).map((path) => ({ kind: "dir" as const, path })),
      ...(includeFiles ? (files.data ?? []).filter(matches).map((path) => ({ kind: "file" as const, path })) : []),
    ].slice(0, MAX_ENTRIES)
    return !prefix && !isRootDirectory(dir) ? [{ kind: "parent", path: parentDirectory(dir) }, ...listed] : listed
  }, [dirs.data, files.data, includeFiles, filter, dir])

  // A new listing starts unselected, so Enter still submits the form until the
  // user picks an entry with the arrow keys. A typed filter preselects the first match.
  useEffect(() => {
    setHighlighted(typing && filter ? entries.findIndex((entry) => entry.kind !== "parent") : -1)
  }, [entries, filter, typing])

  // Keep the highlighted entry in view while the arrow keys move it.
  useEffect(() => {
    if (highlighted < 0) return
    listRef.current?.querySelector(`[data-index="${highlighted}"]`)?.scrollIntoView?.({ block: "nearest" })
  }, [highlighted])

  const choose = useCallback((entry: Entry) => {
    // The entry buttons keep the focus in the field, so no blur consumes the flag.
    pressInListRef.current = false
    setTyping(false)
    if (entry.kind === "file") {
      onValueChange(entry.path)
      setOpen(false)
      return
    }
    // A folder keeps the list open on its content, so the user can go deeper.
    onValueChange(entry.kind === "parent" ? entry.path : entry.path + pathSeparator(entry.path))
    inputRef.current?.focus()
  }, [onValueChange])

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (supported) {
      if (!open && event.key === "ArrowDown") {
        event.preventDefault()
        setOpen(true)
      } else if (open && entries.length > 0) {
        if (event.key === "ArrowDown") {
          event.preventDefault()
          setHighlighted((index) => (index + 1) % entries.length)
        } else if (event.key === "ArrowUp") {
          event.preventDefault()
          setHighlighted((index) => (index <= 0 ? entries.length - 1 : index - 1))
        } else if ((event.key === "Enter" || event.key === "Tab") && highlighted >= 0 && highlighted < entries.length) {
          event.preventDefault()
          choose(entries[highlighted])
        }
      }
    }
    onKeyDown?.(event)
  }

  const showList = active && (loading || entries.length > 0 || dirs.isFetched)

  return (
    <PopoverPrimitive.Root open={showList} onOpenChange={(next) => !next && setOpen(false)}>
      <PopoverPrimitive.Anchor asChild>
        <Input
          {...inputProps}
          ref={inputRef}
          type="text"
          autoComplete={supported ? "off" : inputProps.autoComplete}
          spellCheck={false}
          disabled={disabled}
          value={value}
          role={supported ? "combobox" : undefined}
          aria-expanded={supported ? showList : undefined}
          aria-autocomplete={supported ? "list" : undefined}
          className={cn(incognitoMode && "blur-sm select-none", className)}
          onChange={(event) => {
            onValueChange(event.target.value)
            setTyping(true)
            setOpen(true)
          }}
          onFocus={(event: FocusEvent<HTMLInputElement>) => {
            pressInListRef.current = false
            setTyping(false)
            setOpen(true)
            onFocus?.(event)
          }}
          onClick={(event: MouseEvent<HTMLInputElement>) => {
            setOpen(true)
            onClick?.(event)
          }}
          onBlur={(event: FocusEvent<HTMLInputElement>) => {
            // A blur from Tab or a click elsewhere closes the list. A press inside
            // the list leaves it open; a later press outside closes it.
            if (pressInListRef.current) {
              pressInListRef.current = false
            } else {
              setOpen(false)
            }
            onBlur?.(event)
          }}
          onKeyDown={handleKeyDown}
        />
      </PopoverPrimitive.Anchor>
      <PopoverPrimitive.Portal>
        <PopoverPrimitive.Content
          ref={listRef}
          role="listbox"
          side="bottom"
          align="start"
          sideOffset={4}
          // The focus stays in the field, so the user can keep typing.
          onOpenAutoFocus={(event) => event.preventDefault()}
          onCloseAutoFocus={(event) => event.preventDefault()}
          // A press on the field itself is not "outside": it must not close the list.
          onInteractOutside={(event) => {
            if (inputRef.current?.contains(event.target as Node)) event.preventDefault()
          }}
          onPointerDown={() => {
            pressInListRef.current = true
          }}
          style={{ width: "var(--radix-popper-anchor-width)" }}
          className="z-50 rounded-md border bg-popover text-popover-foreground shadow-md outline-none"
        >
          <div className={cn("truncate border-b px-3 py-1.5 font-mono text-xs text-muted-foreground", incognitoMode && "blur-sm")} title={incognitoMode ? undefined : dir}>
            {dir}
          </div>
          <div className="max-h-60 overflow-y-auto py-1">
            {entries.map((entry, index) => {
              const Icon = entry.kind === "parent" ? CornerLeftUp : entry.kind === "file" ? File : Folder
              return (
                <button
                  key={`${entry.kind}:${entry.path}`}
                  type="button"
                  role="option"
                  data-index={index}
                  aria-selected={highlighted === index}
                  title={incognitoMode || entry.kind === "parent" ? undefined : entry.path}
                  className={cn(
                    "flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm hover:bg-accent hover:text-accent-foreground",
                    highlighted === index && "bg-accent text-accent-foreground"
                  )}
                  // Keeps the focus in the field, so the blur handler does not close the list.
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => choose(entry)}
                >
                  <Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <span className={cn("truncate", incognitoMode && entry.kind !== "parent" && "blur-sm")}>
                    {entry.kind === "parent" ? ".." : baseName(entry.path)}
                  </span>
                </button>
              )
            })}
            {loading && (
              <div className="flex items-center gap-2 px-3 py-1.5 text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                {t("pathInput.loading")}
              </div>
            )}
            {!loading && entries.every((entry) => entry.kind === "parent") && (
              <div className="px-3 py-1.5 text-sm text-muted-foreground">
                {filter ? t("pathInput.noMatches") : t("pathInput.empty")}
              </div>
            )}
          </div>
        </PopoverPrimitive.Content>
      </PopoverPrimitive.Portal>
    </PopoverPrimitive.Root>
  )
})
