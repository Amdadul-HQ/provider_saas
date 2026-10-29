"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { useState } from "react"
import { MenuIcon } from "lucide-react"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { MOBILE_PRIMARY, NAV_ITEMS, isActive } from "./nav-items"
import type { NavBadges } from "./sidebar"

/** Bottom tab bar (phones). 5 slots: Dashboard, Inbox, Customers, Subscriptions, More. */
export function MobileNav({ badges }: { badges?: NavBadges }) {
  const t = useT()
  const pathname = usePathname()
  const [moreOpen, setMoreOpen] = useState(false)
  const primary = NAV_ITEMS.filter((i) => MOBILE_PRIMARY.includes(i.key))
  const rest = NAV_ITEMS.filter((i) => !MOBILE_PRIMARY.includes(i.key))
  const moreActive = rest.some((i) => isActive(pathname, i.href))

  return (
    <>
      <nav
        aria-label={t.shell.menu}
        className="fixed inset-x-0 bottom-0 z-40 border-t bg-card/95 pb-[env(safe-area-inset-bottom)] backdrop-blur md:hidden"
      >
        <ul className="grid h-(--bottom-nav-height) grid-cols-5">
          {primary.map((item) => {
            const active = isActive(pathname, item.href)
            const badge = badges?.[item.key]
            return (
              <li key={item.key}>
                <Link
                  href={item.href}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "relative flex h-full flex-col items-center justify-center gap-0.5 text-[11px] font-medium",
                    active ? "text-primary" : "text-muted-foreground",
                  )}
                >
                  <span className={cn("flex h-7 w-12 items-center justify-center rounded-full transition-colors", active && "bg-primary/10")}>
                    <item.icon className="size-5" aria-hidden />
                  </span>
                  <span className="max-w-full truncate px-1">{t.nav[item.key]}</span>
                  {badge ? (
                    <span className="absolute top-1.5 left-1/2 ml-2 min-w-4 rounded-full bg-warning px-1 text-center text-[10px] leading-4 font-bold text-warning-foreground tabular">
                      {badge > 99 ? "99+" : badge}
                    </span>
                  ) : null}
                </Link>
              </li>
            )
          })}
          <li>
            <button
              type="button"
              onClick={() => setMoreOpen(true)}
              className={cn(
                "flex h-full w-full flex-col items-center justify-center gap-0.5 text-[11px] font-medium",
                moreActive ? "text-primary" : "text-muted-foreground",
              )}
            >
              <span className={cn("flex h-7 w-12 items-center justify-center rounded-full", moreActive && "bg-primary/10")}>
                <MenuIcon className="size-5" aria-hidden />
              </span>
              {t.nav.more}
            </button>
          </li>
        </ul>
      </nav>
      <Sheet open={moreOpen} onOpenChange={setMoreOpen}>
        <SheetContent side="bottom" className="rounded-t-2xl pb-[max(1rem,env(safe-area-inset-bottom))]">
          <SheetHeader>
            <SheetTitle>{t.nav.more}</SheetTitle>
          </SheetHeader>
          <ul className="grid grid-cols-3 gap-2 px-4">
            {rest.map((item) => {
              const active = isActive(pathname, item.href)
              const badge = badges?.[item.key]
              return (
                <li key={item.key}>
                  <Link
                    href={item.href}
                    onClick={() => setMoreOpen(false)}
                    className={cn(
                      "relative flex h-20 flex-col items-center justify-center gap-1.5 rounded-xl border text-xs font-medium",
                      active ? "border-primary/40 bg-primary/5 text-primary" : "bg-card",
                    )}
                  >
                    <item.icon className="size-5" aria-hidden />
                    {t.nav[item.key]}
                    {badge ? (
                      <span className="absolute top-2 right-2 rounded-full bg-warning px-1.5 text-[10px] font-bold text-warning-foreground">{badge}</span>
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
