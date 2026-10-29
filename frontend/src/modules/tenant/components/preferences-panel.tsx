"use client"

import { useTheme } from "next-themes"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { SectionCard } from "@/components/shared"
import { useI18nStore, useT, type Locale } from "@/i18n"

/** Device-level preferences (not tenant settings): language, digits, theme. */
export function PreferencesPanel() {
  const t = useT()
  const { locale, digits, setLocale, setDigits } = useI18nStore()
  const { theme, setTheme } = useTheme()
  return (
    <SectionCard title={t.settings.language} className="max-w-xl">
      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="pref-locale">{t.settings.language}</Label>
          <Select value={locale} onValueChange={(v) => setLocale(v as Locale)}>
            <SelectTrigger id="pref-locale" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="bn">বাংলা</SelectItem>
              <SelectItem value="en">English</SelectItem>
            </SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">{t.settings.languageHint}</p>
        </div>
        <div className="flex min-h-11 items-center justify-between rounded-lg border px-3">
          <Label htmlFor="pref-digits">{t.settings.bengaliDigits}</Label>
          <Switch id="pref-digits" checked={digits === "beng"} onCheckedChange={(c) => setDigits(c ? "beng" : "latn")} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="pref-theme">{t.settings.theme}</Label>
          <Select value={theme ?? "light"} onValueChange={setTheme}>
            <SelectTrigger id="pref-theme" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="light">{t.settings.themeLight}</SelectItem>
              <SelectItem value="dark">{t.settings.themeDark}</SelectItem>
              <SelectItem value="system">{t.settings.themeSystem}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </SectionCard>
  )
}
