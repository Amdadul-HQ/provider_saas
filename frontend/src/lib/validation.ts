import { z } from "zod"
import type { Dict } from "@/i18n"

/** Bangladeshi mobile: 01XXXXXXXXX, optional +88 / 88 prefix. */
export const BD_PHONE = /^(?:\+?88)?01[3-9]\d{8}$/

export function normalizePhone(value: string): string {
  return value.replace(/[\s-]/g, "")
}

export const v = {
  requiredString: (t: Dict) => z.string().trim().min(1, t.errors.required),
  optionalString: () => z.string().trim().optional().nullable(),
  phone: (t: Dict) =>
    z
      .string()
      .trim()
      .refine((s) => BD_PHONE.test(normalizePhone(s)), t.errors.invalidPhone),
  optionalPhone: (t: Dict) =>
    z
      .string()
      .trim()
      .refine((s) => s === "" || BD_PHONE.test(normalizePhone(s)), t.errors.invalidPhone)
      .optional()
      .nullable(),
  optionalEmail: (t: Dict) =>
    z
      .string()
      .trim()
      .refine((s) => s === "" || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s), t.errors.invalidEmail)
      .optional()
      .nullable(),
  money: (t: Dict) => z.number({ error: t.errors.required }).int().min(0, t.errors.nonNegative),
  positiveMoney: (t: Dict) => z.number({ error: t.errors.required }).int().positive(t.errors.positive),
  optionalMoney: (t: Dict) => z.number().int().min(0, t.errors.nonNegative).optional().nullable(),
  positiveInt: (t: Dict) => z.number({ error: t.errors.invalidNumber }).int().positive(t.errors.positive),
  optionalInt: (t: Dict) => z.number({ error: t.errors.invalidNumber }).int().min(0, t.errors.nonNegative).optional().nullable(),
}

/** "" → null so optional GraphQL inputs are omitted rather than sent as empty strings. */
export function emptyToNull<T extends Record<string, unknown>>(obj: T): { [K in keyof T]: T[K] extends string ? T[K] | null : T[K] } {
  const out: Record<string, unknown> = {}
  for (const [k, val] of Object.entries(obj)) out[k] = typeof val === "string" && val.trim() === "" ? null : val
  return out as { [K in keyof T]: T[K] extends string ? T[K] | null : T[K] }
}
