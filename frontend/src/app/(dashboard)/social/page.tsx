"use client"

import { Suspense, useCallback } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { PageHeader, SectionCard } from "@/components/shared"
import { ConnectButtons, MetaConnectCallback, SocialAccountList } from "@/modules/social"
import { useT } from "@/i18n"

function Social() {
  const t = useT()
  const router = useRouter()
  const params = useSearchParams()
  const clearParams = useCallback(() => router.replace("/social"), [router])
  return (
    <>
      <PageHeader title={t.social.title} description={t.social.subtitle} />
      <MetaConnectCallback code={params.get("code")} state={params.get("state")} onDone={clearParams} />
      <div className="space-y-4">
        <ConnectButtons />
        <SectionCard title={t.social.connected}>
          <SocialAccountList />
        </SectionCard>
      </div>
    </>
  )
}

export default function SocialPage() {
  return (
    <Suspense>
      <Social />
    </Suspense>
  )
}
