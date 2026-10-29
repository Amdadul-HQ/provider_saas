/**
 * Session bookkeeping in cookie mode (SPRINT-01-SECURITY M6, ADR-011). The web app holds no token:
 * - Access token: the httpOnly `psaas_at` cookie (15 min). JavaScript only remembers *when* it
 *   expires (`accessTokenExpiresAt` from the auth payload) so requests refresh just before.
 * - Refresh token: the httpOnly `psaas_rt` cookie, rotated on every refresh.
 * - localStorage keeps only a non-secret "a session may exist" hint, so anonymous visitors
 *   (storefront, portal) don't fire a refresh request on every page load.
 * - Legacy: Sprint 1 kept the refresh token in localStorage (`psaas.rt`). `refreshSession` spends it
 *   once to obtain the cookies and then deletes it.
 */
const LEGACY_REFRESH_KEY = "psaas.rt"
const SESSION_HINT_KEY = "psaas.session"

/** Epoch ms when the access cookie expires; null = unknown (fresh page load) or signed out. */
let accessExpiresAt: number | null = null

function storage(): Storage | null {
  if (typeof window === "undefined") return null
  try {
    return window.localStorage
  } catch {
    return null
  }
}

/** Records a fresh session: when its access cookie expires, and that a cookie session exists. */
export function setAccessExpiry(expiresAt: string) {
  const ms = Date.parse(expiresAt)
  accessExpiresAt = Number.isNaN(ms) ? null : ms
  try {
    storage()?.setItem(SESSION_HINT_KEY, "1")
  } catch {
    /* private mode: the hint is an optimisation only */
  }
}

/** True when this browser has (probably) got session cookies: a past login or a legacy token to migrate. */
export function hasSessionHint(): boolean {
  const s = storage()
  if (!s) return false
  try {
    return s.getItem(SESSION_HINT_KEY) === "1" || s.getItem(LEGACY_REFRESH_KEY) !== null
  } catch {
    return false
  }
}

/** Sprint 1 refresh token still sitting in localStorage (to be exchanged for the cookies once). */
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

/** Forgets the session locally. The API clears the cookies on logout or a failed refresh. */
export function clearTokens() {
  accessExpiresAt = null
  try {
    storage()?.removeItem(SESSION_HINT_KEY)
    storage()?.removeItem(LEGACY_REFRESH_KEY)
  } catch {
    /* ignore */
  }
}

/** True when the access cookie is gone or expires within `skewSeconds` (unknown expiry counts as expiring). */
export function accessTokenExpiring(skewSeconds = 30): boolean {
  if (accessExpiresAt === null) return true
  return accessExpiresAt - skewSeconds * 1000 < Date.now()
}
