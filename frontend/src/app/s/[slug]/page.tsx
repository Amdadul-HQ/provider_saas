"use client"

import { useParams } from "next/navigation"
import { useHydrated } from "@/hooks/use-hydrated"
import { PublicStorefrontView } from "@/modules/storefront"

export default function StorefrontPage() {
  const { slug } = useParams<{ slug: string }>()
  const hydrated = useHydrated()
  // data is fetched client-side only (no SSR fetch while the API contract is young)
  if (!hydrated) return <div className="min-h-dvh" aria-busy />
  return <PublicStorefrontView slug={slug} />
}
