"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import type { ReactNode } from "react"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { NAV_ITEMS, isActive, type NavKey } from "./nav-items"

export type NavBadges = Partial<Record<NavKey, number>>

/** Brand mark: the taka sign on an amber tile. Shared by sidebar, topbar and auth frame. */
export function BrandMark({ className }: { className?: string }) {
  return (
    <span
      className={cn(
        "flex size-9 shrink-0 items-center justify-center rounded-xl bg-sidebar-primary font-heading text-lg font-bold text-sidebar-primary-foreground shadow-[inset_0_-2px_0_oklch(0_0_0/0.12)]",
        className,
      )}
      aria-hidden
    >
      ৳
    </span>
  )
}

/** Desktop rail (ADR-009): grouped links, amber indicator on the active item, tenant menu in the footer. */
export function Sidebar({ badges, header, footer }: { badges?: NavBadges; header?: ReactNode; footer?: ReactNode }) {
  const t = useT()
  const pathname = usePathname()
  const sections = Array.from(new Set(NAV_ITEMS.map((i) => i.section)))
  return (
    <aside className="sticky top-0 hidden h-dvh w-64 shrink-0 flex-col bg-sidebar text-sidebar-foreground md:flex">
      <div className="flex h-16 items-center gap-2.5 px-5">
        <BrandMark />
        <span className="font-heading text-lg font-bold tracking-tight">{t.meta.appName}</span>
      </div>
      {header ? <div className="px-3 pb-2">{header}</div> : null}
      <nav className="scroll-thin flex-1 space-y-5 overflow-y-auto px-3 py-2" aria-label={t.shell.menu}>
        {sections.map((section) => (
          <div key={section}>
            <p className="px-3 pb-1.5 text-[11px] font-medium text-sidebar-foreground/45">{t.nav.sections[section]}</p>
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
                        "group/nav relative flex h-10 items-center gap-3 rounded-xl px-3 text-sm transition-[background-color,color,transform] duration-(--dur-fast) ease-(--ease-smooth)",
                        // amber indicator grows from the middle of the left edge
                        "before:absolute before:top-1/2 before:left-0 before:h-5 before:w-[3px] before:-translate-y-1/2 before:scale-y-0 before:rounded-r-full before:bg-sidebar-primary before:transition-transform before:duration-(--dur) before:ease-(--ease-spring)",
                        active
                          ? "bg-sidebar-accent font-semibold text-sidebar-accent-foreground before:scale-y-100"
                          : "text-sidebar-foreground/75 hover:bg-sidebar-accent/55 hover:text-sidebar-accent-foreground active:scale-[0.99]",
                      )}
                    >
                      <item.icon
                        className={cn("size-4.5 transition-colors duration-(--dur-fast)", active ? "text-sidebar-primary" : "group-hover/nav:text-sidebar-foreground")}
                        aria-hidden
                      />
                      <span className="flex-1 truncate">{t.nav[item.key]}</span>
                      {badge ? (
                        <span className="tabular min-w-5 rounded-full bg-sidebar-primary px-1.5 py-0.5 text-center text-[11px] leading-4 font-bold text-sidebar-primary-foreground">
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
