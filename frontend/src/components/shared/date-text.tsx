"use client"

import { formatDate, formatDateTime, formatRelative } from "@/lib/dates"
import { useLocale, useT } from "@/i18n"
import { cn } from "@/lib/utils"

export function DateText({
  value,
  withTime = false,
  relative = false,
  className,
}: {
  value: string | null | undefined
  withTime?: boolean
  relative?: boolean
  className?: string
}) {
  const locale = useLocale()
  if (!value) return <span className={cn("text-muted-foreground", className)}>—</span>
  const text = relative ? formatRelative(value, locale) : withTime ? formatDateTime(value, locale) : formatDate(value, locale)
  return (
    <time dateTime={value} title={formatDateTime(value, locale)} className={cn("whitespace-nowrap", className)}>
      {text}
    </time>
  )
}

/** "৩ দিন বাকি" coloured by urgency. */
export function DaysLeft({ days, className }: { days: number; className?: string }) {
  const t = useT()
  const tone = days < 0 ? "text-destructive" : days <= 3 ? "text-warning-foreground dark:text-warning" : "text-muted-foreground"
  return <span className={cn("text-xs font-medium whitespace-nowrap", tone, className)}>{t.common.daysLeft(days)}</span>
}
