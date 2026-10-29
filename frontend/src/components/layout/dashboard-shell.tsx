"use client"

import type { ReactNode } from "react"
import { LanguageToggle, MobileNav, Sidebar } from "@/components/shared"
import { AuthGuard, useSession } from "@/lib/auth"
import { UserMenu } from "@/modules/auth"
import { NotificationBell } from "@/modules/tenant"
import { useInboxCounts } from "@/modules/inbox"
import { useT } from "@/i18n"

function Shell({ children }: { children: ReactNode }) {
  const t = useT()
  const { tenant } = useSession()
  const [{ data: counts }] = useInboxCounts()
  const badges = { inbox: counts?.inboxCounts.unread ?? 0 }
  return (
    <div className="flex min-h-dvh">
      <Sidebar badges={badges} footer={<UserMenu variant="sidebar" />} />
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-14 items-center justify-between gap-2 border-b bg-background/90 px-4 backdrop-blur pt-[env(safe-area-inset-top)] md:h-16 md:px-6">
          <div className="flex min-w-0 items-center gap-2 md:hidden">
            <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary font-bold text-primary-foreground">৳</span>
            <span className="truncate font-heading font-semibold">{tenant?.name ?? t.meta.appName}</span>
          </div>
          <p className="hidden truncate text-sm text-muted-foreground md:block">{tenant?.name}</p>
          <div className="flex items-center gap-1">
            <LanguageToggle />
            <NotificationBell />
            <div className="md:hidden">
              <UserMenu />
            </div>
          </div>
        </header>
        <main className="pb-safe-nav flex-1 px-4 pt-4 md:px-6 md:pt-6 md:pb-10">{children}</main>
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
