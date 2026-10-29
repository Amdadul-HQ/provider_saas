"use client"

import { BanknoteIcon, BellRingIcon, BoxesIcon, MessageCircleIcon, RepeatIcon, UserPlusIcon, type LucideIcon } from "lucide-react"
import { AnimatedList } from "@/components/ui/animated-list"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"

/** One icon + tile colour per event in `t.auth.demo` (same order). */
const KINDS: { icon: LucideIcon; tile: string }[] = [
  { icon: MessageCircleIcon, tile: "bg-[#1877F2] text-white" },
  { icon: BanknoteIcon, tile: "bg-[#E2136E] text-white" }, // bKash pink
  { icon: BoxesIcon, tile: "bg-info text-info-foreground" },
  { icon: BellRingIcon, tile: "bg-warning text-warning-foreground" },
  { icon: RepeatIcon, tile: "bg-success text-success-foreground" },
  { icon: UserPlusIcon, tile: "bg-gradient-to-br from-[#F58529] via-[#DD2A7B] to-[#8134AF] text-white" },
]

function Notification({ index, position, compact }: { index: number; position: number; compact: boolean }) {
  const t = useT()
  const item = t.auth.demo[index]
  const { icon: Icon, tile } = KINDS[index % KINDS.length]
  return (
    <figure
      className={cn(
        "relative mx-auto w-full cursor-default overflow-hidden rounded-2xl bg-white text-foreground transition-transform duration-200 ease-out hover:scale-[1.03]",
        "[box-shadow:0_0_0_1px_rgba(0,0,0,.03),0_2px_4px_rgba(0,0,0,.05),0_12px_24px_rgba(0,0,0,.08)]",
        compact ? "p-3" : "p-4",
      )}
    >
      <div className="flex items-center gap-3">
        <span className={cn("flex shrink-0 items-center justify-center rounded-2xl", tile, compact ? "size-9" : "size-10")}>
          <Icon className={compact ? "size-4" : "size-5"} aria-hidden />
        </span>
        <span className="flex min-w-0 flex-col">
          <figcaption className="flex items-center gap-1.5 whitespace-nowrap">
            <span className={cn("truncate font-semibold", compact ? "text-sm" : "text-[15px]")}>{item.title}</span>
            <span className="text-muted-foreground/60" aria-hidden>
              ·
            </span>
            <span className="text-xs text-muted-foreground">{t.auth.demoTimes[Math.min(position, t.auth.demoTimes.length - 1)]}</span>
          </figcaption>
          <p className={cn("truncate text-muted-foreground", compact ? "text-xs" : "text-sm")}>{item.body}</p>
        </span>
      </div>
    </figure>
  )
}

/**
 * The auth hero's one moving part (ADR-009, user request 2026-10-29): Magic UI-style animated
 * notification stack of what SellerDesk does for a seller. Clearly labelled as a sample.
 * `compact` = phone hero strip (shorter window, smaller cards).
 */
export function SampleInbox({ className, compact = false }: { className?: string; compact?: boolean }) {
  const t = useT()
  const count = t.auth.demo.length
  return (
    <figure className={cn("relative w-full", compact && "max-w-sm", className)} aria-label={t.auth.sampleInbox}>
      {!compact ? (
        <figcaption className="mb-3 flex items-center gap-2 text-sm font-medium text-white/80">
          <span className="relative flex size-2" aria-hidden>
            <span className="absolute inline-flex size-full animate-ping rounded-full bg-hero-glow opacity-60 motion-reduce:hidden" />
            <span className="relative inline-flex size-2 rounded-full bg-hero-glow" />
          </span>
          {t.auth.sampleInbox}
        </figcaption>
      ) : null}
      <div
        className={cn(
          "overflow-hidden px-1 pt-1",
          compact ? "h-40 sm:h-48 [mask-image:linear-gradient(to_top,transparent_0%,#000_55%)]" : "h-[17.5rem] [mask-image:linear-gradient(to_top,transparent_2%,#000_45%)] 2xl:h-[20rem] [@media(max-height:760px)]:h-[12rem]",
        )}
        aria-hidden
      >
        <AnimatedList
          count={count}
          render={(index, position) => <Notification index={index} position={position} compact={compact} />}
          delay={compact ? 2000 : 1700}
          max={compact ? 3 : 4}
          className={compact ? "gap-2" : "gap-3"}
        />
      </div>
      {!compact ? <p className="mt-2 text-xs text-white/65">{t.auth.sampleInboxHint}</p> : null}
    </figure>
  )
}
