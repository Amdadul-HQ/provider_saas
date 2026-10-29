"use client"

import { useRouter } from "next/navigation"
import { RegisterForm } from "@/modules/auth"
import { useT } from "@/i18n"

export default function RegisterPage() {
  const t = useT()
  const router = useRouter()
  return (
    <div className="space-y-5">
      <div className="space-y-1">
        <h1 className="font-heading text-[1.6rem] leading-tight font-bold tracking-tight">{t.auth.registerTitle}</h1>
        <p className="text-sm text-muted-foreground">{t.auth.registerSubtitle}</p>
      </div>
      <RegisterForm onSuccess={() => router.replace("/social")} />
    </div>
  )
}
