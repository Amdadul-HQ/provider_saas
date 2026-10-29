"use client"

import { useSyncExternalStore } from "react"

const noop = () => () => {}

/** false during SSR and the hydration pass, true afterwards. Gate client-only data fetching on it. */
export function useHydrated(): boolean {
  return useSyncExternalStore(noop, () => true, () => false)
}
