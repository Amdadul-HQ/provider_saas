/**
 * Request options shared by every call to the API (GraphQL, refresh, uploads).
 * - `X-Auth-Mode: cookie`: the API sets/reads the refresh token as an httpOnly cookie and returns
 *   `refreshToken: null` in `AuthPayload` (SPRINT-01-SECURITY M6).
 * - `X-Requested-With`: a custom header forces a CORS preflight, so a cross-site form post can never
 *   ride on the refresh cookie (CSRF guard the API checks).
 * - `credentials: "include"`: the refresh cookie must travel with cross-origin API requests.
 */
export const API_HEADERS: Readonly<Record<string, string>> = Object.freeze({
  "X-Auth-Mode": "cookie",
  "X-Requested-With": "XMLHttpRequest",
})

export const API_CREDENTIALS: RequestCredentials = "include"
