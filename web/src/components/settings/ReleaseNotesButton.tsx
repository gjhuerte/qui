/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { ExternalLink } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"

// qui does not update itself, so a new release only links its notes and downloads.
export function ReleaseNotesButton({ release, className }: { release: { html_url: string }; className?: string }) {
  const { t } = useTranslation("settings")

  return (
    <Button variant="outline" size="sm" className={className} asChild>
      <a href={release.html_url} target="_blank" rel="noopener noreferrer">
        <ExternalLink className="mr-2 h-4 w-4" />
        {t("application.update.releaseNotes")}
      </a>
    </Button>
  )
}
