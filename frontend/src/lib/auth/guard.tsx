"use client"

import { useEffect, type ReactNode } from "react"
import { usePathname, useRouter } from "next/navigation"
import { Loader2Icon, WifiOffIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useT } from "@/i18n"
import { useHydrated } from "@/hooks/use-hydrated"
import type { Role } from "@/gql/graphql"
import { bootstrapSession, useSession } from "./session"

function FullScreen({ children }: { children: ReactNode }) {
  return <div className="flex min-h-dvh flex-col items-center justify-center gap-4 p-6 text-center">{children}</div>
}

/** Protects dashboard routes: waits for the session, redirects anonymous users to /login. */
export function AuthGuard({ children }: { children: ReactNode }) {
  const t = useT()
  const hydrated = useHydrated()
  const { status } = useSession()
  const router = useRouter()
  const pathname = usePathname()

  useEffect(() => {
    if (status === "anonymous") router.replace(`/login?next=${encodeURIComponent(pathname)}`)
  }, [status, router, pathname])

  if (!hydrated || status === "loading" || status === "anonymous") {
    return (
      <FullScreen>
        <Loader2Icon className="size-6 animate-spin text-primary" aria-hidden />
        <p className="text-sm text-muted-foreground">{t.auth.checkingSession}</p>
      </FullScreen>
    )
  }
  if (status === "unreachable") {
    return (
      <FullScreen>
        <span className="flex size-12 items-center justify-center rounded-full bg-destructive/10 text-destructive">
          <WifiOffIcon className="size-6" aria-hidden />
        </span>
        <div className="space-y-1">
          <p className="font-medium">{t.errors.title}</p>
          <p className="max-w-sm text-sm text-muted-foreground">{t.auth.sessionUnreachable}</p>
        </div>
        <Button onClick={() => void bootstrapSession()}>{t.common.retry}</Button>
      </FullScreen>
    )
  }
  return <>{children}</>
}

/** Auth pages a signed-in user may still need (an existing user accepting an invite to another business). */
const OPEN_TO_SIGNED_IN = ["/invite/"]

/** For /login and /register: bounce signed-in users to the dashboard. */
export function GuestGuard({ children }: { children: ReactNode }) {
  const { status } = useSession()
  const router = useRouter()
  const pathname = usePathname()
  const open = OPEN_TO_SIGNED_IN.some((p) => pathname.startsWith(p))
  useEffect(() => {
    if (status === "authenticated" && !open) router.replace("/dashboard")
  }, [status, router, open])
  return <>{children}</>
}

/** Hide UI the current role cannot use (the API enforces it anyway). */
export function RoleGate({ roles, children, fallback = null }: { roles: Role[]; children: ReactNode; fallback?: ReactNode }) {
  const { hasRole } = useSession()
  return <>{hasRole(...roles) ? children : fallback}</>
}
