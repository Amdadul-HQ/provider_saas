"use client"

import type { ReactNode } from "react"
import type { CombinedError } from "@urql/core"
import { ErrorState } from "./error-state"
import { ListSkeleton } from "./skeletons"

/**
 * Standard loading / error / empty switch for a urql result.
 * Renders `children(data)` only when data is present and not empty.
 */
export function QueryState<T>({
  data,
  fetching,
  error,
  onRetry,
  isEmpty,
  empty,
  loading,
  children,
  compact,
}: {
  data: T | null | undefined
  fetching: boolean
  error?: CombinedError | null
  onRetry?: () => void
  isEmpty?: (data: T) => boolean
  empty?: ReactNode
  loading?: ReactNode
  children: (data: T) => ReactNode
  compact?: boolean
}) {
  if (data === undefined || data === null) {
    if (error) return <ErrorState error={error} onRetry={onRetry} compact={compact} />
    if (fetching) return <>{loading ?? <ListSkeleton />}</>
    return <>{empty ?? null}</>
  }
  if (isEmpty?.(data)) return <>{empty ?? null}</>
  return <>{children(data)}</>
}
