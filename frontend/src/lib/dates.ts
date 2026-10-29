import { differenceInCalendarDays, formatDistanceToNowStrict } from "date-fns"
import { bn, enUS } from "date-fns/locale"
import type { Locale } from "@/i18n/types"

/** All business dates are shown in Bangladesh time regardless of the device timezone. */
export const APP_TIME_ZONE = "Asia/Dhaka"

function toDate(value: string | Date): Date {
  return typeof value === "string" ? new Date(value) : value
}

function intlLocale(locale: Locale) {
  return locale === "bn" ? "bn-BD" : "en-GB"
}

export function formatDate(value: string | Date | null | undefined, locale: Locale = "bn"): string {
  if (!value) return "—"
  return new Intl.DateTimeFormat(intlLocale(locale), {
    timeZone: APP_TIME_ZONE,
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(toDate(value))
}

export function formatDateTime(value: string | Date | null | undefined, locale: Locale = "bn"): string {
  if (!value) return "—"
  return new Intl.DateTimeFormat(intlLocale(locale), {
    timeZone: APP_TIME_ZONE,
    day: "numeric",
    month: "short",
    hour: "numeric",
    minute: "2-digit",
  }).format(toDate(value))
}

export function formatTime(value: string | Date | null | undefined, locale: Locale = "bn"): string {
  if (!value) return ""
  return new Intl.DateTimeFormat(intlLocale(locale), {
    timeZone: APP_TIME_ZONE,
    hour: "numeric",
    minute: "2-digit",
  }).format(toDate(value))
}

export function formatRelative(value: string | Date | null | undefined, locale: Locale = "bn"): string {
  if (!value) return "—"
  return formatDistanceToNowStrict(toDate(value), {
    addSuffix: true,
    locale: locale === "bn" ? bn : enUS,
  })
}

/** Calendar days from today (Dhaka) to `value`; negative when in the past. */
export function daysUntil(value: string | Date): number {
  const dhakaToday = new Date(new Date().toLocaleString("en-US", { timeZone: APP_TIME_ZONE }))
  const target = new Date(toDate(value).toLocaleString("en-US", { timeZone: APP_TIME_ZONE }))
  return differenceInCalendarDays(target, dhakaToday)
}

/** `yyyy-mm-dd` (for <input type="date">) → ISO timestamp at 00:00 Dhaka (UTC+6, no DST). */
export function dateInputToISO(value: string | null | undefined): string | null {
  if (!value) return null
  return new Date(`${value}T00:00:00+06:00`).toISOString()
}

/** ISO → `yyyy-mm-dd` in Dhaka time, for <input type="date"> default values. */
export function isoToDateInput(value: string | null | undefined): string {
  if (!value) return ""
  return new Intl.DateTimeFormat("en-CA", { timeZone: APP_TIME_ZONE }).format(new Date(value))
}

/** `yyyy-mm-ddThh:mm` (datetime-local, entered as Dhaka time) → ISO. */
export function dateTimeInputToISO(value: string | null | undefined): string | null {
  if (!value) return null
  return new Date(`${value}:00+06:00`).toISOString()
}
