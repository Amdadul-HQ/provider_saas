import type { LucideIcon } from "lucide-react"
import type { ReactNode } from "react"
import Link from "next/link"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"

/**
 * KPI tile (ADR-009): label, tonal icon chip, one big number, optional hint.
 * Linked tiles lift on hover; plain tiles stay still. Put a grid of these in `.stagger`.
 */
export function StatCard({
  label,
  value,
  hint,
  icon: Icon,
  tone = "default",
  href,
  loading,
  className,
}: {
  label: ReactNode
  value: ReactNode
  hint?: ReactNode
  icon?: LucideIcon
  tone?: "default" | "money" | "warning" | "danger" | "success"
  href?: string
  loading?: boolean
  className?: string
}) {
  const iconTone = {
    default: "bg-secondary text-secondary-foreground",
    money: "bg-accent text-accent-foreground",
    warning: "bg-warning/20 text-warning-foreground dark:text-warning",
    danger: "bg-destructive/10 text-destructive",
    success: "bg-success/12 text-success",
  }[tone]
  const body = (
    <div
      className={cn(
        "flex h-full flex-col gap-3 rounded-2xl border border-border/80 bg-card p-4 shadow-xs transition-[box-shadow,border-color,transform] duration-(--dur) ease-(--ease-smooth) md:p-5",
        href && "group-hover/stat:-translate-y-0.5 group-hover/stat:border-primary/30 group-hover/stat:shadow-md group-active/stat:translate-y-0 group-active/stat:shadow-xs",
        className,
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <span className="text-xs leading-snug font-medium text-muted-foreground md:text-[13px]">{label}</span>
        {Icon ? (
          <span className={cn("flex size-9 shrink-0 items-center justify-center rounded-xl", iconTone)}>
            <Icon className="size-4.5" aria-hidden />
          </span>
        ) : null}
      </div>
      <div className="mt-auto">
        {loading ? <Skeleton className="h-8 w-24" /> : <div className="tabular font-heading text-2xl font-bold tracking-tight md:text-[1.75rem] md:leading-none">{value}</div>}
        {hint ? <div className="mt-1.5 text-xs text-muted-foreground">{hint}</div> : null}
      </div>
    </div>
  )
  return href ? (
    <Link href={href} className="group/stat block h-full rounded-2xl outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
      {body}
    </Link>
  ) : (
    body
  )
}
