/**
 * Client-side preview of message-template placeholders (BUG-17). Mirrors the backend's
 * `inbox.Render`: `{{ name }}` is case-insensitive; placeholders without a value are kept as-is so
 * the seller sees what still needs filling before sending.
 */
export type TemplateVars = Partial<Record<string, string | null | undefined>>

export type RenderedTemplate = {
  text: string
  /** Placeholders left in `text` (unknown names or known names without a value), lower-cased, unique. */
  missing: string[]
}

const PLACEHOLDER = /\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}/g

/** Aliases used by older templates and the settings hint (`{{name}}`, `{{expires_at}}`). */
const ALIASES: Record<string, string> = { name: "customer", expires_at: "expiry", expires: "expiry", due: "amount" }

export function renderTemplate(body: string, vars: TemplateVars): RenderedTemplate {
  const missing = new Set<string>()
  const text = body.replace(PLACEHOLDER, (match, raw: string) => {
    const key = raw.toLowerCase()
    const value = vars[key] ?? vars[ALIASES[key] ?? ""]
    if (value !== null && value !== undefined && value.trim() !== "") return value
    missing.add(key)
    return match
  })
  return { text, missing: [...missing] }
}

/** Placeholders still present in `text` (e.g. after the seller edited the composer). */
export function unfilledPlaceholders(text: string): string[] {
  return [...new Set([...text.matchAll(PLACEHOLDER)].map((m) => m[1].toLowerCase()))]
}
