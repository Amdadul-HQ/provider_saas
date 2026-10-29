"use client"

import type { CombinedError } from "@urql/core"
import { RefreshCwIcon, WifiOffIcon, LockIcon, SearchXIcon, TriangleAlertIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { errorKind, serverMessage } from "@/lib/graphql/errors"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"

export function ErrorState({
  error,
  onRetry,
  title,
  className,
  compact = false,
}: {
  error?: CombinedError | Error | null
  onRetry?: () => void
  title?: string
  className?: string
  compact?: boolean
}) {
  const t = useT()
  const kind = errorKind(error) ?? "generic"
  const Icon =
    kind === "network" ? WifiOffIcon : kind === "forbidden" || kind === "unauthenticated" ? LockIcon : kind === "notFound" ? SearchXIcon : TriangleAlertIcon
  const message =
    kind === "network"
      ? t.errors.network
      : kind === "unauthenticated"
        ? t.errors.unauthenticated
        : kind === "forbidden"
          ? t.errors.forbidden
          : kind === "notFound"
            ? t.errors.notFound
            : (serverMessage(error) ?? t.errors.generic)
  return (
    <div
      role="alert"
      className={cn(
        "flex flex-col items-center justify-center gap-3 rounded-xl border border-destructive/20 bg-destructive/5 text-center",
        compact ? "p-5" : "p-10",
        className,
      )}
    >
      <div className="flex size-11 items-center justify-center rounded-full bg-destructive/10 text-destructive">
        <Icon className="size-5" aria-hidden />
      </div>
      <div className="space-y-1">
        <p className="font-medium">{title ?? t.errors.title}</p>
        <p className="mx-auto max-w-sm text-sm text-muted-foreground">{message}</p>
      </div>
      {onRetry ? (
        <Button variant="outline" onClick={onRetry}>
          <RefreshCwIcon /> {t.common.retry}
        </Button>
      ) : null}
    </div>
  )
}
