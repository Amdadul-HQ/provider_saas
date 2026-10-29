"use client"

import { useEffect } from "react"

/** Calls `fn` every `ms` while the tab is visible (cheap "live" updates without subscriptions). */
export function usePoll(fn: () => void, ms: number, enabled = true) {
  useEffect(() => {
    if (!enabled) return
    const id = setInterval(() => {
      if (document.visibilityState === "visible") fn()
    }, ms)
    return () => clearInterval(id)
  }, [fn, ms, enabled])
}
