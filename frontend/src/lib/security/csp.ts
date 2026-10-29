/**
 * Content-Security-Policy (SPRINT-01-SECURITY M6). Built per request in `src/middleware.ts` because
 * production uses a per-request nonce, which a static `next.config.ts` header cannot carry.
 *
 * - production: `script-src 'self' 'nonce-…' 'strict-dynamic'` — no inline/eval script without the nonce.
 *   Next.js reads the nonce from the request's CSP header and stamps it on its own scripts; the root
 *   layout passes it to next-themes' inline script.
 * - development: React Refresh / webpack need `'unsafe-eval'` and inline scripts, so the nonce is
 *   omitted (a nonce would make browsers ignore `'unsafe-inline'`).
 */
export function originOf(url: string | undefined): string | null {
  if (!url) return null
  try {
    return new URL(url).origin
  } catch {
    return null
  }
}

export function buildCsp({ nonce, dev, apiUrl }: { nonce: string; dev: boolean; apiUrl?: string }): string {
  const api = originOf(apiUrl)
  const directives: Record<string, string[]> = {
    "default-src": ["'self'"],
    "script-src": dev ? ["'self'", "'unsafe-inline'", "'unsafe-eval'"] : ["'self'", `'nonce-${nonce}'`, "'strict-dynamic'"],
    // Tailwind/Radix/next-font set inline style attributes; styles cannot execute code
    "style-src": ["'self'", "'unsafe-inline'"],
    // API origin also serves /uploads (product images, avatars) in dev
    "img-src": ["'self'", "data:", "blob:", "https:", ...(api ? [api] : [])],
    "font-src": ["'self'", "data:"],
    "connect-src": ["'self'", ...(api ? [api] : []), ...(dev ? ["ws:"] : [])],
    "manifest-src": ["'self'"],
    "worker-src": ["'self'"],
    "object-src": ["'none'"],
    "base-uri": ["'self'"],
    "form-action": ["'self'"],
    "frame-ancestors": ["'none'"],
  }
  // no `upgrade-insecure-requests`: it would rewrite an http API origin (local `next start`) to https;
  // HSTS (next.config.ts, production) already pins the app origin to https
  return Object.entries(directives)
    .map(([k, v]) => `${k} ${v.join(" ")}`)
    .join("; ")
}

/** 128-bit random nonce, base64. Works in the Edge runtime (middleware) and Node. */
export function makeNonce(): string {
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  let bin = ""
  for (const b of bytes) bin += String.fromCharCode(b)
  return btoa(bin)
}
