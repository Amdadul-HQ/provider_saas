"use client"

import type { ReactNode } from "react"
import { usePathname } from "next/navigation"
import { BrandMark, LanguageToggle, MobileNav, Sidebar, ThemeToggle } from "@/components/shared"
import { AuthGuard, useSession } from "@/lib/auth"
import { UserMenu } from "@/modules/auth"
import { NotificationBell } from "@/modules/tenant"
import { useInboxCounts } from "@/modules/inbox"
import { useT } from "@/i18n"

function Shell({ children }: { children: ReactNode }) {
  const t = useT()
  const pathname = usePathname()
  const { tenant } = useSession()
  const [{ data: counts }] = useInboxCounts()
  const badges = { inbox: counts?.inboxCounts.unread ?? 0 }
  return (
    <div className="flex min-h-dvh">
      <Sidebar badges={badges} footer={<UserMenu variant="sidebar" />} />
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 border-b border-border/70 bg-background/85 pt-[env(safe-area-inset-top)] backdrop-blur-md supports-backdrop-filter:bg-background/70">
          <div className="mx-auto flex h-(--topbar-height) w-full max-w-(--content-max) items-center justify-between gap-2 px-4 md:h-16 md:px-8">
            <div className="flex min-w-0 items-center gap-2.5 md:hidden">
              <BrandMark className="size-8 rounded-lg text-base" />
              <span className="truncate font-heading font-bold tracking-tight">{tenant?.name ?? t.meta.appName}</span>
            </div>
            <p className="hidden truncate text-sm font-medium text-muted-foreground md:block">{tenant?.name}</p>
            <div className="flex items-center gap-1">
              <ThemeToggle />
              <LanguageToggle />
              <NotificationBell />
              <div className="md:hidden">
                <UserMenu />
              </div>
            </div>
          </div>
        </header>
        <main className="pb-safe-nav flex-1 md:pb-12">
          {/* one rise per route (ADR-009); search-param changes (inbox ?c=) keep the tree */}
          <div key={pathname} className="page-enter mx-auto w-full max-w-(--content-max) px-4 pt-4 md:px-8 md:pt-7">
            {children}
          </div>
        </main>
      </div>
      <MobileNav badges={badges} />
    </div>
  )
}

export function DashboardShell({ children }: { children: ReactNode }) {
  return (
    <AuthGuard>
      <Shell>{children}</Shell>
    </AuthGuard>
  )
}
