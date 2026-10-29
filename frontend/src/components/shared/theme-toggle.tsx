"use client"

import { MoonIcon, SunIcon } from "lucide-react"
import { useTheme } from "next-themes"
import { Button } from "@/components/ui/button"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"

/**
 * One-tap light/dark switch. Both icons render and CSS (`dark:`) picks one, so server and client
 * markup match (no hydration flash). The full light/dark/system choice stays in Settings.
 */
export function ThemeToggle({ className }: { className?: string }) {
  const t = useT()
  const { resolvedTheme, setTheme } = useTheme()
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      className={cn("rounded-full", className)}
      aria-label={t.shell.theme}
      title={t.shell.theme}
      onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
    >
      <SunIcon className="size-4.5 rotate-0 transition-transform duration-(--dur) dark:hidden" aria-hidden />
      <MoonIcon className="hidden size-4.5 transition-transform duration-(--dur) dark:block" aria-hidden />
    </Button>
  )
}
