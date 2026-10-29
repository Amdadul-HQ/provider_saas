"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { useState } from "react"
import { LayoutGridIcon } from "lucide-react"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { MOBILE_PRIMARY, NAV_ITEMS, isActive } from "./nav-items"
import type { NavBadges } from "./sidebar"

/**
 * Floating dock for phones (ADR-009): 5 slots — Dashboard, Inbox, Customers, Subscriptions, More.
 * It floats `--dock-gap` above the safe area; `pb-safe-nav` on the page keeps content clear of it.
 */
export function MobileNav({ badges }: { badges?: NavBadges }) {
  const t = useT()
  const pathname = usePathname()
  const [moreOpen, setMoreOpen] = useState(false)
  const primary = NAV_ITEMS.filter((i) => MOBILE_PRIMARY.includes(i.key))
  const rest = NAV_ITEMS.filter((i) => !MOBILE_PRIMARY.includes(i.key))
  const moreActive = rest.some((i) => isActive(pathname, i.href))

  const slot = (active: boolean) =>
    cn(
      "relative flex h-full w-full flex-col items-center justify-center gap-0.5 text-[11px] font-medium transition-colors duration-(--dur-fast)",
      active ? "text-primary" : "text-muted-foreground active:text-foreground",
    )
  const pill = (active: boolean) =>
    cn(
      "flex h-7 w-12 items-center justify-center rounded-full transition-[background-color,transform] duration-(--dur) ease-(--ease-spring)",
      active ? "scale-100 bg-primary/12" : "scale-90 bg-transparent",
    )

  return (
    <>
      <nav
        aria-label={t.shell.menu}
        className="fixed inset-x-3 bottom-[calc(env(safe-area-inset-bottom)+var(--dock-gap))] z-40 rounded-[1.6rem] border border-border/70 bg-card/88 shadow-float backdrop-blur-xl supports-backdrop-filter:bg-card/80 md:hidden"
      >
        <ul className="grid h-(--dock-height) grid-cols-5 px-1">
          {primary.map((item) => {
            const active = isActive(pathname, item.href)
            const badge = badges?.[item.key]
            return (
              <li key={item.key}>
                <Link href={item.href} aria-current={active ? "page" : undefined} className={slot(active)}>
                  <span className={pill(active)}>
                    <item.icon className="size-5" aria-hidden />
                  </span>
                  <span className="max-w-full truncate px-1">{t.nav[item.key]}</span>
                  {badge ? (
                    <span className="tabular absolute top-1.5 left-1/2 ml-2 min-w-4 rounded-full bg-warning px-1 text-center text-[10px] leading-4 font-bold text-warning-foreground ring-2 ring-card">
                      {badge > 99 ? "99+" : badge}
                    </span>
                  ) : null}
                </Link>
              </li>
            )
          })}
          <li>
            <button type="button" onClick={() => setMoreOpen(true)} className={slot(moreActive)} aria-haspopup="dialog" aria-expanded={moreOpen}>
              <span className={pill(moreActive)}>
                <LayoutGridIcon className="size-5" aria-hidden />
              </span>
              {t.nav.more}
            </button>
          </li>
        </ul>
      </nav>
      <Sheet open={moreOpen} onOpenChange={setMoreOpen}>
        <SheetContent side="bottom" className="rounded-t-3xl pb-[max(1rem,env(safe-area-inset-bottom))]">
          <SheetHeader>
            <SheetTitle>{t.nav.more}</SheetTitle>
          </SheetHeader>
          <ul className="stagger grid grid-cols-3 gap-2 px-4">
            {rest.map((item) => {
              const active = isActive(pathname, item.href)
              const badge = badges?.[item.key]
              return (
                <li key={item.key}>
                  <Link
                    href={item.href}
                    onClick={() => setMoreOpen(false)}
                    className={cn(
                      "relative flex h-20 flex-col items-center justify-center gap-1.5 rounded-2xl border text-xs font-medium transition-[background-color,transform] duration-(--dur-fast) active:scale-[0.97]",
                      active ? "border-primary/30 bg-primary/8 text-primary" : "bg-card shadow-xs",
                    )}
                  >
                    <item.icon className={cn("size-5", !active && "text-muted-foreground")} aria-hidden />
                    {t.nav[item.key]}
                    {badge ? (
                      <span className="tabular absolute top-2 right-2 rounded-full bg-warning px-1.5 text-[10px] font-bold text-warning-foreground">{badge}</span>
                    ) : null}
                  </Link>
                </li>
              )
            })}
          </ul>
        </SheetContent>
      </Sheet>
    </>
  )
}
