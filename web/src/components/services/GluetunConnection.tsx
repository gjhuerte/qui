/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { FieldHelp } from "@/components/ui/field-help"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { SaveFooter } from "@/components/ui/save-footer"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { useGluetunSettings, useUpdateGluetunSettings } from "@/hooks/useServices"
import type { GluetunAuthType, GluetunSettings } from "@/types"
import { Loader2 } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

const AUTH_TYPES: GluetunAuthType[] = ["none", "apikey", "basic"]
const DEFAULT_DOCKER_HOST = "unix:///var/run/docker.sock"

export function GluetunConnection() {
  const { data } = useGluetunSettings()
  if (!data) {
    return <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
  }
  return <ConnectionForm settings={data} />
}

function ConnectionForm({ settings }: { settings: GluetunSettings }) {
  const { t } = useTranslation("services")
  const update = useUpdateGluetunSettings()
  const [controlUrl, setControlUrl] = useState(settings.controlUrl)
  const [authType, setAuthType] = useState<GluetunAuthType>(settings.authType)
  const [username, setUsername] = useState(settings.username)
  const [secret, setSecret] = useState("")
  const [dockerEnabled, setDockerEnabled] = useState(settings.dockerHost !== "")
  const [dockerHost, setDockerHost] = useState(settings.dockerHost || DEFAULT_DOCKER_HOST)
  const [containerName, setContainerName] = useState(settings.containerName)

  const save = () => {
    update.mutate({
      controlUrl,
      authType,
      username,
      // An untouched field keeps the stored secret.
      secret: secret === "" ? undefined : secret,
      dockerHost: dockerEnabled ? dockerHost : "",
      containerName,
    }, {
      onSuccess: () => {
        setSecret("")
        toast.success(t("connection.saved"))
      },
      onError: (err) => toast.error(err instanceof Error ? err.message : String(err)),
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("connection.control.title")}</CardTitle>
        <CardDescription>{t("connection.control.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <div className="flex items-center gap-1.5">
              <Label htmlFor="gluetun-control-url">{t("connection.control.url")}</Label>
              <FieldHelp>{t("connection.control.urlHelp")}</FieldHelp>
            </div>
            <Input id="gluetun-control-url" value={controlUrl} onChange={(e) => setControlUrl(e.target.value)} placeholder="http://127.0.0.1:8000" />
          </div>
          <div className="space-y-2">
            <div className="flex items-center gap-1.5">
              <Label htmlFor="gluetun-auth-type">{t("connection.control.auth")}</Label>
              <FieldHelp>{t("connection.control.authHelp")}</FieldHelp>
            </div>
            <Select value={authType} onValueChange={(value) => setAuthType(value as GluetunAuthType)}>
              <SelectTrigger id="gluetun-auth-type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {AUTH_TYPES.map(type => <SelectItem key={type} value={type}>{t(`connection.control.authTypes.${type}`)}</SelectItem>)}
              </SelectContent>
            </Select>
          </div>
          {authType === "basic" && (
            <div className="space-y-2">
              <Label htmlFor="gluetun-username">{t("connection.control.username")}</Label>
              <Input id="gluetun-username" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="off" />
            </div>
          )}
          {authType !== "none" && (
            <div className="space-y-2">
              <Label htmlFor="gluetun-secret">
                {authType === "apikey" ? t("connection.control.apiKey") : t("connection.control.password")}
              </Label>
              <Input
                id="gluetun-secret"
                type="password"
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                placeholder={settings.hasSecret ? t("connection.control.secretSaved") : ""}
                autoComplete="off"
                data-1p-ignore="true"
              />
            </div>
          )}
        </div>

        <div className="space-y-1 border-t pt-6">
          <h3 className="font-semibold">{t("connection.docker.title")}</h3>
          <p className="text-sm text-muted-foreground">{t("connection.docker.description")}</p>
        </div>
        <div className="flex items-center justify-between gap-3">
          <div className="space-y-0.5">
            <Label htmlFor="gluetun-docker-enabled" className="font-medium">{t("connection.docker.enable")}</Label>
            <p className="text-xs text-muted-foreground">{t("connection.docker.enableHelp")}</p>
          </div>
          <Switch id="gluetun-docker-enabled" checked={dockerEnabled} onCheckedChange={setDockerEnabled} />
        </div>
        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <div className="flex items-center gap-1.5">
              <Label htmlFor="gluetun-docker-host">{t("connection.docker.host")}</Label>
              <FieldHelp>{t("connection.docker.hostHelp")}</FieldHelp>
            </div>
            <Input id="gluetun-docker-host" value={dockerHost} disabled={!dockerEnabled} onChange={(e) => setDockerHost(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="gluetun-container">{t("connection.docker.container")}</Label>
            <Input id="gluetun-container" value={containerName} onChange={(e) => setContainerName(e.target.value)} />
          </div>
        </div>
        <div className="space-y-2">
          <p className="text-xs text-muted-foreground">{t("connection.docker.composeHelp")}</p>
          <pre className="overflow-auto rounded-md border bg-muted/30 p-3 font-mono text-xs">{COMPOSE_SNIPPET}</pre>
        </div>
      </CardContent>
      <SaveFooter pending={update.isPending} onSave={save} label={t("connection.save")} />
    </Card>
  )
}

// A code sample, not prose: it stays the same in every language.
const COMPOSE_SNIPPET = `  qui:
    image: ghcr.io/gjhuerte/qui:latest
    network_mode: "service:gluetun"
    user: "1000:1000"
    group_add:
      - "\${DOCKER_GID}"   # getent group docker | cut -d: -f3
    volumes:
      - ./qui-config:/config
      - /var/run/docker.sock:/var/run/docker.sock`
