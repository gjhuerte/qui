/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { PageHeader } from "@/components/layout/PageHeader"
import { SectionedPage, type SectionNavGroup } from "@/components/layout/SectionedPage"
import { GluetunConfig } from "@/components/services/GluetunConfig"
import { GluetunConnection } from "@/components/services/GluetunConnection"
import { GluetunLogs } from "@/components/services/GluetunLogs"
import { GluetunOverview } from "@/components/services/GluetunOverview"
import { SeedboxActivity } from "@/components/services/SeedboxActivity"
import { SeedboxOverview } from "@/components/services/SeedboxOverview"
import { ToneBadge } from "@/components/services/shared"
import { SERVICES_NAV_GROUPS, type ServicesTab } from "@/components/services/tabs"
import { useGluetunStatus } from "@/hooks/useServices"
import { vpnTone } from "@/lib/services-tone"
import { useTranslation } from "react-i18next"

interface ServicesPageProps {
  activeTab: ServicesTab
  onTabChange: (tab: ServicesTab) => void
}

export function ServicesPage({ activeTab, onTabChange }: ServicesPageProps) {
  const { t } = useTranslation("services")
  const { data: gluetun } = useGluetunStatus()
  const navGroups: SectionNavGroup<ServicesTab>[] = SERVICES_NAV_GROUPS.map(group => ({
    label: t(group.labelKey),
    sections: group.tabs.map(tab => ({ value: tab, label: t(`tabs.${tab}`) })),
  }))

  const vpn = gluetun?.control

  return (
    <div className="space-y-6 p-4 lg:p-6 pb-16">
      <PageHeader
        title={t("pageTitle")}
        description={t("pageDescription")}
        actions={vpn && (
          <ToneBadge tone={vpn.reachable ? vpnTone(vpn.vpnStatus) : "bad"}>
            {vpn.reachable
              ? t("vpnBadge", { status: t([`states.${vpn.vpnStatus}`, "states.unknown"]), ip: vpn.publicIp?.ip ?? "—" })
              : t("states.unreachable")}
          </ToneBadge>
        )}
      />

      <SectionedPage navLabel={t("pageTitle")} groups={navGroups} activeSection={activeTab} onSectionChange={onTabChange}>
        {activeTab === "gluetun" && <GluetunOverview onNavigate={onTabChange} />}
        {activeTab === "gluetun-logs" && <GluetunLogs />}
        {activeTab === "gluetun-config" && <GluetunConfig />}
        {activeTab === "gluetun-connection" && <GluetunConnection />}
        {activeTab === "seedbox" && <SeedboxOverview />}
        {activeTab === "seedbox-activity" && <SeedboxActivity />}
      </SectionedPage>
    </div>
  )
}
