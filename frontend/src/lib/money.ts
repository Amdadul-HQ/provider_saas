/**
 * Money helpers (ADR-004). Amounts travel as integer minor units (paisa).
 * Never use floats for storage; floats only exist transiently in form inputs.
 */
import { intlTag, type Digits } from "./digits"

export type { Digits } from "./digits"

const SYMBOLS: Record<string, string> = { BDT: "৳", USD: "$", INR: "₹", EUR: "€" }

export function currencySymbol(currency = "BDT"): string {
  return SYMBOLS[currency.toUpperCase()] ?? currency.toUpperCase()
}

/** 35000 paisa → 350 (major units) */
export function fromMinor(minor: number): number {
  return minor / 100
}

/** "350.5" taka → 35050 paisa. Rounds to the nearest paisa. */
export function toMinor(major: number | string): number {
  const n = typeof major === "string" ? Number(major.replace(/[,\s৳]/g, "")) : major
  if (!Number.isFinite(n)) return 0
  return Math.round(n * 100)
}

/** en-IN gives the lakh grouping (1,00,000) familiar in BD; bn-BD the same grouping in Bengali. */
function numberLocale(digits: Digits): string {
  return digits === "beng" ? intlTag("bn-BD", "beng") : intlTag("en-IN", "latn")
}

/** Counts, quantities and percentages. Always pass the user's digit setting (`useDigits()`). */
export function formatNumber(value: number | null | undefined, digits: Digits = "latn", maxFractionDigits = 0): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return "—"
  return new Intl.NumberFormat(numberLocale(digits), { maximumFractionDigits: maxFractionDigits }).format(value)
}

/** 0.4567 → "46%" */
export function formatPercent(ratio: number | null | undefined, digits: Digits = "latn"): string {
  if (ratio === null || ratio === undefined || !Number.isFinite(ratio)) return "—"
  return `${formatNumber(Math.round(ratio * 100), digits)}%`
}

/** 35000 → "৳ 350"; 35050 → "৳ 350.50"; -1000 → "-৳ 10" */
export function formatMoney(
  minor: number | null | undefined,
  opts: { currency?: string; digits?: Digits; compact?: boolean } = {},
): string {
  const { currency = "BDT", digits = "latn", compact = false } = opts
  const value = fromMinor(minor ?? 0)
  const abs = Math.abs(value)
  const sign = value < 0 ? "-" : ""
  const locale = numberLocale(digits)
  const hasFraction = Math.round(abs * 100) % 100 !== 0
  const body = compact
    ? new Intl.NumberFormat(locale, { notation: "compact", maximumFractionDigits: 1 }).format(abs)
    : new Intl.NumberFormat(locale, {
        minimumFractionDigits: hasFraction ? 2 : 0,
        maximumFractionDigits: 2,
      }).format(abs)
  return `${sign}${currencySymbol(currency)} ${body}`
}
