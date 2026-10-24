/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { createMemoryHistory, createRootRoute, createRouter, RouterContextProvider } from "@tanstack/react-router"
import { cleanup, render, screen } from "@testing-library/react"
import { HttpResponse, http } from "msw"
import { useState } from "react"
import { afterEach, expect, it } from "vitest"

import "@/i18n"
import type { SettingsSearch } from "@/routes/_authenticated/settings"
import { server } from "@/test/msw/server"
import { Settings } from "./Settings"

let client: QueryClient

afterEach(() => {
  cleanup()
  client.clear()
})

it("links the release notes when a new release is available and application-info fails", async () => {
  server.use(
    http.get("*/api/application/info", () => HttpResponse.json({ error: "Application info unavailable" }, { status: 500 })),
    http.get("*/api/auth/me", () => HttpResponse.json({ username: "demo", auth_method: "builtin" })),
    http.get("*/api/version/latest", () => HttpResponse.json({ tag_name: "v1.31.1", html_url: "https://example.invalid/releases/v1.31.1" })),
    http.get("*/api/version", () => HttpResponse.json({ version: "1.30.0", restart: true }))
  )
  function Page() {
    const [search, setSearch] = useState<SettingsSearch>({ tab: "application" })
    return <Settings search={search} onSearchChange={setSearch} />
  }
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const router = createRouter({ routeTree: createRootRoute(), history: createMemoryHistory({ initialEntries: ["/settings"] }) })
  render(
    <RouterContextProvider router={router}>
      <QueryClientProvider client={client}><Page /></QueryClientProvider>
    </RouterContextProvider>
  )

  expect(await screen.findByText("Application info unavailable")).toBeTruthy()
  const link = await screen.findByRole("link", { name: "Release notes" })
  expect(link.getAttribute("href")).toBe("https://example.invalid/releases/v1.31.1")
})
