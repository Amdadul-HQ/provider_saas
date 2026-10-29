import type { LucideIcon } from "lucide-react"
import type { ReactNode } from "react"
import Link from "next/link"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"

export function StatCard({
  label,
  value,
  hint,
  icon: Icon,
  tone = "default",
  href,
  loading,
}: {
  label: ReactNode
  value: ReactNode
  hint?: ReactNode
  icon?: LucideIcon
  tone?: "default" | "money" | "warning" | "danger" | "success"
  href?: string
  loading?: boolean
}) {
  const iconTone = {
    default: "bg-secondary text-secondary-foreground",
    money: "bg-accent text-accent-foreground",
    warning: "bg-warning/20 text-warning-foreground dark:text-warning",
    danger: "bg-destructive/10 text-destructive",
    success: "bg-success/12 text-success",
  }[tone]
  const body = (
    <div className="flex h-full flex-col gap-2 rounded-xl border bg-card p-3.5 transition-colors md:p-4">
      <div className="flex items-start justify-between gap-2">
        <span className="text-xs leading-snug font-medium text-muted-foreground md:text-sm">{label}</span>
        {Icon ? (
          <span className={cn("flex size-8 shrink-0 items-center justify-center rounded-lg", iconTone)}>
            <Icon className="size-4" aria-hidden />
          </span>
        ) : null}
      </div>
      {loading ? <Skeleton className="h-7 w-20" /> : <div className="tabular text-xl font-semibold tracking-tight md:text-2xl">{value}</div>}
      {hint ? <div className="text-xs text-muted-foreground">{hint}</div> : null}
    </div>
  )
  return href ? (
    <Link href={href} className="block rounded-xl outline-none hover:[&>div]:border-primary/40 focus-visible:ring-3 focus-visible:ring-ring/50">
      {body}
    </Link>
  ) : (
    body
  )
}
