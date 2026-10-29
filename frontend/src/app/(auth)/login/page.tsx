"use client"

import { Suspense } from "react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { LoginForm } from "@/modules/auth"
import { safeNext } from "@/lib/auth"
import { useT } from "@/i18n"

function Login() {
  const t = useT()
  const router = useRouter()
  const params = useSearchParams()
  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="font-heading text-2xl font-semibold">{t.auth.loginTitle}</h1>
        <p className="text-sm text-muted-foreground">{t.auth.loginSubtitle}</p>
      </div>
      <LoginForm onSuccess={() => router.replace(safeNext(params.get("next"), window.location.origin))} />
      <p className="text-center text-sm text-muted-foreground">
        {t.auth.noAccount}{" "}
        <Link href="/register" className="font-medium text-primary hover:underline">
          {t.auth.createAccount}
        </Link>
      </p>
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
