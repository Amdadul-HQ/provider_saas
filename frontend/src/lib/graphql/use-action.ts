"use client"

import { useCallback } from "react"
import { useMutation, type AnyVariables, type TypedDocumentNode } from "urql"
import { toast } from "sonner"
import { useT } from "@/i18n"
import { describeError, rawErrorMessage } from "./errors"

export { describeError } from "./errors"

/**
 * Mutation + toast feedback + cache invalidation by typename (needed for mutations that return
 * scalars such as `Boolean!`, which the document cache cannot attribute to a type).
 */
export function useAction<Data, Vars extends AnyVariables>(
  document: TypedDocumentNode<Data, Vars>,
  options: { success?: string; typenames?: string[]; silent?: boolean } = {},
) {
  const t = useT()
  const [state, execute] = useMutation<Data, Vars>(document)
  const { success, typenames, silent } = options
  const typenameKey = typenames?.join(",")
  const run = useCallback(
    async (variables: Vars): Promise<Data | null> => {
      const result = await execute(variables, typenameKey ? { additionalTypenames: typenameKey.split(",") } : undefined)
      if (result.error) {
        // raw server text only as a dev-build description (BUG-07)
        if (!silent) toast.error(describeError(t, result.error), { description: rawErrorMessage(result.error) ?? undefined })
        return null
      }
      if (success) toast.success(success)
      return result.data ?? null
    },
    [execute, success, typenameKey, silent, t],
  )
  return [run, state.fetching, state.error] as const
}

/** Stable urql context objects so `useQuery` doesn't re-execute on every render. */
const contexts = new Map<string, { additionalTypenames: string[] }>()
export function typenameContext(...typenames: string[]) {
  const key = typenames.join(",")
  let ctx = contexts.get(key)
  if (!ctx) {
    ctx = { additionalTypenames: typenames }
    contexts.set(key, ctx)
  }
  return ctx
}
