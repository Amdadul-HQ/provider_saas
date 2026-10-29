"use client"

import { useEffect, useMemo } from "react"
import { create } from "zustand"
import { persist } from "zustand/middleware"
import bn from "./bn"
import en from "./en"
import type { Locale } from "./types"
import { toDigits, type Digits } from "@/lib/digits"
import { formatMoney, formatNumber, formatPercent } from "@/lib/money"
import { formatDate, formatDateTime, formatRelative, formatTime } from "@/lib/dates"
import type * as G from "@/gql/graphql"

export type { Locale } from "./types"
export { LOCALES, DEFAULT_LOCALE } from "./types"

/** The dictionary shape; `bn` is the source of truth, `en` must match it. */
export type Dict = typeof bn

/** Every schema enum has a label map; the compiler flags missing/extra values. */
type EnumLabelMap = {
  Role: G.Role
  Platform: G.Platform
  SocialAccountStatus: G.SocialAccountStatus
  CustomerStatus: G.CustomerStatus
  CustomerSource: G.CustomerSource
  DeliveryType: G.DeliveryType
  MasterAccountStatus: G.MasterAccountStatus
  SlotStatus: G.SlotStatus
  SubscriptionStatus: G.SubscriptionStatus
  InvoiceStatus: G.InvoiceStatus
  PaymentMethod: G.PaymentMethod
  PaymentStatus: G.PaymentStatus
  LedgerEntryType: G.LedgerEntryType
  ConversationStatus: G.ConversationStatus
  TemplateCategory: G.TemplateCategory
  PostStatus: G.PostStatus
  TicketKind: G.TicketKind
  TicketPriority: G.TicketPriority
  TicketStatus: G.TicketStatus
  ReminderTrigger: G.ReminderTrigger
  ReminderChannel: G.ReminderChannel
  ReminderStatus: G.ReminderStatus
  OrderStatus: G.OrderStatus
  OrderSource: G.OrderSource
  MessageKind: G.MessageKind
  CampaignStatus: G.CampaignStatus
  ImportKind: G.ImportKind
  ImportStatus: G.ImportStatus
  AutoReplyMatch: G.AutoReplyMatch
}
export type EnumName = keyof EnumLabelMap
export type EnumValue<E extends EnumName> = EnumLabelMap[E]
type EnumLabels = { [K in EnumName]: Record<EnumLabelMap[K], string> }
// compile-time exhaustiveness check for both dictionaries
const _bnEnums: EnumLabels = bn.enums
const _enEnums: EnumLabels = en.enums
void _bnEnums
void _enEnums

const dictionaries: Record<Locale, Dict> = { bn, en }

type I18nState = {
  locale: Locale
  digits: Digits
  setLocale: (locale: Locale) => void
  setDigits: (digits: Digits) => void
}

export const useI18nStore = create<I18nState>()(
  persist(
    (set) => ({
      locale: "bn",
      digits: "latn",
      setLocale: (locale) => set({ locale }),
      setDigits: (digits) => set({ digits }),
    }),
    // skipHydration: server always renders `bn`; the client rehydrates after mount (no mismatch)
    { name: "psaas.i18n", skipHydration: true },
  ),
)

/** Call once near the root: restores the saved language and keeps <html lang> in sync. */
export function useI18nBootstrap() {
  const locale = useI18nStore((s) => s.locale)
  useEffect(() => {
    void useI18nStore.persist.rehydrate()
  }, [])
  useEffect(() => {
    document.documentElement.lang = locale
  }, [locale])
}

export function useLocale(): Locale {
  return useI18nStore((s) => s.locale)
}

export function useDigits(): Digits {
  return useI18nStore((s) => s.digits)
}

/**
 * Strings whose digits are literal (input formats, codes) and must not follow the digit setting.
 * Everything else in the dictionary — static text and the output of every template function —
 * is rewritten to the user's numbering system so one setting governs the whole UI (BUG-08).
 */
const KEEP_DIGITS = new Set([
  "auth.identifierPlaceholder",
  "errors.invalidPhone",
  "billing.trxIdPlaceholder",
  "inventory.labelPlaceholder",
  "settings.bengaliDigits",
])

const MASK_RE = /\u0000([\uE000-\uF8FF])\u0000/g

/** Wraps a template function: numbers it interpolates follow `digits`; string arguments (names, codes) are left untouched. */
function localizeFn(fn: (...args: never[]) => unknown, digits: Digits) {
  return (...args: unknown[]) => {
    const raw: string[] = []
    const masked = args.map((a) => {
      if (typeof a !== "string") return a
      raw.push(a)
      return `\u0000${String.fromCharCode(0xe000 + raw.length - 1)}\u0000`
    })
    const out = toDigits(String((fn as (...a: unknown[]) => unknown)(...masked)), digits)
    return out.replace(MASK_RE, (_m, ch: string) => raw[ch.charCodeAt(0) - 0xe000] ?? "")
  }
}

export function localizeDigits<T>(node: T, digits: Digits, path = ""): T {
  if (typeof node === "string") return (KEEP_DIGITS.has(path) ? node : toDigits(node, digits)) as T
  if (typeof node === "function") return localizeFn(node as (...args: never[]) => unknown, digits) as T
  // arrays stay arrays (Object.entries would turn them into index-keyed objects with no length)
  if (Array.isArray(node)) return node.map((v, i) => localizeDigits(v, digits, path ? `${path}.${i}` : String(i))) as T
  if (node && typeof node === "object") {
    const out: Record<string, unknown> = {}
    for (const [k, v] of Object.entries(node)) out[k] = localizeDigits(v, digits, path ? `${path}.${k}` : k)
    return out as T
  }
  return node
}

const localized = new Map<string, Dict>()
export function getLocalizedDictionary(locale: Locale, digits: Digits): Dict {
  const key = `${locale}:${digits}`
  let dict = localized.get(key)
  if (!dict) {
    dict = localizeDigits(dictionaries[locale], digits)
    localized.set(key, dict)
  }
  return dict
}

/** Typed dictionary for the active locale and digit setting: `const t = useT(); t.customers.title` */
export function useT(): Dict {
  const locale = useLocale()
  const digits = useDigits()
  return getLocalizedDictionary(locale, digits)
}

/** Formatters bound to the active locale + digit setting. Use these for every number/date render. */
export function useFormat() {
  const locale = useLocale()
  const digits = useDigits()
  return useMemo(
    () => ({
      digits,
      number: (v: number | null | undefined, maxFractionDigits = 0) => formatNumber(v, digits, maxFractionDigits),
      percent: (ratio: number | null | undefined) => formatPercent(ratio, digits),
      money: (minor: number | null | undefined, opts: { currency?: string; compact?: boolean } = {}) => formatMoney(minor, { ...opts, digits }),
      date: (v: string | Date | null | undefined) => formatDate(v, locale, digits),
      dateTime: (v: string | Date | null | undefined) => formatDateTime(v, locale, digits),
      time: (v: string | Date | null | undefined) => formatTime(v, locale, digits),
      relative: (v: string | Date | null | undefined) => formatRelative(v, locale, digits),
      /** Identifiers that are numbers (ticket #12, order #7): digits follow the setting, no grouping. */
      id: (v: number | string) => toDigits(String(v), digits),
    }),
    [locale, digits],
  )
}

/** Label for a schema enum value, e.g. `enumLabel(t, "InvoiceStatus", "PAID")`. */
export function enumLabel<E extends EnumName>(t: Dict, name: E, value: EnumValue<E> | null | undefined): string {
  return labelFor(t, name, value)
}

/** Every label namespace: schema enums (`t.enums`) plus string-typed API codes (`t.labels`). */
export type LabelName = EnumName | keyof Dict["labels"]

/** `password_changed` → "Password changed". Free text (spaces, Bengali) is returned unchanged. */
export function humanize(value: string): string {
  if (!/^[A-Za-z0-9]+([_-][A-Za-z0-9]+)*$/.test(value)) return value
  const s = value.replace(/[_-]+/g, " ").toLowerCase()
  return s.charAt(0).toUpperCase() + s.slice(1)
}

const warned = new Set<string>()

/**
 * Label for any enum-like string from the API (BUG-06). Looks the value up case-insensitively in
 * `t.labels[name]` and `t.enums[name]`; unknown codes fall back to a humanised string and warn in
 * development so the missing key gets added.
 */
export function labelFor(t: Dict, name: LabelName, value: string | null | undefined): string {
  if (value === null || value === undefined || value === "") return "—"
  const maps = [(t.labels as Record<string, Record<string, string>> | undefined)?.[name], (t.enums as Record<string, Record<string, string>> | undefined)?.[name]]
  for (const map of maps) {
    if (!map) continue
    const hit = map[value] ?? map[value.toUpperCase()] ?? map[value.toLowerCase()]
    if (hit !== undefined) return hit
  }
  const human = humanize(value)
  if (process.env.NODE_ENV === "development" && human !== value && !warned.has(`${name}.${value}`)) {
    warned.add(`${name}.${value}`)
    console.warn(`[i18n] missing label ${name}.${value} — add it to src/i18n/{bn,en}.ts`)
  }
  return human
}

/** `const label = useLabel(); label("AtRiskReason", r.reason)` */
export function useLabel() {
  const t = useT()
  return useMemo(() => (name: LabelName, value: string | null | undefined) => labelFor(t, name, value), [t])
}

export function getDictionary(locale: Locale): Dict {
  return dictionaries[locale]
}
