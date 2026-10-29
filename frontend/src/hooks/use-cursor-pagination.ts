"use client"

import { useCallback, useMemo, useState } from "react"
import type { PageInput } from "@/gql/graphql"

/**
 * Cursor pagination state for `PageInput { first, after }`.
 * Keeps a stack of previous cursors so "previous page" works with forward-only cursors.
 */
export function useCursorPagination(pageSize = 25) {
  const [stack, setStack] = useState<(string | null)[]>([null])
  const after = stack[stack.length - 1] ?? null
  const page: PageInput = useMemo(() => ({ first: pageSize, after }), [pageSize, after])

  const next = useCallback((endCursor: string | null | undefined) => {
    if (!endCursor) return
    setStack((s) => [...s, endCursor])
  }, [])
  const prev = useCallback(() => setStack((s) => (s.length > 1 ? s.slice(0, -1) : s)), [])
  const reset = useCallback(() => setStack([null]), [])

  return { page, next, prev, reset, pageIndex: stack.length - 1, canPrev: stack.length > 1 }
}

/**
 * Some list queries return plain arrays (no Connection). Assumption until the backend adds
 * connections: when a full page comes back there may be more, and the cursor is the last row id.
 */
export function arrayPageInfo<T extends { id: string }>(rows: T[] | undefined, pageSize: number) {
  const list = rows ?? []
  return {
    hasNextPage: list.length >= pageSize,
    endCursor: list.length ? list[list.length - 1]!.id : null,
  }
}
