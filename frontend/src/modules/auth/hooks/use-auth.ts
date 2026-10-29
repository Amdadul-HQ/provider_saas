"use client"

import { useCallback } from "react"
import { useMutation, useQuery } from "urql"
import {
  AcceptInviteDocument,
  ChangePasswordDocument,
  InviteDocument,
  LoginDocument,
  LogoutAllDocument,
  LogoutDocument,
  RegisterDocument,
  SwitchTenantDocument,
  type LoginInput,
  type RegisterInput,
} from "@/gql/graphql"
import { useSessionStore } from "@/lib/auth"
import { useAction } from "@/lib/graphql/use-action"
import { useT } from "@/i18n"

export function useLogin() {
  const [state, execute] = useMutation(LoginDocument)
  const setAuthenticated = useSessionStore((s) => s.setAuthenticated)
  const login = useCallback(
    async (input: LoginInput) => {
      const res = await execute({ input })
      if (res.data?.login) setAuthenticated(res.data.login)
      return res
    },
    [execute, setAuthenticated],
  )
  return { login, fetching: state.fetching, error: state.error }
}

export function useRegister() {
  const [state, execute] = useMutation(RegisterDocument)
  const setAuthenticated = useSessionStore((s) => s.setAuthenticated)
  const register = useCallback(
    async (input: RegisterInput) => {
      const res = await execute({ input })
      if (res.data?.register) setAuthenticated(res.data.register)
      return res
    },
    [execute, setAuthenticated],
  )
  return { register, fetching: state.fetching, error: state.error }
}

/** Revokes this device's refresh cookie (the API clears it), then drops in-memory state. */
export function useLogout() {
  const [, execute] = useMutation(LogoutDocument)
  const signOut = useSessionStore((s) => s.signOut)
  return useCallback(async () => {
    await execute({}).catch(() => undefined)
    signOut()
    // full reload drops the urql cache and every in-memory secret
    window.location.assign("/login")
  }, [execute, signOut])
}

/** Revokes every session of the current user on every device (token version bump). */
export function useLogoutAll() {
  const [state, execute] = useMutation(LogoutAllDocument)
  const signOut = useSessionStore((s) => s.signOut)
  const logoutAll = useCallback(async () => {
    const res = await execute({})
    if (res.error) return res
    signOut()
    window.location.assign("/login")
    return res
  }, [execute, signOut])
  return { logoutAll, fetching: state.fetching, error: state.error }
}

export function useSwitchTenant() {
  const [state, execute] = useMutation(SwitchTenantDocument)
  const setAuthenticated = useSessionStore((s) => s.setAuthenticated)
  const switchTenant = useCallback(
    async (tenantId: string) => {
      const res = await execute({ tenantId })
      if (res.data?.switchTenant) {
        setAuthenticated(res.data.switchTenant)
        window.location.assign("/dashboard")
      }
      return res
    },
    [execute, setAuthenticated],
  )
  return { switchTenant, fetching: state.fetching }
}

export function useChangePassword() {
  const t = useT()
  return useAction(ChangePasswordDocument, { success: t.auth.passwordChanged })
}

/** Public invite preview (no session needed). */
export function useInvitePreview(token: string) {
  return useQuery({ query: InviteDocument, variables: { token }, requestPolicy: "network-only" })
}

export function useAcceptInvite() {
  const [state, execute] = useMutation(AcceptInviteDocument)
  const setAuthenticated = useSessionStore((s) => s.setAuthenticated)
  const accept = useCallback(
    async (vars: { token: string; password: string; name: string | null }) => {
      const res = await execute(vars)
      if (res.data?.acceptInvite) setAuthenticated(res.data.acceptInvite)
      return res
    },
    [execute, setAuthenticated],
  )
  return { accept, fetching: state.fetching, error: state.error }
}
