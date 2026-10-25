/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import type { Tone } from "@/lib/services-tone"
import type { ReactNode } from "react"

const toneClasses: Record<Tone, string> = {
  good: "border-transparent bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  warn: "border-transparent bg-amber-500/15 text-amber-700 dark:text-amber-400",
  bad: "border-transparent bg-destructive/15 text-destructive",
  neutral: "",
}

export function ToneBadge({ tone, children }: { tone: Tone; children: ReactNode }) {
  return (
    <Badge variant={tone === "neutral" ? "secondary" : "outline"} className={toneClasses[tone]}>
      {children}
    </Badge>
  )
}

/** A label/value pair in a details grid. */
export function InfoItem({ label, children, mono }: { label: ReactNode; children: ReactNode; mono?: boolean }) {
  return (
    <div className="min-w-0 space-y-0.5">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className={cn("text-sm font-medium break-words", mono && "font-mono")}>{children}</dd>
    </div>
  )
}

export function InfoGrid({ children }: { children: ReactNode }) {
  return <dl className="grid gap-x-6 gap-y-4 sm:grid-cols-2 lg:grid-cols-3">{children}</dl>
}
