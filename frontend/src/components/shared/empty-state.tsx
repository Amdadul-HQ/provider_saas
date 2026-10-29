import type { LucideIcon } from "lucide-react"
import { InboxIcon } from "lucide-react"
import type { ReactNode } from "react"
import { cn } from "@/lib/utils"

/** An empty screen is an invitation to act: icon in a soft halo, one line, one action. */
export function EmptyState({
  icon: Icon = InboxIcon,
  title,
  description,
  action,
  className,
  compact = false,
}: {
  icon?: LucideIcon
  title: ReactNode
  description?: ReactNode
  action?: ReactNode
  className?: string
  compact?: boolean
}) {
  return (
    <div
      className={cn(
        "animate-pop flex flex-col items-center justify-center rounded-2xl border border-dashed border-border bg-muted/40 text-center",
        compact ? "gap-2.5 p-6" : "gap-4 p-10 md:p-14",
        className,
      )}
    >
      <div className={cn("flex items-center justify-center rounded-full bg-primary/6", compact ? "size-14" : "size-20")}>
        <div className={cn("flex items-center justify-center rounded-full bg-secondary text-primary", compact ? "size-10" : "size-14")}>
          <Icon className={compact ? "size-5" : "size-6"} aria-hidden />
        </div>
      </div>
      <div className="space-y-1">
        <p className="font-medium">{title}</p>
        {description ? <p className="mx-auto max-w-sm text-sm text-muted-foreground">{description}</p> : null}
      </div>
      {action}
    </div>
  )
}
