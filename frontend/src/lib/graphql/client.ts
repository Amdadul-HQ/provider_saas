import { cacheExchange, Client, fetchExchange, type Operation } from "@urql/core"
import { authExchange } from "@urql/exchange-auth"
import { Kind } from "graphql"
import { GRAPHQL_URL } from "@/lib/config"
import { refreshSession, useSessionStore } from "@/lib/auth/session"
import { accessTokenExpiring, hasSessionHint } from "@/lib/auth/tokens"
import { API_CREDENTIALS, API_HEADERS } from "./transport"

/** Operations that must work without a session (public storefront/portal + auth itself). */
const PUBLIC_OPERATIONS = new Set([
  "Login",
  "Register",
  "RefreshToken",
  "Logout",
  "PublicStorefront",
  "PublicOrder",
  "PlaceOrder",
  "Portal",
  "PortalRequestRenewal",
  "Invite",
  "AcceptInvite",
])

function operationName(op: Operation): string | null {
  for (const def of op.query.definitions) {
    if (def.kind === Kind.OPERATION_DEFINITION) return def.name?.value ?? null
  }
  return null
}

function isPublic(op: Operation) {
  const name = operationName(op)
  return name !== null && PUBLIC_OPERATIONS.has(name)
}

export function makeClient() {
  return new Client({
    url: GRAPHQL_URL,
    // cookie auth mode (M6, ADR-011): the access and refresh cookies travel with every request;
    // the custom header is the CSRF guard. No token is ever added by JavaScript.
    fetchOptions: { credentials: API_CREDENTIALS, headers: { ...API_HEADERS } },
    requestPolicy: "cache-and-network",
    // urql >=4 defaults to GET for queries; always POST so gqlgen needs no GET transport and
    // query variables (phones, TrxIDs) never land in proxy/access logs via the URL.
    preferGetMethod: false,
    exchanges: [
      cacheExchange,
      authExchange(async () => ({
        addAuthToOperation(operation) {
          return operation // the httpOnly psaas_at cookie authenticates
        },
        willAuthError(operation) {
          if (isPublic(operation)) return false
          // access cookie expired/expiring (or unknown after a reload) but a session may exist → refresh first
          return hasSessionHint() && accessTokenExpiring()
        },
        didAuthError(error, operation) {
          // a failed login/acceptInvite is "wrong password", not an expired session: never refresh + retry it
          if (isPublic(operation)) return false
          const status = (error.response as { status?: number } | undefined)?.status
          return status === 401 || error.graphQLErrors.some((e) => e.extensions?.code === "UNAUTHENTICATED")
        },
        async refreshAuth() {
          const result = await refreshSession()
          if (result === "invalid") useSessionStore.getState().signOut()
        },
      })),
      fetchExchange,
    ],
  })
}
