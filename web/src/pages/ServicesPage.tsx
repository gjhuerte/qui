/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { PageHeader } from "@/components/layout/PageHeader"
import { SectionedPage, type SectionNavGroup } from "@/components/layout/SectionedPage"
import { AppsDashboard } from "@/components/services/AppsDashboard"
import { AppsIssues } from "@/components/services/AppsIssues"
import { AppsManage } from "@/components/services/AppsManage"
import { GluetunConfig } from "@/components/services/GluetunConfig"
import { GluetunConnection } from "@/components/services/GluetunConnection"
import { GluetunLogs } from "@/components/services/GluetunLogs"
import { GluetunOverview } from "@/components/services/GluetunOverview"
import { MoviesView, SeriesView, SubtitlesView } from "@/components/services/LibraryViews"
import { MediaServersDashboard } from "@/components/services/MediaServersDashboard"
import { MediaServersManage } from "@/components/services/MediaServersManage"
import { MediaSyncAccounts } from "@/components/services/MediaSyncAccounts"
import { MediaSyncActivity } from "@/components/services/MediaSyncActivity"
import { MediaSyncPairs } from "@/components/services/MediaSyncPairs"
import { PlaylistServices } from "@/components/services/PlaylistServices"
import { PlaylistsPage } from "@/components/services/PlaylistsPage"
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
        {activeTab === "apps" && <AppsDashboard onNavigate={onTabChange} />}
        {activeTab === "apps-issues" && <AppsIssues onNavigate={onTabChange} />}
        {activeTab === "apps-movies" && <MoviesView onNavigate={onTabChange} />}
        {activeTab === "apps-series" && <SeriesView onNavigate={onTabChange} />}
        {activeTab === "apps-subtitles" && <SubtitlesView onNavigate={onTabChange} />}
        {activeTab === "apps-manage" && <AppsManage />}
        {activeTab === "media-servers" && <MediaServersDashboard onNavigate={onTabChange} />}
        {activeTab === "media-servers-manage" && <MediaServersManage />}
        {activeTab === "media-sync" && <MediaSyncPairs onNavigate={onTabChange} />}
        {activeTab === "media-sync-accounts" && <MediaSyncAccounts onNavigate={onTabChange} />}
        {activeTab === "media-sync-activity" && <MediaSyncActivity onNavigate={onTabChange} />}
        {activeTab === "playlists" && <PlaylistsPage onNavigate={onTabChange} />}
        {activeTab === "playlist-services" && <PlaylistServices onNavigate={onTabChange} />}
      </SectionedPage>
    </div>
  )
}
