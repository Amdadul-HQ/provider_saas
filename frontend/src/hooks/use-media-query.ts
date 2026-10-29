"use client"

import { useSyncExternalStore } from "react"

/** SSR-safe media query; the server snapshot assumes mobile (mobile-first). */
export function useMediaQuery(query: string): boolean {
  return useSyncExternalStore(
    (onChange) => {
      const mql = window.matchMedia(query)
      mql.addEventListener("change", onChange)
      return () => mql.removeEventListener("change", onChange)
    },
    () => window.matchMedia(query).matches,
    () => false,
  )
}

export function useIsDesktop() {
  return useMediaQuery("(min-width: 768px)")
}
