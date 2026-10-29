"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import type { ReactNode } from "react"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { NAV_ITEMS, isActive, type NavKey } from "./nav-items"

export type NavBadges = Partial<Record<NavKey, number>>

export function Sidebar({ badges, header, footer }: { badges?: NavBadges; header?: ReactNode; footer?: ReactNode }) {
  const t = useT()
  const pathname = usePathname()
  const sections = Array.from(new Set(NAV_ITEMS.map((i) => i.section)))
  return (
    <aside className="sticky top-0 hidden h-dvh w-64 shrink-0 flex-col bg-sidebar text-sidebar-foreground md:flex">
      <div className="flex h-16 items-center gap-2 px-5">
        <span className="flex size-8 items-center justify-center rounded-lg bg-sidebar-primary font-bold text-sidebar-primary-foreground">৳</span>
        <span className="font-heading text-lg font-semibold tracking-tight">{t.meta.appName}</span>
      </div>
      {header ? <div className="px-3 pb-2">{header}</div> : null}
      <nav className="flex-1 space-y-4 overflow-y-auto px-3 py-2" aria-label={t.shell.menu}>
        {sections.map((section) => (
          <div key={section}>
            <p className="px-2 pb-1 text-[11px] font-semibold tracking-wider text-sidebar-foreground/50 uppercase">
              {t.nav.sections[section]}
            </p>
            <ul className="space-y-0.5">
              {NAV_ITEMS.filter((i) => i.section === section).map((item) => {
                const active = isActive(pathname, item.href)
                const badge = badges?.[item.key]
                return (
                  <li key={item.key}>
                    <Link
                      href={item.href}
                      aria-current={active ? "page" : undefined}
                      className={cn(
                        "flex h-10 items-center gap-3 rounded-lg px-2.5 text-sm transition-colors",
                        active
                          ? "bg-sidebar-accent font-medium text-sidebar-accent-foreground"
                          : "text-sidebar-foreground/80 hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground",
                      )}
                    >
                      <item.icon className={cn("size-4.5", active && "text-sidebar-primary")} aria-hidden />
                      <span className="flex-1">{t.nav[item.key]}</span>
                      {badge ? (
                        <span className="rounded-full bg-sidebar-primary px-1.5 py-0.5 text-[11px] font-semibold text-sidebar-primary-foreground tabular">
                          {badge > 99 ? "99+" : badge}
                        </span>
                      ) : null}
                    </Link>
                  </li>
                )
              })}
            </ul>
          </div>
        ))}
      </nav>
      {footer ? <div className="border-t border-sidebar-border p-3">{footer}</div> : null}
    </aside>
  )
}
