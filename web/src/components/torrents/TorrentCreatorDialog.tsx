/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useEffect, useState } from "react"

import { useForm } from "@tanstack/react-form"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { AlertCircle, ChevronDown, Info, Loader2 } from "lucide-react"
import { toast } from "sonner"
import { z } from "zod"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger
} from "@/components/ui/collapsible"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { PathInput } from "@/components/ui/path-input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { useInstanceTrackers } from "@/hooks/useInstanceTrackers"
import { useQBittorrentAppInfo } from "@/hooks/useQBittorrentAppInfo"
import { api } from "@/lib/api"
import type { TorrentCreationParams, TorrentFormat } from "@/types"

import { useTranslation } from "react-i18next"

import { getPieceSizeOptions, TorrentPieceSize } from "./piece-size"

/** Parse newline-separated input into array of non-empty trimmed strings */
function parseLines(input: string): string[] {
  return input.split("\n").map((line) => line.trim()).filter(Boolean)
}

const torrentFilePathSchema = z.string().trim().refine(
  (value) => value === "" || (!value.endsWith("/") && !value.endsWith("\\"))
)

const FORM_ID = "torrent-creator-form"

interface TorrentCreatorDialogProps {
  instanceId: number
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function TorrentCreatorDialog({ instanceId, open, onOpenChange }: TorrentCreatorDialogProps) {
  const { t } = useTranslation("torrents")
  const [error, setError] = useState<string | null>(null)
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const queryClient = useQueryClient()

  const { versionInfo } = useQBittorrentAppInfo(instanceId)
  const supportsFormatSelection = versionInfo.isLibtorrent2 !== false
  const libtorrentVersionLabel = versionInfo.libtorrentMajorVersion? `libtorrent ${versionInfo.libtorrentMajorVersion}.x`: "libtorrent 1.x"

  // Fetch active trackers for the select dropdown
  const { data: activeTrackers } = useInstanceTrackers(instanceId, { enabled: open })


  const mutation = useMutation({
    mutationFn: async (data: TorrentCreationParams) => {
      return api.createTorrent(instanceId, data)
    },
    onSuccess: () => {
      setError(null)
      onOpenChange(false)
      form.reset()
      // Invalidate tasks and badge count so polling views update immediately
      queryClient.invalidateQueries({ queryKey: ["torrent-creation-tasks", instanceId] })
      queryClient.invalidateQueries({ queryKey: ["active-task-count", instanceId] })
      toast.success(t("creatorDialog.toast.taskQueued"))
    },
    onError: (err: Error) => {
      setError(err.message)
      toast.error(err.message || t("creatorDialog.toast.taskFailed"))
    },
  })

  const form = useForm({
    defaultValues: {
      sourcePath: "",
      private: true,
      trackers: "",
      comment: "",
      source: "",
      startSeeding: true,
      // Advanced options
      format: "v1" as TorrentFormat,
      pieceSize: "",
      torrentFilePath: "",
      urlSeeds: "",
    },
    onSubmit: async ({ value }) => {
      setError(null)

      const trackers = parseLines(value.trackers)
      const urlSeeds = parseLines(value.urlSeeds)

      const params: TorrentCreationParams = {
        sourcePath: value.sourcePath,
        private: value.private,
        trackers: trackers.length > 0 ? trackers : undefined,
        comment: value.comment || undefined,
        source: value.source || undefined,
        startSeeding: value.startSeeding,
        format: supportsFormatSelection ? value.format : "v1",
        pieceSize: value.pieceSize ? parseInt(value.pieceSize) : undefined,
        torrentFilePath: value.torrentFilePath || undefined,
        urlSeeds: urlSeeds.length > 0 ? urlSeeds : undefined,
      }

      mutation.mutate(params)
    },
  })

  useEffect(() => {
    if (!supportsFormatSelection) {
      form.setFieldValue("format", "v1")
    }
  }, [supportsFormatSelection, form])

  // Reset form and error state when dialog closes
  useEffect(() => {
    if (!open) {
      form.reset()
      setError(null)
      setAdvancedOpen(false)
    }
  }, [open, form])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl max-h-[90dvh] flex flex-col">
        <DialogHeader className="flex-shrink-0">
          <DialogTitle>{t("creatorDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("creatorDialog.description")}
          </DialogDescription>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto min-h-0">
          <form
            id={FORM_ID}
            onSubmit={(e) => {
              e.preventDefault()
              e.stopPropagation()
              form.handleSubmit()
            }}
            className="space-y-4"
          >
            {error && (
              <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            {/* Source Path */}
            <form.Field name="sourcePath">
              {(field) => (
                <div className="space-y-2">
                  <Label htmlFor="sourcePath">
                    {t("creatorDialog.sourcePath")} <span className="text-destructive">*</span>
                  </Label>
                  <PathInput
                    id="sourcePath"
                    instanceId={instanceId}
                    includeFiles
                    placeholder={t("creatorDialog.sourcePathPlaceholder")}
                    value={field.state.value}
                    onBlur={field.handleBlur}
                    onValueChange={field.handleChange}
                    required
                  />

                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <span>{t("creatorDialog.sourcePathHelp")}</span>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Info className="h-4 w-4 cursor-help shrink-0" />
                      </TooltipTrigger>
                      <TooltipContent>
                        <p>{t("creatorDialog.sourcePathTooltip")}</p>
                      </TooltipContent>
                    </Tooltip>
                  </div>
                </div>
              )}
            </form.Field>

            {/* Private */}
            <form.Field name="private">
              {(field) => (
                <div className="flex items-center justify-between">
                  <div className="space-y-2">
                    <Label htmlFor="private">{t("creatorDialog.privateTorrent")}</Label>
                    <p className="text-sm text-muted-foreground">
                      {t("creatorDialog.privateDescription")}
                    </p>
                  </div>
                  <Switch
                    id="private"
                    checked={field.state.value}
                    onCheckedChange={field.handleChange}
                  />
                </div>
              )}
            </form.Field>

            {/* Trackers */}
            <form.Field name="trackers">
              {(field) => (
                <div className="space-y-2">
                  <Label htmlFor="trackers">{t("creatorDialog.trackers")}</Label>
                  {activeTrackers && Object.keys(activeTrackers).length > 0 && (
                    <div className="space-y-2">
                      <p className="text-sm text-muted-foreground">
                        {t("creatorDialog.trackersSelectHelp")}
                      </p>
                      <Select
                        value=""
                        onValueChange={(trackerUrl) => {
                          const currentTrackers = field.state.value
                          const newTrackers = currentTrackers? `${currentTrackers}\n${trackerUrl}`: trackerUrl
                          field.handleChange(newTrackers)
                        }}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder={t("creatorDialog.trackersSelectPlaceholder")} />
                        </SelectTrigger>
                        <SelectContent>
                          {Object.entries(activeTrackers)
                            .sort(([domainA], [domainB]) => domainA.localeCompare(domainB))
                            .map(([domain, url]) => (
                              <SelectItem key={domain} value={url}>
                                {domain}
                              </SelectItem>
                            ))}
                        </SelectContent>
                      </Select>
                    </div>
                  )}
                  <p className="text-sm text-muted-foreground">
                    {t("creatorDialog.trackersHelp")}
                  </p>
                  <Textarea
                    id="trackers"
                    placeholder={t("creatorDialog.trackersPlaceholder")}
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.target.value)}
                    rows={4}
                  />
                </div>
              )}
            </form.Field>

            {/* Comment */}
            <form.Field name="comment">
              {(field) => (
                <div className="space-y-2">
                  <Label htmlFor="comment">{t("creatorDialog.comment")}</Label>
                  <Input
                    id="comment"
                    placeholder={t("creatorDialog.commentPlaceholder")}
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                </div>
              )}
            </form.Field>

            {/* Source */}
            <form.Field name="source">
              {(field) => (
                <div className="space-y-2">
                  <Label htmlFor="source">{t("creatorDialog.source")}</Label>
                  <Input
                    id="source"
                    placeholder={t("creatorDialog.sourcePlaceholder")}
                    value={field.state.value}
                    onChange={(e) => field.handleChange(e.target.value)}
                  />
                </div>
              )}
            </form.Field>

            {/* Start Seeding */}
            <form.Field name="startSeeding">
              {(field) => (
                <div className="flex items-center justify-between">
                  <div className="space-y-0.5">
                    <Label htmlFor="startSeeding">{t("creatorDialog.addToQBittorrent")}</Label>
                    <p className="text-sm text-muted-foreground">
                      {t("creatorDialog.addToQBittorrentDescription")}
                    </p>
                  </div>
                  <Switch
                    id="startSeeding"
                    checked={field.state.value}
                    onCheckedChange={field.handleChange}
                  />
                </div>
              )}
            </form.Field>

            {/* Advanced Options */}
            <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
              <CollapsibleTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  className="w-full justify-between p-0 hover:bg-transparent"
                >
                  <span className="text-sm font-medium">{t("creatorDialog.advancedOptions")}</span>
                  <ChevronDown
                    className={`h-4 w-4 transition-transform ${advancedOpen ? "rotate-180" : ""}`}
                  />
                </Button>
              </CollapsibleTrigger>
              <CollapsibleContent className="space-y-4 pt-4">
                {/* Torrent Format */}
                {supportsFormatSelection ? (
                  <form.Field name="format">
                    {(field) => (
                      <div className="space-y-2">
                        <Label htmlFor="format">{t("creatorDialog.torrentFormat")}</Label>
                        <Select
                          value={field.state.value}
                          onValueChange={(value) => field.handleChange(value as TorrentFormat)}
                        >
                          <SelectTrigger id="format">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="v1">{t("creatorDialog.formatV1")}</SelectItem>
                            <SelectItem value="v2">{t("creatorDialog.formatV2")}</SelectItem>
                            <SelectItem value="hybrid">{t("creatorDialog.formatHybrid")}</SelectItem>
                          </SelectContent>
                        </Select>
                        <p className="text-sm text-muted-foreground">
                          {t("creatorDialog.formatHelp")}
                        </p>
                      </div>
                    )}
                  </form.Field>
                ) : (
                  <Alert className="bg-muted/40 text-muted-foreground">
                    <Info className="h-4 w-4" />
                    <AlertTitle>{t("creatorDialog.formatUnavailableTitle")}</AlertTitle>
                    <AlertDescription>
                      {t("creatorDialog.formatUnavailableDescription", { version: libtorrentVersionLabel })}
                    </AlertDescription>
                  </Alert>
                )}

                {/* Piece Size
                  https://github.com/qbittorrent/qBittorrent/blob/master/src/gui/torrentcreatordialog.cpp#L86-L92

                  m_ui->comboPieceSize->addItem(tr("Auto"), 0);
                  for (int i = 4; i <= 17; ++i)
                  {
                      const int size = 1024 << i;
                      const QString displaySize = Utils::Misc::friendlyUnit(size, false, 0);
                      m_ui->comboPieceSize->addItem(displaySize, size);
                  }
              */}
                <form.Field name="pieceSize">
                  {(field) => (
                    <div className="space-y-2">
                      <Label htmlFor="pieceSize">{t("creatorDialog.pieceSize")}</Label>
                      <Select
                        value={field.state.value || TorrentPieceSize.Auto}
                        onValueChange={field.handleChange}
                      >
                        <SelectTrigger id="pieceSize">
                          <SelectValue placeholder={t("creatorDialog.pieceSizePlaceholder")} />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value={TorrentPieceSize.Auto}>
                            {t("creatorDialog.pieceSizePlaceholder")}
                          </SelectItem>
                          {getPieceSizeOptions().map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <p className="text-sm text-muted-foreground">
                        {t("creatorDialog.pieceSizeHelp")}
                      </p>
                    </div>
                  )}
                </form.Field>

                {/* Torrent File Path */}
                <form.Field
                  name="torrentFilePath"
                  validators={{
                    onChange: ({ value }) => {
                      const result = torrentFilePathSchema.safeParse(value)
                      return result.success ? undefined : t("creatorDialog.saveTorrentToInvalid")
                    },
                  }}
                >
                  {(field) => (
                    <div className="space-y-2">
                      <Label htmlFor="torrentFilePath">{t("creatorDialog.saveTorrentTo")}</Label>
                      <PathInput
                        id="torrentFilePath"
                        instanceId={instanceId}
                        placeholder={t("creatorDialog.saveTorrentToPlaceholder")}
                        value={field.state.value}
                        aria-invalid={field.state.meta.isTouched && !!field.state.meta.errors[0]}
                        onBlur={field.handleBlur}
                        onValueChange={field.handleChange}
                      />

                      <div className="flex items-center gap-2 text-sm text-muted-foreground">
                        <span>{t("creatorDialog.saveTorrentToHelp")}</span>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Info className="h-4 w-4 cursor-help shrink-0" />
                          </TooltipTrigger>
                          <TooltipContent className="max-w-xs">
                            <p>{t("creatorDialog.saveTorrentToTooltip")}</p>
                          </TooltipContent>
                        </Tooltip>
                      </div>
                      {field.state.meta.isTouched && field.state.meta.errors[0] && (
                        <p className="text-sm text-destructive">{field.state.meta.errors[0]}</p>
                      )}
                    </div>
                  )}
                </form.Field>

                {/* URL Seeds */}
                <form.Field name="urlSeeds">
                  {(field) => (
                    <div className="space-y-2">
                      <Label htmlFor="urlSeeds">{t("creatorDialog.webSeeds")}</Label>
                      <Textarea
                        id="urlSeeds"
                        placeholder={t("creatorDialog.webSeedsPlaceholder")}
                        value={field.state.value}
                        onChange={(e) => field.handleChange(e.target.value)}
                        rows={3}
                      />
                      <p className="text-sm text-muted-foreground">
                        {t("creatorDialog.webSeedsHelp")}
                      </p>
                    </div>
                  )}
                </form.Field>
              </CollapsibleContent>
            </Collapsible>
          </form>
        </div>

        <DialogFooter className="flex-shrink-0">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={mutation.isPending}
          >
            {t("creatorDialog.cancel")}
          </Button>
          <Button type="submit" form={FORM_ID} disabled={mutation.isPending}>
            {mutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {t("creatorDialog.createTorrent")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
