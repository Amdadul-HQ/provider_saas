/**
 * Token storage (SPRINT-01-SECURITY M6, cookie mode).
 * - Access token: memory only (never persisted) — limits XSS blast radius.
 * - Refresh token: an httpOnly cookie the API sets when the request carries `X-Auth-Mode: cookie`.
 *   JavaScript never sees it. Only a non-secret "a session may exist" hint is kept in localStorage,
 *   so anonymous visitors (storefront, portal) don't fire a refresh request on every page load.
 * - Legacy: Sprint 1 kept the refresh token in localStorage (`psaas.rt`). `refreshSession` spends it
 *   once to obtain the cookie and then deletes it.
 */
const LEGACY_REFRESH_KEY = "psaas.rt"
const SESSION_HINT_KEY = "psaas.session"

let accessToken: string | null = null

function storage(): Storage | null {
  if (typeof window === "undefined") return null
  try {
    return window.localStorage
  } catch {
    return null
  }
}

export function getAccessToken() {
  return accessToken
}

/** Stores the access token in memory and remembers that a cookie session exists. */
export function setAccessToken(access: string) {
  accessToken = access
  try {
    storage()?.setItem(SESSION_HINT_KEY, "1")
  } catch {
    /* private mode: the hint is an optimisation only */
  }
}

/** True when this browser has (probably) got a refresh cookie: a past login or a legacy token to migrate. */
export function hasSessionHint(): boolean {
  const s = storage()
  if (!s) return false
  try {
    return s.getItem(SESSION_HINT_KEY) === "1" || s.getItem(LEGACY_REFRESH_KEY) !== null
  } catch {
    return false
  }
}

/** Sprint 1 refresh token still sitting in localStorage (to be exchanged for the cookie once). */
export function getLegacyRefreshToken(): string | null {
  try {
    return storage()?.getItem(LEGACY_REFRESH_KEY) ?? null
  } catch {
    return null
  }
}

export function clearLegacyRefreshToken() {
  try {
    storage()?.removeItem(LEGACY_REFRESH_KEY)
  } catch {
    /* ignore */
  }
}

export function clearTokens() {
  accessToken = null
  try {
    storage()?.removeItem(SESSION_HINT_KEY)
    storage()?.removeItem(LEGACY_REFRESH_KEY)
  } catch {
    /* ignore */
  }
}

type JwtClaims = { exp?: number; tid?: string; tenant_id?: string; tenantId?: string; [k: string]: unknown }

export function decodeJwt(token: string | null): JwtClaims | null {
  if (!token) return null
  const part = token.split(".")[1]
  if (!part) return null
  try {
    const json = atob(part.replace(/-/g, "+").replace(/_/g, "/"))
    return JSON.parse(json) as JwtClaims
  } catch {
    return null
  }
}

/** True when the access token is missing or expires within `skewSeconds`. */
export function accessTokenExpiring(skewSeconds = 30): boolean {
  const claims = decodeJwt(accessToken)
  if (!claims?.exp) return accessToken === null
  return claims.exp * 1000 - skewSeconds * 1000 < Date.now()
}

export function tenantIdFromToken(): string | null {
  const c = decodeJwt(accessToken)
  return (c?.tenant_id ?? c?.tid ?? c?.tenantId ?? null) as string | null
}
