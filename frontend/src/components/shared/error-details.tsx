"use client"

import type { CombinedError } from "@urql/core"
import { rawErrorMessage } from "@/lib/graphql/errors"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"

/** Collapsible raw error text — rendered only in development builds (BUG-07). */
export function ErrorDetails({ error, className }: { error: CombinedError | Error | string | null | undefined; className?: string }) {
  const t = useT()
  const raw = rawErrorMessage(error)
  if (!raw) return null
  return (
    <details className={cn("text-left text-[11px] text-muted-foreground", className)}>
      <summary className="cursor-pointer select-none">{t.errors.devDetails}</summary>
      <pre className="mt-1 max-w-full overflow-x-auto font-mono break-words whitespace-pre-wrap">{raw}</pre>
    </details>
  )
}
