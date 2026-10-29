"use client"

import { Suspense } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { LoginForm } from "@/modules/auth"
import { safeNext } from "@/lib/auth"
import { useT } from "@/i18n"

function Login() {
  const t = useT()
  const router = useRouter()
  const params = useSearchParams()
  return (
    <div className="space-y-5">
      <div className="space-y-1">
        <h1 className="font-heading text-[1.6rem] leading-tight font-bold tracking-tight">{t.auth.loginTitle}</h1>
        <p className="text-sm text-muted-foreground">{t.auth.loginSubtitle}</p>
      </div>
      <LoginForm onSuccess={() => router.replace(safeNext(params.get("next"), window.location.origin))} />
    </div>
  )
}

export default function LoginPage() {
  return (
    <Suspense>
      <Login />
    </Suspense>
  )
}
