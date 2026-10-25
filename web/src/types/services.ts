/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

export type SeedboxContactState = "updated" | "unchanged" | "waiting" | "throttled" | "rejected" | "unreachable"
export type SeedboxContactSource = "schedule" | "manual" | "cookie"

export interface SeedboxContact {
  id: number
  source: SeedboxContactSource
  state: SeedboxContactState
  httpStatus?: number
  message?: string
  ip?: string
  asn?: number
  asOrg?: string
  createdAt: string
}

export interface SeedboxStatus {
  mamId: {
    configured: boolean
    masked?: string
  }
  enabled: boolean
  intervalSeconds: number
  nextContactAt?: string
  latest?: SeedboxContact
  contacts: SeedboxContact[]
}

export type GluetunAuthType = "none" | "apikey" | "basic"

export interface GluetunSettings {
  controlUrl: string
  authType: GluetunAuthType
  username: string
  hasSecret: boolean
  dockerHost: string
  containerName: string
}

export interface GluetunSettingsInput {
  controlUrl: string
  authType: GluetunAuthType
  username: string
  /** Omit to keep the stored secret; an empty string clears it. */
  secret?: string
  dockerHost: string
  containerName: string
}

export interface GluetunPublicIP {
  ip: string
  country?: string
  region?: string
  city?: string
  location?: string
  organization?: string
  postalCode?: string
  timezone?: string
  hostname?: string
}

export interface GluetunHealthCheck {
  start: string
  exitCode: number
  output: string
}

export interface GluetunContainer {
  id: string
  name: string
  image: string
  state: string
  running: boolean
  exitCode: number
  error?: string
  createdAt?: string
  startedAt?: string
  finishedAt?: string
  restartCount: number
  restartPolicy?: string
  memoryLimit: number
  cpuLimit: number
  capAdd: string[]
  health?: {
    status: string
    failingStreak: number
    checks: GluetunHealthCheck[]
  }
  ports: { container: string; host: string }[]
  environment: { name: string; value: string; redacted?: boolean }[]
}

export interface GluetunDependent {
  id: string
  name: string
  image: string
  state: string
  status: string
}

export type GluetunRestartStep = "gluetun" | "waiting" | "dependents" | "done"

export interface GluetunRestartState {
  running: boolean
  step?: GluetunRestartStep
  containers?: string[]
  startedAt?: string
  finishedAt?: string
  error?: string
}

export interface GluetunStatus {
  control: {
    reachable: boolean
    error?: string
    vpnStatus?: string
    dnsStatus?: string
    publicIp?: GluetunPublicIP
    forwardedPorts: number[]
  }
  docker: {
    available: boolean
    error?: string
    container?: GluetunContainer
    stats?: {
      cpuPercent: number
      memoryUsage: number
      memoryLimit: number
      rxBytes: number
      txBytes: number
      pids: number
    }
    dependents: GluetunDependent[]
  }
  restart: GluetunRestartState
}

export type GluetunLogLevel = "debug" | "info" | "warn" | "error" | "other"

export interface GluetunLogLine {
  time?: string
  level: GluetunLogLevel
  module?: string
  message: string
}

export type AppType = "sonarr" | "radarr" | "prowlarr" | "bazarr" | "tautulli"

export const APP_TYPES: AppType[] = ["sonarr", "radarr", "prowlarr", "bazarr", "tautulli"]

export interface AppInstance {
  id: number
  type: AppType
  name: string
  baseUrl: string
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface AppInstanceInput {
  type: AppType
  name: string
  baseUrl: string
  /** Empty on update keeps the stored key. */
  apiKey: string
  enabled: boolean
}

export interface AppHealthItem {
  level: string
  source?: string
  message: string
}

export interface AppQueueItem {
  title: string
  status?: string
  progress: number
  timeLeft?: string
  client?: string
  warning?: boolean
  messages?: string[]
}

export interface AppSession {
  user: string
  title: string
  state: string
  progress: number
  player?: string
  decision?: string
  quality?: string
}

export interface AppStatus {
  instance: AppInstance
  online: boolean
  error?: string
  version?: string
  checkedAt: string
  health: AppHealthItem[]
  queue?: { total: number; items: AppQueueItem[] }
  queueIssues?: AppQueueItem[]
  missing?: number
  disks?: { path: string; free: number; total: number }[]
  upcoming?: { title: string; subtitle?: string; date: string; hasFile: boolean }[]
  indexers?: { total: number; enabled: number; failing: string[]; queries: number; grabs: number }
  subtitles?: { wantedEpisodes: number; wantedMovies: number }
  streams?: { count: number; bandwidth: number; sessions: AppSession[] }
  warnings?: string[]
}

export interface AppInstanceRef {
  id: number
  name: string
}

export interface AppLibrary<T> {
  items: T[]
  instances: AppInstanceRef[]
  errors: (AppInstanceRef & { error: string })[]
  loadedAt: string
}

export interface AppMovie {
  instance: AppInstanceRef
  id: number
  title: string
  year?: number
  status?: string
  monitored: boolean
  hasFile: boolean
  isAvailable: boolean
  quality?: string
  sizeOnDisk: number
  added?: string
  release?: string
  imdbId?: string
  link: string
}

export interface AppSeries {
  instance: AppInstanceRef
  id: number
  title: string
  year?: number
  status?: string
  monitored: boolean
  network?: string
  seasons: number
  episodeFiles: number
  episodes: number
  totalEpisodes: number
  missing: number
  sizeOnDisk: number
  nextAiring?: string
  link: string
}

export interface AppWantedSubtitle {
  instance: AppInstanceRef
  kind: "episode" | "movie"
  title: string
  episode?: string
  languages: string[]
  link: string
}
