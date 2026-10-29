/**
 * Post-login redirect target from `?next=` (SPRINT-01-SECURITY M7).
 * Only same-origin relative paths are allowed: the raw value must start with exactly one "/"
 * (not "//" or "/\\", which browsers and the WHATWG URL parser treat as protocol-relative), and the
 * resolved URL must stay on `origin`. Anything else falls back to `/dashboard`.
 */
export function safeNext(next: string | null | undefined, origin: string, fallback = "/dashboard"): string {
  if (!next || !next.startsWith("/") || next.startsWith("//") || next.startsWith("/\\")) return fallback
  // control characters (tab/newline are stripped by the URL parser and can smuggle a second slash)
  if (/[\u0000-\u001f\u007f\\]/.test(next)) return fallback
  try {
    const url = new URL(next, origin)
    if (url.origin !== new URL(origin).origin) return fallback
    return `${url.pathname}${url.search}${url.hash}`
  } catch {
    return fallback
  }
}
