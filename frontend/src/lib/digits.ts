/**
 * One digit setting for the whole UI (BUG-08): "latn" → 0-9, "beng" → ০-৯.
 * Applied to money, counts, dates and every i18n string (see `useT`).
 */
export type Digits = "latn" | "beng"

const BENGALI = "০১২৩৪৫৬৭৮৯"

/** Rewrites every Latin/Bengali digit in `text` to the requested numbering system. */
export function toDigits(text: string, digits: Digits): string {
  if (digits === "beng") return text.replace(/[0-9]/g, (d) => BENGALI[Number(d)])
  return text.replace(/[০-৯]/g, (d) => String(BENGALI.indexOf(d)))
}

/** Intl locale tag with an explicit numbering system, e.g. `bn-BD-u-nu-latn`. */
export function intlTag(locale: string, digits: Digits): string {
  return `${locale}-u-nu-${digits}`
}
