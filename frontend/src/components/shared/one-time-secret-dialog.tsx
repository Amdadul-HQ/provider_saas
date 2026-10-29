"use client"

import type { ReactNode } from "react"
import { TriangleAlertIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useT } from "@/i18n"
import { CopyButton } from "./copy-button"
import { ResponsiveDialog } from "./responsive-dialog"

/**
 * Shows a value the API returns exactly once (invite link, webhook token) with a copy button.
 * The caller drops the value from state on close, so it is never rendered again.
 */
export function OneTimeSecretDialog({
  value,
  onClose,
  title,
  warning,
  meta,
}: {
  value: string | null
  onClose: () => void
  title: ReactNode
  warning: ReactNode
  meta?: ReactNode
}) {
  const t = useT()
  return (
    <ResponsiveDialog
      open={value !== null}
      onOpenChange={(o) => !o && onClose()}
      title={title}
      footer={<Button onClick={onClose}>{t.common.done}</Button>}
    >
      <div className="space-y-3">
        <p className="flex gap-2 rounded-lg bg-warning/15 p-3 text-sm text-warning-foreground dark:text-warning" role="note">
          <TriangleAlertIcon className="mt-0.5 size-4 shrink-0" aria-hidden />
          <span>{warning}</span>
        </p>
        <div className="flex items-center gap-2 rounded-lg border bg-muted/40 p-2">
          <code className="min-w-0 flex-1 font-mono text-sm break-all select-all" data-testid="one-time-secret">
            {value}
          </code>
          {value ? <CopyButton value={value} /> : null}
        </div>
        {meta ? <p className="text-xs text-muted-foreground">{meta}</p> : null}
      </div>
    </ResponsiveDialog>
  )
}
