/** Client-side password guidance for sign-up and password changes. The API enforces only `min 8`. */
export type PasswordChecks = { length: boolean; letterDigit: boolean; extra: boolean }

export function passwordChecks(p: string): PasswordChecks {
  return {
    length: p.length >= 8,
    letterDigit: /\p{L}/u.test(p) && /\p{Nd}/u.test(p),
    extra: p.length >= 12 || /[^\p{L}\p{M}\p{Nd}\p{Cf}\s]/u.test(p),
  }
}

/** 0 = empty, 1 = weak, 2 = fair (meets the rules), 3 = strong. */
export function passwordScore(p: string): 0 | 1 | 2 | 3 {
  if (!p) return 0
  const c = passwordChecks(p)
  if (!c.length || !c.letterDigit) return 1
  return c.extra ? 3 : 2
}
