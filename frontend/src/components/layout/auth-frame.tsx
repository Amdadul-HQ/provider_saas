"use client"

import type { ReactNode } from "react"
import { BellRingIcon, MessagesSquareIcon, ReceiptTextIcon } from "lucide-react"
import { LanguageToggle } from "@/components/shared"
import { GuestGuard } from "@/lib/auth"
import { useT } from "@/i18n"

/** Split layout: brand story on desktop, compact header on phones. */
export function AuthFrame({ children }: { children: ReactNode }) {
  const t = useT()
  return (
    <GuestGuard>
      <div className="grid min-h-dvh lg:grid-cols-[1.1fr_1fr]">
        <aside className="relative hidden flex-col justify-between overflow-hidden bg-sidebar p-10 text-sidebar-foreground lg:flex">
          <div className="flex items-center gap-2">
            <span className="flex size-9 items-center justify-center rounded-lg bg-sidebar-primary text-lg font-bold text-sidebar-primary-foreground">৳</span>
            <span className="font-heading text-xl font-semibold">{t.meta.appName}</span>
          </div>
          <div className="space-y-6">
            <h2 className="max-w-md font-heading text-3xl leading-snug font-semibold">{t.meta.tagline}</h2>
            <ul className="space-y-3 text-sidebar-foreground/85">
              <li className="flex items-center gap-3">
                <MessagesSquareIcon className="size-5 text-sidebar-primary" /> {t.inbox.subtitle}
              </li>
              <li className="flex items-center gap-3">
                <ReceiptTextIcon className="size-5 text-sidebar-primary" /> {t.billing.paymentsSubtitle}
              </li>
              <li className="flex items-center gap-3">
                <BellRingIcon className="size-5 text-sidebar-primary" /> {t.reminders.subtitle}
              </li>
            </ul>
          </div>
          <div aria-hidden className="pointer-events-none absolute -right-24 -bottom-24 size-96 rounded-full bg-sidebar-primary/10 blur-3xl" />
        </aside>
        <main className="flex flex-col">
          <div className="flex items-center justify-between p-4">
            <span className="flex items-center gap-2 lg:invisible">
              <span className="flex size-8 items-center justify-center rounded-lg bg-primary font-bold text-primary-foreground">৳</span>
              <span className="font-heading font-semibold">{t.meta.appName}</span>
            </span>
            <LanguageToggle />
          </div>
          <div className="flex flex-1 items-center justify-center px-4 pb-10">
            <div className="w-full max-w-sm">{children}</div>
          </div>
        </main>
      </div>
    </GuestGuard>
  )
}
