"use client"

import { print } from "graphql"
import { create } from "zustand"
import { RefreshTokenDocument, type RefreshTokenMutation, type Role, type SessionUserFragment } from "@/gql/graphql"
import { GRAPHQL_URL } from "@/lib/config"
import { clearTokens, getRefreshToken, setTokens, tenantIdFromToken } from "./tokens"

export type SessionStatus = "loading" | "authenticated" | "anonymous" | "unreachable"
export type SessionUser = SessionUserFragment

type SessionState = {
  status: SessionStatus
  user: SessionUser | null
  activeTenantId: string | null
  setAuthenticated: (payload: { accessToken: string; refreshToken: string; user: SessionUser }) => void
  setStatus: (status: SessionStatus) => void
  signOut: () => void
}

export const useSessionStore = create<SessionState>()((set) => ({
  status: "loading",
  user: null,
  activeTenantId: null,
  setAuthenticated: ({ accessToken, refreshToken, user }) => {
    setTokens(accessToken, refreshToken)
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
let inflight: Promise<RefreshResult> | null = null

/**
 * Single-flight refresh used by both the app bootstrap and the urql auth exchange, so a rotating
 * refresh token is never spent twice concurrently.
 */
export function refreshSession(): Promise<RefreshResult> {
  if (inflight) return inflight
  inflight = (async (): Promise<RefreshResult> => {
    const refreshToken = getRefreshToken()
    if (!refreshToken) {
      useSessionStore.getState().signOut()
      return "invalid"
    }
    let res: Response
    try {
      res = await fetch(GRAPHQL_URL, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ query: print(RefreshTokenDocument), variables: { refreshToken } }),
      })
    } catch {
      return "network"
    }
    if (res.status >= 500) return "network"
    try {
      const body = (await res.json()) as { data?: RefreshTokenMutation | null }
      const payload = body.data?.refreshToken
      if (!payload) {
        useSessionStore.getState().signOut()
        return "invalid"
      }
      useSessionStore.getState().setAuthenticated(payload)
      return "ok"
    } catch {
      return "network"
    }
  })().finally(() => {
    inflight = null
  })
  return inflight
}

/** Called once on app start. */
export async function bootstrapSession() {
  if (!getRefreshToken()) {
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
