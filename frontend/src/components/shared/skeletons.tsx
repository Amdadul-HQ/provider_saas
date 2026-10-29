import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"

export function CardsSkeleton({ count = 4, className }: { count?: number; className?: string }) {
  return (
    <div className={cn("stagger grid grid-cols-2 gap-3 lg:grid-cols-4", className)} aria-busy>
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="space-y-4 rounded-2xl border border-border/80 bg-card p-4 shadow-xs md:p-5">
          <div className="flex items-start justify-between">
            <Skeleton className="h-3.5 w-24" />
            <Skeleton className="size-9 rounded-xl" />
          </div>
          <Skeleton className="h-8 w-20" />
        </div>
      ))}
    </div>
  )
}

export function ListSkeleton({ rows = 5, className }: { rows?: number; className?: string }) {
  return (
    <div className={cn("stagger space-y-2", className)} aria-busy>
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex items-center gap-3 py-1.5">
          <Skeleton className="size-10 shrink-0 rounded-full" />
          <div className="flex-1 space-y-2">
            <Skeleton className="h-3.5 w-[55%]" />
            <Skeleton className="h-3 w-[80%]" />
          </div>
        </div>
      ))}
    </div>
  )
}

export function PageSkeleton() {
  return (
    <div className="space-y-5" aria-busy>
      <div className="space-y-2">
        <Skeleton className="h-7 w-48" />
        <Skeleton className="h-4 w-72" />
      </div>
      <CardsSkeleton />
      <div className="rounded-2xl border border-border/80 bg-card p-5 shadow-xs">
        <ListSkeleton />
      </div>
    </div>
  )
}
