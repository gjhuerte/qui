/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Loader2 } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog"
import type { RestartOverlay as RestartOverlayState } from "@/hooks/useRestart"

export function RestartOverlay({ state }: { state: RestartOverlayState }) {
  const { t } = useTranslation("settings")

  if (state === "hidden") {
    return null
  }

  const slow = state === "slow"
  const title = slow ? t("application.restart.overlay.slowTitle") : t("application.restart.overlay.restartingTitle")
  const detail = slow ? t("application.restart.overlay.slowDetail") : t("application.restart.overlay.restartingDetail")

  // A modal with no close path: it traps focus, so the keyboard cannot reach
  // a Settings tab whose switch would unmount the poll.
  return (
    <Dialog open>
      <DialogContent
        showCloseButton={false}
        className="inset-0 z-[100] flex max-w-none translate-x-0 translate-y-0 items-center justify-center rounded-none border-0 bg-background/95 shadow-none backdrop-blur-sm sm:max-w-none">
        <div role="status" aria-live="polite" className="flex max-w-md flex-col items-center gap-3 text-center">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          <DialogTitle className="font-medium">{title}</DialogTitle>
          <DialogDescription>{detail}</DialogDescription>
        </div>
      </DialogContent>
    </Dialog>
  )
}
