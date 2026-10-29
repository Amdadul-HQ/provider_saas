/**
 * Token storage.
 * - Access token: memory only (never persisted) — limits XSS blast radius.
 * - Refresh token: localStorage. The API returns it in the GraphQL body, so the browser cannot
 *   set it as an httpOnly cookie itself; switch to a cookie once the backend issues one.
 */
const REFRESH_KEY = "psaas.rt"

let accessToken: string | null = null

export function getAccessToken() {
  return accessToken
}

export function getRefreshToken(): string | null {
  if (typeof window === "undefined") return null
  try {
    return window.localStorage.getItem(REFRESH_KEY)
  } catch {
    return null
  }
}

export function setTokens(access: string, refresh: string) {
  accessToken = access
  try {
    window.localStorage.setItem(REFRESH_KEY, refresh)
  } catch {
    /* private mode: session lasts until reload */
  }
}

export function clearTokens() {
  accessToken = null
  try {
    window.localStorage.removeItem(REFRESH_KEY)
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
