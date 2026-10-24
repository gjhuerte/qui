/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { themes, type Theme } from "@/config/themes"
import { useBuiltinThemes } from "@/hooks/useBuiltinThemes"
import { useCustomThemes } from "@/hooks/useCustomThemes"
import { useTheme } from "@/hooks/useTheme"
import { getThemeColors, getThemeVariation } from "@/utils/theme"
import { buildThemeCatalog } from "@/lib/theme-catalog"
import { ThemeLayoutSettings } from "@/components/themes/ThemeLayoutSettings"
import { Check, Palette, AlertTriangle, FolderOpen, RefreshCw } from "lucide-react"
import { useTranslation } from "react-i18next"

interface ThemeCardProps {
  theme: Theme
  isSelected: boolean
  onSelect: () => void
  onVariationSelect: (themeId: string, variationId: string) => void
}

function ThemeCard({ theme, isSelected, onSelect, onVariationSelect }: ThemeCardProps) {
  const { t } = useTranslation("settings")
  // Get current variation for theme (validated)
  const variation = getThemeVariation(theme.id)

  // Helper to extract colors from theme
  const colors = getThemeColors(theme)

  return (
    <div role="listitem" className="h-full">
      <Card
        className={`h-full cursor-pointer gap-2 py-3 transition-all duration-200 hover:shadow-md ${
          isSelected ? "ring-2 ring-primary" : ""
        }`}
        onClick={onSelect}
      >
        <CardHeader className="gap-1 px-3">
          <div className="flex items-center justify-between">
            <CardTitle className="flex min-w-0 items-center gap-1 truncate text-sm">
              {theme.name}
              {isSelected && (
                <Check className="h-4 w-4 shrink-0 text-primary" />
              )}
            </CardTitle>
          </div>
          {theme.description && (
            <CardDescription className="line-clamp-1 text-xs">
              {theme.description}
            </CardDescription>
          )}
        </CardHeader>
        <CardContent className="space-y-2 px-3">
          {/* Theme preview colors and variations */}
          <div className="flex items-center justify-between gap-2 flex-wrap">
            {/* Preview colors */}
            <div className="flex gap-1">
              <div
                className="h-3 w-3 rounded-full ring-1 ring-black/10 dark:ring-white/10"
                style={{
                  backgroundColor: colors.primary,
                  backgroundImage: "none",
                  background: colors.primary + " !important",
                }}
              />
              <div
                className="h-3 w-3 rounded-full ring-1 ring-black/10 dark:ring-white/10"
                style={{
                  backgroundColor: colors.secondary,
                  backgroundImage: "none",
                  background: colors.secondary + " !important",
                }}
              />
              <div
                className="h-3 w-3 rounded-full ring-1 ring-black/10 dark:ring-white/10"
                style={{
                  backgroundColor: colors.accent,
                  backgroundImage: "none",
                  background: colors.accent + " !important",
                }}
              />
            </div>

            {/* Variation colors */}
            {colors.variations && colors.variations.length > 0 && (
              <div className="flex gap-1">
                {colors.variations.map((v) => {
                  const selected = variation === v.id
                  return (
                    <button
                      key={v.id}
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation()
                        onVariationSelect(theme.id, v.id)
                      }}
                      className={`h-3 w-3 rounded-full transition-all ${
                        selected ? "ring-2 ring-black dark:ring-white" : "ring-1 ring-black/10 dark:ring-white/10"
                      }`}
                      style={{
                        backgroundColor: v.color,
                        backgroundImage: "none",
                        background: v.color + " !important",
                      }}
                    />
                  )
                })}
              </div>
            )}
          </div>

          {theme.isCustom && (
            <div className="flex items-center gap-1 sm:gap-2">
              <Badge variant="secondary" className="px-1.5 text-[10px]">
                <FolderOpen className="mr-0.5 h-2.5 w-2.5" />
                {t("themes.custom.badge")}
              </Badge>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

export function ThemeSelector() {
  const { t } = useTranslation("settings")
  const { theme: currentTheme, setTheme, setVariation } = useTheme()
  const {
    customThemes,
    errors: customThemeErrors,
    directory: customThemesDirectory,
    isFetching: customThemesFetching,
    isError: customThemesError,
    refetch: refetchCustomThemes,
  } = useCustomThemes()
  // Subscribe so the picker re-renders when the async theme registry lands.
  useBuiltinThemes()

  const themeCatalog = buildThemeCatalog(themes, customThemes)

  const handleVariationSelect = (themeId: string, variationId: string) => {
    setTheme(themeId)
    setVariation(variationId)
  }

  return (
    <Card className="gap-4 py-4">
      <CardHeader className="px-4">
        <CardTitle className="flex items-center gap-2">
          <Palette className="h-5 w-5" />
          {t("themes.selector.title")}
        </CardTitle>
        <CardDescription>
          {t("themes.selector.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 px-4">
        <div
          role="list"
          aria-label={t("themes.selector.title")}
          className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3"
        >
          {themeCatalog.map((theme) => (
            <ThemeCard
              key={theme.id}
              theme={theme}
              isSelected={currentTheme === theme.id}
              onSelect={() => setTheme(theme.id)}
              onVariationSelect={handleVariationSelect}
            />
          ))}
        </div>

        <ThemeLayoutSettings />

        {/* Custom Themes (sideloaded CSS) */}
        <div className="rounded-md border bg-muted/20 p-3">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0 space-y-1">
              <h4 className="flex items-center gap-2 text-sm font-medium">
                <FolderOpen className="h-4 w-4 text-muted-foreground" />
                {t("themes.custom.title")}
                <Badge variant="secondary" className="text-[10px]">
                  {t("themes.custom.badge")}
                </Badge>
              </h4>

              {customThemesDirectory && (
                <code className="block break-all rounded bg-muted px-2 py-1 text-xs">
                  {customThemesDirectory}
                </code>
              )}
              {customThemesError ? (
                <p className="flex items-center gap-1 text-xs font-medium text-destructive">
                  <AlertTriangle className="h-3 w-3" />
                  {t("themes.custom.loadError")}
                </p>
              ) : (
                <p className="text-xs text-muted-foreground">
                  {customThemes.length === 0 && !customThemesFetching
                    ? t("themes.custom.noneFound")
                    : t("themes.custom.detectedCount", { total: customThemes.length })}
                </p>
              )}
            </div>

            <Button
              variant="outline"
              size="sm"
              onClick={() => refetchCustomThemes()}
              disabled={customThemesFetching}
            >
              <RefreshCw className={`mr-1 h-3 w-3 ${customThemesFetching ? "animate-spin" : ""}`} />
              {t("themes.custom.refresh")}
            </Button>
          </div>

          {customThemeErrors.length > 0 && (
            <div className="mt-2 space-y-1 border-t pt-2">
              <p className="flex items-center gap-1 text-xs font-medium text-destructive">
                <AlertTriangle className="h-3 w-3" />
                {t("themes.custom.parseErrorsTitle")}
              </p>
              <ul className="list-inside list-disc space-y-0.5 text-xs text-destructive">
                {customThemeErrors.map((error) => (
                  <li key={error.filename}>{error.filename}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
