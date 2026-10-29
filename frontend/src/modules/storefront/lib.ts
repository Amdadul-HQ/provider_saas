/** `paymentNumbers` is free-form JSON in the schema; accept both shapes we expect sellers to save. */
export type PaymentNumber = { method: string; number: string; type?: string | null }

export function parsePaymentNumbers(json: unknown): PaymentNumber[] {
  if (Array.isArray(json)) {
    return json
      .filter((x): x is Record<string, unknown> => typeof x === "object" && x !== null)
      .map((x) => ({ method: String(x.method ?? ""), number: String(x.number ?? ""), type: x.type ? String(x.type) : null }))
      .filter((x) => x.number)
  }
  if (json && typeof json === "object") {
    return Object.entries(json as Record<string, unknown>).map(([method, number]) => ({ method: method.toUpperCase(), number: String(number) }))
  }
  return []
}

/** theme JSON → CSS custom properties (only a safe hex primary is honoured). */
export function themeStyle(theme: unknown): React.CSSProperties | undefined {
  const primary = theme && typeof theme === "object" ? (theme as Record<string, unknown>).primaryColor : undefined
  if (typeof primary === "string" && /^#[0-9a-f]{6}$/i.test(primary)) {
    return { "--primary": primary, "--ring": primary } as React.CSSProperties
  }
  return undefined
}
