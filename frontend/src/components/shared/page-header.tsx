import type { ReactNode } from "react"
import { cn } from "@/lib/utils"

export function PageHeader({
  title,
  description,
  actions,
  back,
  className,
}: {
  title: ReactNode
  description?: ReactNode
  actions?: ReactNode
  back?: ReactNode
  className?: string
}) {
  return (
    <div className={cn("mb-4 flex flex-col gap-3 md:mb-6 md:flex-row md:items-end md:justify-between", className)}>
      <div className="min-w-0 space-y-1">
        {back}
        <h1 className="font-heading text-xl font-semibold tracking-tight md:text-2xl">{title}</h1>
        {description ? <p className="text-sm text-muted-foreground">{description}</p> : null}
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </div>
  )
}
