"use client"

import { useParams } from "next/navigation"
import { useHydrated } from "@/hooks/use-hydrated"
import { PortalView } from "@/modules/storefront"

export default function PortalPage() {
  const { token } = useParams<{ token: string }>()
  const hydrated = useHydrated()
  if (!hydrated) return <div className="min-h-dvh" aria-busy />
  return <PortalView token={token} />
}
