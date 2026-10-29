"use client"

import { useEffect } from "react"
import { create } from "zustand"
import { persist } from "zustand/middleware"
import bn from "./bn"
import en from "./en"
import type { Locale } from "./types"
import type { Digits } from "@/lib/money"
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

/** Typed dictionary for the active locale: `const t = useT(); t.customers.title` */
export function useT(): Dict {
  const locale = useLocale()
  return dictionaries[locale]
}

/** Label for a schema enum value, e.g. `enumLabel(t, "InvoiceStatus", "PAID")`. */
export function enumLabel<E extends EnumName>(t: Dict, name: E, value: EnumValue<E> | null | undefined): string {
  if (!value) return "—"
  const labels = t.enums[name] as Record<string, string>
  return labels[value] ?? value
}

export function getDictionary(locale: Locale): Dict {
  return dictionaries[locale]
}
