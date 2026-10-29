"use client"

import type { ReactNode } from "react"
import Link from "next/link"
import { CalendarClockIcon, CheckCircle2Icon, CreditCardIcon, HandCoinsIcon, MessagesSquareIcon, type LucideIcon } from "lucide-react"
import { Skeleton } from "@/components/ui/skeleton"
import { formatMoney, formatNumber } from "@/lib/money"
import { useSession } from "@/lib/auth"
import { useDigits, useFormat, useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { useDashboardStats } from "../hooks/use-analytics"

function greetingKey(hour: number): "morning" | "afternoon" | "evening" {
  if (hour < 12) return "morning"
  if (hour < 17) return "afternoon"
  return "evening"
}

function FocusTile({ href, icon: Icon, value, label }: { href: string; icon: LucideIcon; value: ReactNode; label: ReactNode }) {
  return (
    <Link
      href={href}
      className="group/tile flex min-w-0 items-center gap-3 rounded-2xl bg-white/10 px-3.5 py-3 ring-1 ring-white/15 backdrop-blur-sm transition-[background-color,transform] duration-(--dur-fast) ease-(--ease-smooth) outline-none hover:bg-white/16 focus-visible:ring-2 focus-visible:ring-white/70 active:scale-[0.98]"
    >
      <span className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-white/12 text-white">
        <Icon className="size-4.5" aria-hidden />
      </span>
      <span className="min-w-0">
        <span className="tabular block font-heading text-lg leading-none font-bold text-white">{value}</span>
        <span className="mt-1 block truncate text-xs text-white/75">{label}</span>
      </span>
    </Link>
  )
}

/**
 * Dashboard hero (ADR-009): greeting, how many things need attention today, and deep-link tiles
 * for each. The one place on the screen that is allowed to be loud; everything below stays quiet.
 */
export function TodayFocus({ action, className }: { action?: ReactNode; className?: string }) {
  const t = useT()
  const f = useFormat()
  const digits = useDigits()
  const { user } = useSession()
  const [{ data, fetching }] = useDashboardStats()
  const s = data?.dashboardStats
  const now = new Date()
  const firstName = user?.name.trim().split(/\s+/)[0]

  const tiles = s
    ? [
        s.openConversations > 0 && { key: "inbox", href: "/inbox", icon: MessagesSquareIcon, value: formatNumber(s.openConversations, digits), label: t.dashboard.openConversations },
        s.expiringIn7Days > 0 && { key: "expiring", href: "/subscriptions", icon: CalendarClockIcon, value: formatNumber(s.expiringIn7Days, digits), label: t.dashboard.expiringIn7Days },
        s.pendingPayments > 0 && { key: "payments", href: "/payments", icon: CreditCardIcon, value: formatNumber(s.pendingPayments, digits), label: t.dashboard.pendingPayments },
        s.totalDue > 0 && { key: "due", href: "/customers", icon: HandCoinsIcon, value: formatMoney(s.totalDue, { currency: s.currency, digits, compact: true }), label: t.dashboard.focusDue },
      ].filter((x): x is Exclude<typeof x, false> => Boolean(x))
    : []

  return (
    <section className={cn("hero-surface relative overflow-hidden rounded-3xl p-5 text-white shadow-md md:p-7", className)} aria-label={t.dashboard.title}>
      {/* river line: a quiet wave along the bottom edge */}
      <svg aria-hidden className="pointer-events-none absolute inset-x-0 bottom-0 h-16 w-full text-white/8" viewBox="0 0 800 64" preserveAspectRatio="none">
        <path d="M0 40 C 120 10, 220 70, 360 40 S 600 10, 800 40 L 800 64 L 0 64 Z" fill="currentColor" />
      </svg>
      <div className="relative flex flex-col gap-5 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0">
          <p className="text-sm text-white/75">
            {t.dashboard.greeting[greetingKey(now.getHours())]}
            {firstName ? `, ${firstName}` : ""}
            <span className="text-white/45"> · {f.date(now)}</span>
          </p>
          {s ? (
            <h1 className="mt-1.5 font-heading text-2xl leading-tight font-bold tracking-tight md:text-[1.9rem]">
              {tiles.length > 0 ? t.dashboard.focusHeadline(tiles.length) : t.dashboard.focusAllClear}
            </h1>
          ) : (
            <Skeleton className="mt-2 h-8 w-64 bg-white/15 before:via-white/20" />
          )}
        </div>
        {action ? <div className="shrink-0 md:pt-1">{action}</div> : null}
      </div>
      <div className="relative mt-5">
        {s ? (
          tiles.length > 0 ? (
            <div className="stagger grid grid-cols-2 gap-2.5 md:flex md:flex-wrap">
              {tiles.map((tile) => (
                <FocusTile key={tile.key} href={tile.href} icon={tile.icon} value={tile.value} label={tile.label} />
              ))}
            </div>
          ) : (
            <p className="flex items-center gap-2 text-sm text-white/80">
              <CheckCircle2Icon className="size-4.5 text-hero-glow" aria-hidden /> {t.dashboard.subtitle}
            </p>
          )
        ) : fetching ? (
          <div className="grid grid-cols-2 gap-2.5 md:flex">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-16 rounded-2xl bg-white/12 before:via-white/20 md:w-48" />
            ))}
          </div>
        ) : null}
      </div>
    </section>
  )
}
