/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { DEFAULT_SERVICES_TAB, servicesSearchSchema, type ServicesTab } from "@/components/services/tabs"
import { ServicesPage } from "@/pages/ServicesPage"
import { createFileRoute } from "@tanstack/react-router"

export const Route = createFileRoute("/_authenticated/services")({
  validateSearch: servicesSearchSchema,
  component: ServicesRoute,
})

function ServicesRoute() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const handleTabChange = (tab: ServicesTab) => {
    navigate({
      search: { tab },
      replace: true,
    })
  }

  return <ServicesPage activeTab={search.tab ?? DEFAULT_SERVICES_TAB} onTabChange={handleTabChange} />
}
