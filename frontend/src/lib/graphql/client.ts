import { cacheExchange, Client, fetchExchange, type Operation } from "@urql/core"
import { authExchange } from "@urql/exchange-auth"
import { Kind } from "graphql"
import { GRAPHQL_URL } from "@/lib/config"
import { refreshSession, useSessionStore } from "@/lib/auth/session"
import { accessTokenExpiring, getAccessToken, getRefreshToken } from "@/lib/auth/tokens"

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
    requestPolicy: "cache-and-network",
    // urql >=4 defaults to GET for queries; always POST so gqlgen needs no GET transport and
    // query variables (phones, TrxIDs) never land in proxy/access logs via the URL.
    preferGetMethod: false,
    exchanges: [
      cacheExchange,
      authExchange(async (utils) => ({
        addAuthToOperation(operation) {
          const token = getAccessToken()
          if (!token || isPublic(operation)) return operation
          return utils.appendHeaders(operation, { Authorization: `Bearer ${token}` })
        },
        willAuthError(operation) {
          if (isPublic(operation)) return false
          // no/expiring access token but we can refresh → refresh before sending
          return getRefreshToken() !== null && accessTokenExpiring()
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
