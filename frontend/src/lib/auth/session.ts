"use client"

import { print } from "graphql"
import { create } from "zustand"
import { RefreshTokenDocument, type RefreshTokenMutation, type Role, type SessionUserFragment } from "@/gql/graphql"
import { GRAPHQL_URL } from "@/lib/config"
import { API_CREDENTIALS, API_HEADERS } from "@/lib/graphql/transport"
import { clearLegacyRefreshToken, clearTokens, getLegacyRefreshToken, hasSessionHint, setAccessToken, tenantIdFromToken } from "./tokens"

export type SessionStatus = "loading" | "authenticated" | "anonymous" | "unreachable"
export type SessionUser = SessionUserFragment

/** What the auth mutations return. In cookie mode there is no refresh token in the body. */
export type AuthPayloadLike = { accessToken: string; user: SessionUser }

type SessionState = {
  status: SessionStatus
  user: SessionUser | null
  activeTenantId: string | null
  setAuthenticated: (payload: AuthPayloadLike) => void
  setStatus: (status: SessionStatus) => void
  signOut: () => void
}

export const useSessionStore = create<SessionState>()((set) => ({
  status: "loading",
  user: null,
  activeTenantId: null,
  setAuthenticated: ({ accessToken, user }) => {
    // a refreshToken in the payload (non-cookie server) is deliberately ignored: never persisted
    setAccessToken(accessToken)
    const fromToken = tenantIdFromToken()
    const active = user.memberships.find((m) => m.tenant.id === fromToken) ?? user.memberships[0]
    set({ status: "authenticated", user, activeTenantId: active?.tenant.id ?? null })
  },
  setStatus: (status) => set({ status }),
  signOut: () => {
    clearTokens()
    set({ status: "anonymous", user: null, activeTenantId: null })
  },
}))

export type RefreshResult = "ok" | "invalid" | "network"

type GraphQLBody = { data?: RefreshTokenMutation | null; errors?: { extensions?: { code?: string } }[] }

/** One `refreshToken` call. `legacyToken` is passed only to migrate a Sprint 1 localStorage token. */
async function exchange(legacyToken: string | null): Promise<RefreshResult> {
  let res: Response
  try {
    res = await fetch(GRAPHQL_URL, {
      method: "POST",
      credentials: API_CREDENTIALS,
      headers: { "content-type": "application/json", ...API_HEADERS },
      body: JSON.stringify({ query: print(RefreshTokenDocument), variables: legacyToken ? { refreshToken: legacyToken } : {} }),
    })
  } catch {
    return "network"
  }
  if (res.status === 401 || res.status === 403) return "invalid"
  if (res.status >= 500) return "network"
  let body: GraphQLBody
  try {
    body = (await res.json()) as GraphQLBody
  } catch {
    return "network"
  }
  const payload = body.data?.refreshToken
  if (!payload) return "invalid"
  useSessionStore.getState().setAuthenticated(payload)
  return "ok"
}

let inflight: Promise<RefreshResult> | null = null

/**
 * Single-flight refresh used by both the app bootstrap and the urql auth exchange, so a rotating
 * refresh cookie is never spent twice concurrently. A legacy localStorage token is spent once to
 * obtain the cookie and then deleted (unless the API was unreachable, so it can be retried).
 */
export function refreshSession(): Promise<RefreshResult> {
  if (inflight) return inflight
  inflight = (async (): Promise<RefreshResult> => {
    const legacy = getLegacyRefreshToken()
    let result = await exchange(legacy)
    if (legacy && result !== "network") {
      clearLegacyRefreshToken()
      // an invalid legacy token says nothing about the cookie: try the cookie once
      if (result === "invalid") result = await exchange(null)
    }
    if (result === "invalid") useSessionStore.getState().signOut()
    return result
  })().finally(() => {
    inflight = null
  })
  return inflight
}

/** Called once on app start. */
export async function bootstrapSession() {
  if (!hasSessionHint()) {
    useSessionStore.getState().setStatus("anonymous")
    return
  }
  useSessionStore.getState().setStatus("loading")
  const result = await refreshSession()
  if (result === "network") useSessionStore.getState().setStatus("unreachable")
}

export function useSession() {
  const status = useSessionStore((s) => s.status)
  const user = useSessionStore((s) => s.user)
  const activeTenantId = useSessionStore((s) => s.activeTenantId)
  const membership = user?.memberships.find((m) => m.tenant.id === activeTenantId) ?? null
  const role: Role | null = membership?.role ?? null
  return {
    status,
    user,
    membership,
    tenant: membership?.tenant ?? null,
    role,
    hasRole: (...roles: Role[]) => (role ? roles.includes(role) : false),
  }
}
