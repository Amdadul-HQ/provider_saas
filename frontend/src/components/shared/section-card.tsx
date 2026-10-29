import type { ReactNode } from "react"
import { cn } from "@/lib/utils"

/** Titled panel used across dashboard and detail pages (ADR-009 surface: white, hairline, soft lift). */
export function SectionCard({
  title,
  description,
  actions,
  children,
  className,
  bodyClassName,
}: {
  title?: ReactNode
  description?: ReactNode
  actions?: ReactNode
  children: ReactNode
  className?: string
  bodyClassName?: string
}) {
  return (
    <section className={cn("flex flex-col rounded-2xl border border-border/80 bg-card shadow-xs", className)}>
      {title || actions ? (
        <header className="flex items-center justify-between gap-3 px-4 pt-4 pb-2 md:px-5 md:pt-5">
          <div className="min-w-0">
            <h2 className="font-heading text-[15px] leading-snug font-semibold">{title}</h2>
            {description ? <p className="mt-0.5 text-xs text-muted-foreground">{description}</p> : null}
          </div>
          {actions ? <div className="flex shrink-0 items-center gap-1">{actions}</div> : null}
        </header>
      ) : null}
      <div className={cn("flex-1 px-4 pb-4 md:px-5 md:pb-5", !title && !actions && "pt-4 md:pt-5", bodyClassName)}>{children}</div>
    </section>
  )
}

export function DetailRow({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-3 py-1.5 text-sm">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="text-right font-medium">{children}</dd>
    </div>
  )
}
