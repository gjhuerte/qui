/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { useState } from "react"
import { afterEach, describe, expect, it, vi } from "vitest"

import { PathInput } from "./path-input"

const state = vi.hoisted(() => ({ supported: true, savePath: "/data" }))

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }))
vi.mock("@/lib/incognito", () => ({ useIncognitoMode: () => [false, () => {}] }))
vi.mock("@/hooks/useInstanceCapabilities", () => ({
  useInstanceCapabilities: () => ({ data: { supportsPathAutocomplete: false, supportsFolderBrowsing: state.supported } }),
}))
vi.mock("@/hooks/useInstancePreferences", () => ({
  useInstancePreferences: () => ({ preferences: { save_path: state.savePath } }),
}))
vi.mock("@/hooks/useDirectoryContent", () => {
  const listings: Record<string, { dirs: string[]; files: string[] }> = {
    "/": { dirs: ["/data", "/srv"], files: [] },
    "/data/": { dirs: ["/data/movies", "/data/music", "/data/tv"], files: ["/data/notes.txt"] },
    "/data/movies/": { dirs: ["/data/movies/2024"], files: [] },
    "C:\\data\\": { dirs: ["C:\\data\\movies"], files: [] },
  }
  return {
    useDirectoryContent: (_id: number, path: string, options: { enabled?: boolean; mode?: string }) => ({
      data: options.enabled ? listings[path]?.[options.mode === "files" ? "files" : "dirs"] ?? [] : undefined,
      isLoading: false,
      isFetched: Boolean(options.enabled),
    }),
  }
})

afterEach(() => {
  cleanup()
  state.supported = true
  state.savePath = "/data"
})

function Field({ initial = "", includeFiles = false, onKeyDown }: { initial?: string; includeFiles?: boolean; onKeyDown?: (event: React.KeyboardEvent<HTMLInputElement>) => void }) {
  const [value, setValue] = useState(initial)
  return <PathInput aria-label="path" instanceId={1} value={value} onValueChange={setValue} includeFiles={includeFiles} onKeyDown={onKeyDown} />
}

const input = () => screen.getByLabelText("path") as HTMLInputElement
const names = () => screen.queryAllByRole("option").map((option) => option.textContent)

describe("PathInput", () => {
  it("lists the default save path when an empty field is clicked", () => {
    render(<Field />)
    fireEvent.focus(input())
    expect(names()).toEqual(["..", "movies", "music", "tv"])
  })

  it("opens a folder on click and keeps the list open on its content", () => {
    render(<Field />)
    fireEvent.focus(input())
    fireEvent.click(screen.getByRole("option", { name: "movies" }))
    expect(input().value).toBe("/data/movies/")
    expect(names()).toEqual(["..", "2024"])
  })

  it("goes up one level with ..", () => {
    render(<Field initial="/data/movies/" />)
    fireEvent.focus(input())
    fireEvent.click(screen.getByRole("option", { name: ".." }))
    expect(input().value).toBe("/data/")
    expect(names()).toEqual(["..", "movies", "music", "tv"])
  })

  it("shows no .. at the root", () => {
    state.savePath = ""
    render(<Field />)
    fireEvent.focus(input())
    expect(names()).toEqual(["data", "srv"])
  })

  it("filters by the typed name and picks the first match with Enter", () => {
    render(<Field />)
    fireEvent.focus(input())
    fireEvent.change(input(), { target: { value: "/data/mu" } })
    expect(names()).toEqual(["music"])
    fireEvent.keyDown(input(), { key: "Enter" })
    expect(input().value).toBe("/data/music/")
  })

  it("does not take Enter from the form for a value that it did not type", () => {
    const onKeyDown = vi.fn((event: React.KeyboardEvent<HTMLInputElement>) => event.defaultPrevented)
    render(<Field initial="/data/movies" onKeyDown={onKeyDown} />)
    fireEvent.focus(input())
    fireEvent.keyDown(input(), { key: "Enter" })
    expect(onKeyDown).toHaveReturnedWith(false)
    expect(input().value).toBe("/data/movies")
  })

  it("moves through the list with the arrow keys", () => {
    render(<Field />)
    fireEvent.focus(input())
    fireEvent.keyDown(input(), { key: "ArrowDown" })
    fireEvent.keyDown(input(), { key: "ArrowDown" })
    fireEvent.keyDown(input(), { key: "Enter" })
    expect(input().value).toBe("/data/movies/")
  })

  it("closes the list when a file is picked", () => {
    render(<Field includeFiles />)
    fireEvent.focus(input())
    fireEvent.click(screen.getByRole("option", { name: "notes.txt" }))
    expect(input().value).toBe("/data/notes.txt")
    expect(names()).toEqual([])
  })

  it("closes the list when the field loses focus", () => {
    render(<Field />)
    fireEvent.focus(input())
    fireEvent.blur(input())
    expect(names()).toEqual([])
  })

  it("browses a Windows path with backslashes", () => {
    render(<Field initial={"C:\\data\\"} />)
    fireEvent.focus(input())
    expect(names()).toEqual(["..", "movies"])
    fireEvent.click(screen.getByRole("option", { name: "movies" }))
    expect(input().value).toBe("C:\\data\\movies\\")
  })

  it("is a plain text field when qBittorrent cannot list folders", () => {
    state.supported = false
    render(<Field />)
    fireEvent.focus(input())
    expect(names()).toEqual([])
    expect(input().getAttribute("role")).toBeNull()
  })
})
