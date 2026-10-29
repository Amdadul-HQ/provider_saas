import { cn } from "cn"

/** Shimmer placeholder (ADR-009): a soft highlight sweeps across instead of the whole block pulsing. */
function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      aria-hidden
      className={cn(
        "relative overflow-hidden rounded-lg bg-muted before:absolute before:inset-0 before:animate-shimmer before:bg-linear-to-r before:from-transparent before:via-card/70 before:to-transparent dark:before:via-foreground/8",
        className
      )}
      {...props}
    />
  )
}

export { Skeleton }
