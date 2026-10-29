# ADR-011 Access token in an httpOnly cookie (web app)
Status: accepted · 2026-10-29 · extends SECURITY M6 (refresh cookie)

## Context
M6 put the refresh token in the httpOnly `psaas_rt` cookie and kept the access token in
JavaScript memory. The founder asked for both tokens in cookies, so the web app never holds
a token at all, and a reload never depends on JavaScript state.

## Decision
In cookie mode (`X-Auth-Mode: cookie`, sent by the web app on every request):
- Every auth payload (login, register, refreshToken, switchTenant, acceptInvite) sets two cookies:
  - `psaas_at`: access JWT; HttpOnly, SameSite=Lax, Path=/, Secure outside development,
    Max-Age = access TTL (15 min).
  - `psaas_rt`: refresh token, unchanged (Path=/graphql, Max-Age = refresh TTL, rotated on use).
- `AuthPayload.accessToken` and `refreshToken` are null. The payload carries
  `accessTokenExpiresAt` (so the client refreshes before expiry without decoding a JWT) and
  `tenantId` (the business the session is scoped to).
- The auth middleware reads `Authorization: Bearer` first (API/mobile clients, body mode),
  then the `psaas_at` cookie. Both go through the same Verify (signature, exp, token_version).
- CSRFGuard rejects any /graphql request that carries either cookie but has neither
  `X-Requested-With: XMLHttpRequest` nor an allowed Origin. SameSite=Lax plus no GraphQL
  mutations over GET cover the rest.
- logout, and a failed refresh, clear both cookies.
- The web app keeps no token: urql sends `credentials: "include"`, refreshes when
  `accessTokenExpiresAt` is near or the API answers UNAUTHENTICATED, then retries.

Body mode (no header) is unchanged: both tokens come back in the payload.

## Consequences
- XSS can no longer read either token; it can still act as the user while the page is open
  (same as before), so CSP and output escaping still matter.
- Any new REST route that reads the principal from the cookie (e.g. POST /uploads) must sit
  behind CSRFGuard.
