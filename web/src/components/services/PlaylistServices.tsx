/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { ToneBadge } from "@/components/services/shared"
import type { ServicesTab } from "@/components/services/tabs"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { useMediaServers } from "@/hooks/useMediaSync"
import {
  useConnectSpotify,
  useConnectYTMusic,
  useDisconnectPlaylistService,
  usePlaylistServices,
  useSaveSpotifyApp,
  useStartSpotifySignIn
} from "@/hooks/usePlaylists"
import { getApiBaseUrl } from "@/lib/base-url"
import { copyTextToClipboard } from "@/lib/utils"
import type { PlaylistServicesStatus } from "@/types"
import { AlertTriangle, Copy, ExternalLink, Loader2, LogIn, Unplug } from "lucide-react"
import { useState } from "react"
import { Trans, useTranslation } from "react-i18next"
import { toast } from "sonner"

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

const linkClass = "font-medium text-primary underline-offset-4 hover:underline"

export function PlaylistServices({ onNavigate }: { onNavigate: (tab: ServicesTab) => void }) {
  const { data: status, isLoading } = usePlaylistServices()
  if (isLoading || !status) {
    return <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
  }
  return (
    <div className="space-y-6">
      <MediaServersCard onNavigate={onNavigate} />
      <SpotifyCard key={`${status.spotify.clientId}-${status.spotify.connected}`} status={status} />
      <YTMusicCard status={status} />
    </div>
  )
}

function StatusBadge({ connected, name }: { connected: boolean; name?: string }) {
  const { t } = useTranslation("services")
  return (
    <ToneBadge tone={connected ? "good" : "neutral"}>
      {connected ? t("playlists.setup.connectedAs", { name: name || "—" }) : t("playlists.setup.notConnected")}
    </ToneBadge>
  )
}

function MediaServersCard({ onNavigate }: { onNavigate: (tab: ServicesTab) => void }) {
  const { t } = useTranslation("services")
  const { data: servers } = useMediaServers()
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("playlists.setup.serversTitle")}</CardTitle>
        <CardDescription>{t("playlists.setup.serversDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {!servers || servers.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("mediaSync.servers.none")}</p>
        ) : (
          <div className="flex flex-wrap gap-2">
            {servers.map(s => (
              <ToneBadge key={s.id} tone={s.enabled ? "good" : "neutral"}>{s.name} · {t(`mediaSync.types.${s.type}`)}</ToneBadge>
            ))}
          </div>
        )}
        <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
          <li>{t("playlists.setup.serversAccount")}</li>
          <li>{t("playlists.setup.serversMatch")}</li>
        </ul>
      </CardContent>
      <CardFooter>
        <Button variant="outline" size="sm" onClick={() => onNavigate("media-servers-manage")}>{t("playlists.setup.manageServers")}</Button>
      </CardFooter>
    </Card>
  )
}

function SpotifyCard({ status }: { status: PlaylistServicesStatus }) {
  const { t } = useTranslation("services")
  const saveApp = useSaveSpotifyApp()
  const start = useStartSpotifySignIn()
  const connect = useConnectSpotify()
  const disconnect = useDisconnectPlaylistService()
  const [clientId, setClientId] = useState(status.spotify.clientId ?? "")
  const [clientSecret, setClientSecret] = useState("")
  const [redirectUri, setRedirectUri] = useState(status.spotify.redirectUri)
  const [redirected, setRedirected] = useState("")
  const [started, setStarted] = useState(false)
  const appSaved = !!status.spotify.clientId && status.spotify.hasSecret
  const quiCallback = `${window.location.origin}${getApiBaseUrl()}/playlists/services/spotify/callback`
  // With qui's own address, the callback route finishes the sign-in.
  const usesCallback = status.spotify.redirectUri !== status.spotify.defaultRedirectUri
  // Set by the callback route when the sign-in failed.
  const callbackError = new URLSearchParams(window.location.search).get("spotifyError")

  const copyRedirect = async () => {
    try {
      await copyTextToClipboard(status.spotify.redirectUri)
      toast.success(t("playlists.setup.copied"))
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  const signIn = () => start.mutate(undefined, {
    onSuccess: ({ url }) => {
      window.open(url, "_blank", "noopener")
      setStarted(true)
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle>{t("playlists.services.spotify")}</CardTitle>
          <StatusBadge connected={status.spotify.connected} name={status.spotify.accountName} />
        </div>
        <CardDescription>{t("playlists.setup.spotifyDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        <ol className="list-decimal space-y-2 pl-5 text-sm text-muted-foreground">
          <li>
            <Trans
              t={t}
              i18nKey="playlists.setup.spotifySteps.dashboard"
              components={{ dashboard: <a href="https://developer.spotify.com/dashboard" target="_blank" rel="noreferrer" className={linkClass} /> }}
            />
          </li>
          <li>
            {t("playlists.setup.spotifySteps.redirect")}
            <div className="mt-1 flex items-center gap-2">
              <code className="rounded bg-muted px-2 py-1 font-mono text-xs text-foreground">{status.spotify.redirectUri}</code>
              <Button variant="ghost" size="icon" className="h-7 w-7" onClick={copyRedirect} aria-label={t("playlists.setup.copy")}>
                <Copy className="h-3.5 w-3.5" />
              </Button>
            </div>
          </li>
          <li>{t("playlists.setup.spotifySteps.api")}</li>
          <li>{t("playlists.setup.spotifySteps.credentials")}</li>
          <li>{t("playlists.setup.spotifySteps.signIn")}</li>
          <li>{t("playlists.setup.spotifySteps.finish")}</li>
        </ol>
        {callbackError && (
          <Alert variant="destructive">
            <AlertTriangle className="h-4 w-4" />
            <AlertDescription className="break-words">{t("playlists.setup.spotifyFailed", { error: callbackError })}</AlertDescription>
          </Alert>
        )}
        <Alert>
          <AlertTriangle className="h-4 w-4" />
          <AlertDescription>{t("playlists.setup.spotifyPremium")}</AlertDescription>
        </Alert>

        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="spotify-client-id">{t("playlists.setup.clientId")}</Label>
            <Input id="spotify-client-id" value={clientId} onChange={(e) => setClientId(e.target.value)} autoComplete="off" spellCheck={false} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="spotify-client-secret">{t("playlists.setup.clientSecret")}</Label>
            <Input
              id="spotify-client-secret"
              type="password"
              value={clientSecret}
              onChange={(e) => setClientSecret(e.target.value)}
              placeholder={status.spotify.hasSecret ? t("mediaSync.form.tokenSaved") : ""}
              autoComplete="off"
              data-1p-ignore="true"
            />
          </div>
        </div>
        <div className="space-y-2">
          <Label htmlFor="spotify-redirect-uri">{t("playlists.setup.redirectUri")}</Label>
          <Input
            id="spotify-redirect-uri"
            value={redirectUri}
            onChange={(e) => setRedirectUri(e.target.value)}
            className="font-mono text-xs"
            autoComplete="off"
            spellCheck={false}
          />
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" onClick={() => setRedirectUri(quiCallback)}>{t("playlists.setup.useQuiAddress")}</Button>
            <Button variant="outline" size="sm" onClick={() => setRedirectUri(status.spotify.defaultRedirectUri)}>{t("playlists.setup.useDefault")}</Button>
          </div>
          <p className="text-xs text-muted-foreground">{t("playlists.setup.redirectHelp")}</p>
          {window.location.protocol !== "https:" && redirectUri === quiCallback && (
            <p className="text-xs text-destructive">{t("playlists.setup.redirectNeedsHttps")}</p>
          )}
        </div>
        <Button
          variant="outline"
          onClick={() => saveApp.mutate({ clientId, clientSecret, redirectUri }, {
            onSuccess: () => {
              setClientSecret("")
              toast.success(t("playlists.toasts.spotifyAppSaved"))
            },
            onError: (err) => toast.error(errorMessage(err)),
          })}
          disabled={saveApp.isPending || clientId.trim() === ""}
        >
          {saveApp.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {t("playlists.setup.saveApp")}
        </Button>

        {appSaved && (
          <div className="space-y-3 border-t pt-4">
            <Button onClick={signIn} disabled={start.isPending}>
              {start.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <LogIn className="mr-2 h-4 w-4" />}
              {status.spotify.connected ? t("playlists.setup.signInAgain") : t("playlists.setup.signIn")}
              <ExternalLink className="ml-2 h-3.5 w-3.5" />
            </Button>
            {!usesCallback && (started || !status.spotify.connected) && (
              <div className="space-y-2">
                <Label htmlFor="spotify-redirected">{t("playlists.setup.pasteAddress")}</Label>
                <div className="flex flex-col gap-2 sm:flex-row">
                  <Input
                    id="spotify-redirected"
                    value={redirected}
                    onChange={(e) => setRedirected(e.target.value)}
                    placeholder={t("playlists.setup.addressPlaceholder", { uri: status.spotify.redirectUri })}
                    className="font-mono text-xs"
                    spellCheck={false}
                  />
                  <Button
                    onClick={() => connect.mutate(redirected, {
                      onSuccess: (s) => {
                        setRedirected("")
                        toast.success(t("playlists.toasts.connected", { name: s.spotify.accountName ?? "" }))
                      },
                      onError: (err) => toast.error(errorMessage(err)),
                    })}
                    disabled={connect.isPending || redirected.trim() === ""}
                  >
                    {connect.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                    {t("playlists.setup.connect")}
                  </Button>
                </div>
              </div>
            )}
          </div>
        )}
      </CardContent>
      {(status.spotify.connected || appSaved) && (
        <CardFooter>
          <DisconnectButton onClick={() => disconnect.mutate("spotify", { onError: (err) => toast.error(errorMessage(err)) })} pending={disconnect.isPending} />
        </CardFooter>
      )}
    </Card>
  )
}

function DisconnectButton({ onClick, pending }: { onClick: () => void; pending: boolean }) {
  const { t } = useTranslation("services")
  return (
    <Button variant="outline" size="sm" onClick={onClick} disabled={pending}>
      <Unplug className="mr-2 h-4 w-4" />
      {t("playlists.setup.disconnect")}
    </Button>
  )
}

function YTMusicCard({ status }: { status: PlaylistServicesStatus }) {
  const { t } = useTranslation("services")
  const connect = useConnectYTMusic()
  const disconnect = useDisconnectPlaylistService()
  const [headers, setHeaders] = useState("")

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle>{t("playlists.services.ytmusic")}</CardTitle>
          <StatusBadge connected={status.ytmusic.connected} name={status.ytmusic.accountName} />
        </div>
        <CardDescription>{t("playlists.setup.ytDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        <Alert>
          <AlertTriangle className="h-4 w-4" />
          <AlertDescription>{t("playlists.setup.ytWarning")}</AlertDescription>
        </Alert>
        <ol className="list-decimal space-y-2 pl-5 text-sm text-muted-foreground">
          <li>
            <Trans
              t={t}
              i18nKey="playlists.setup.ytSteps.open"
              components={{ ytm: <a href="https://music.youtube.com" target="_blank" rel="noreferrer" className={linkClass} /> }}
            />
          </li>
          <li>{t("playlists.setup.ytSteps.devtools")}</li>
          <li>{t("playlists.setup.ytSteps.request")}</li>
          <li>{t("playlists.setup.ytSteps.copy")}</li>
          <li>{t("playlists.setup.ytSteps.paste")}</li>
        </ol>
        <div className="space-y-2">
          <Label htmlFor="ytmusic-headers">{t("playlists.setup.ytHeaders")}</Label>
          <Textarea
            id="ytmusic-headers"
            rows={6}
            value={headers}
            onChange={(e) => setHeaders(e.target.value)}
            placeholder={t("playlists.setup.ytPlaceholder")}
            className="font-mono text-xs"
            spellCheck={false}
            autoComplete="off"
          />
        </div>
      </CardContent>
      <CardFooter className="flex justify-between gap-2">
        {status.ytmusic.connected ? (
          <DisconnectButton onClick={() => disconnect.mutate("ytmusic", { onError: (err) => toast.error(errorMessage(err)) })} pending={disconnect.isPending} />
        ) : <span />}
        <Button
          onClick={() => connect.mutate(headers, {
            onSuccess: (s) => {
              setHeaders("")
              toast.success(t("playlists.toasts.connected", { name: s.ytmusic.accountName ?? "" }))
            },
            onError: (err) => toast.error(errorMessage(err)),
          })}
          disabled={connect.isPending || headers.trim() === ""}
        >
          {connect.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {t("playlists.setup.connect")}
        </Button>
      </CardFooter>
    </Card>
  )
}
