"use client"

import { useEffect } from "react"
import { ErrorState } from "@/components/shared"

/** Route-level boundary: a render error shows a friendly retry instead of a blank "Application error". */
export default function RouteError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    console.error(error)
  }, [error])
  return (
    <div className="mx-auto max-w-lg p-6 pt-16">
      <ErrorState error={error} onRetry={reset} />
    </div>
  )
}
