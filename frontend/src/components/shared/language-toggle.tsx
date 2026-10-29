"use client"

import { LanguagesIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useI18nStore, useT } from "@/i18n"

/** One-tap বাংলা ⇄ English switch, persisted in localStorage. */
export function LanguageToggle({ className }: { className?: string }) {
  const t = useT()
  const locale = useI18nStore((s) => s.locale)
  const setLocale = useI18nStore((s) => s.setLocale)
  return (
    <Button
      variant="ghost"
      size="sm"
      className={className}
      aria-label={t.shell.language}
      onClick={() => setLocale(locale === "bn" ? "en" : "bn")}
    >
      <LanguagesIcon />
      {locale === "bn" ? "EN" : "বাং"}
    </Button>
  )
}
