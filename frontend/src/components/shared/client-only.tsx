"use client"

import type { ReactNode } from "react"
import { useHydrated } from "@/hooks/use-hydrated"

/** Renders `children` only after hydration (client-only data such as public pages fetched with urql). */
export function ClientOnly({ children, fallback = <div className="min-h-dvh" aria-busy /> }: { children: ReactNode; fallback?: ReactNode }) {
  const hydrated = useHydrated()
  return hydrated ? children : fallback
}
