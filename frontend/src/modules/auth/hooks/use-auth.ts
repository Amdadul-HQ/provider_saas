"use client"

import { useCallback } from "react"
import { useMutation } from "urql"
import { ChangePasswordDocument, LoginDocument, LogoutDocument, RegisterDocument, SwitchTenantDocument, type LoginInput, type RegisterInput } from "@/gql/graphql"
import { getRefreshToken, useSessionStore } from "@/lib/auth"
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

export function useLogout() {
  const [, execute] = useMutation(LogoutDocument)
  const signOut = useSessionStore((s) => s.signOut)
  return useCallback(async () => {
    const refreshToken = getRefreshToken()
    if (refreshToken) await execute({ refreshToken }).catch(() => undefined)
    signOut()
    // full reload drops the urql cache and every in-memory secret
    window.location.assign("/login")
  }, [execute, signOut])
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
