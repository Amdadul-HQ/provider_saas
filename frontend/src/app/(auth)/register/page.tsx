"use client"

import Link from "next/link"
import { useRouter } from "next/navigation"
import { RegisterForm } from "@/modules/auth"
import { useT } from "@/i18n"

export default function RegisterPage() {
  const t = useT()
  const router = useRouter()
  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="font-heading text-2xl font-semibold">{t.auth.registerTitle}</h1>
        <p className="text-sm text-muted-foreground">{t.auth.registerSubtitle}</p>
      </div>
      <RegisterForm onSuccess={() => router.replace("/social")} />
      <p className="text-center text-sm text-muted-foreground">
        {t.auth.haveAccount}{" "}
        <Link href="/login" className="font-medium text-primary hover:underline">
          {t.auth.login}
        </Link>
      </p>
    </div>
  )
}
