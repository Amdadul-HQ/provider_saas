"use client"

import type { ReactNode } from "react"
import { BellRingIcon, MessagesSquareIcon, ReceiptTextIcon } from "lucide-react"
import { BrandMark, LanguageToggle } from "@/components/shared"
import { GuestGuard } from "@/lib/auth"
import { useT } from "@/i18n"

/** Split layout: brand story on the hero surface (desktop), compact header on phones. */
export function AuthFrame({ children }: { children: ReactNode }) {
  const t = useT()
  const points = [
    { icon: MessagesSquareIcon, text: t.inbox.subtitle },
    { icon: ReceiptTextIcon, text: t.billing.paymentsSubtitle },
    { icon: BellRingIcon, text: t.reminders.subtitle },
  ]
  return (
    <GuestGuard>
      <div className="grid min-h-dvh lg:grid-cols-[1.1fr_1fr]">
        <aside className="hero-surface relative hidden flex-col justify-between overflow-hidden p-10 text-white lg:flex">
          <div className="flex items-center gap-2.5">
            <BrandMark />
            <span className="font-heading text-xl font-bold tracking-tight">{t.meta.appName}</span>
          </div>
          <div className="stagger relative space-y-8">
            <h2 className="max-w-md font-heading text-[2.2rem] leading-[1.25] font-bold tracking-tight">{t.meta.tagline}</h2>
            <ul className="space-y-3">
              {points.map(({ icon: Icon, text }) => (
                <li key={text} className="flex items-center gap-3 text-white/85">
                  <span className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-white/10 ring-1 ring-white/15">
                    <Icon className="size-4.5 text-hero-glow" aria-hidden />
                  </span>
                  {text}
                </li>
              ))}
            </ul>
          </div>
          <svg aria-hidden className="pointer-events-none absolute inset-x-0 bottom-0 h-40 w-full text-white/6" viewBox="0 0 800 160" preserveAspectRatio="none">
            <path d="M0 90 C 140 30, 260 150, 400 90 S 660 30, 800 90 L 800 160 L 0 160 Z" fill="currentColor" />
            <path d="M0 120 C 160 70, 280 170, 420 120 S 680 70, 800 120 L 800 160 L 0 160 Z" fill="currentColor" />
          </svg>
        </aside>
        <main className="flex flex-col">
          <div className="flex items-center justify-between p-4">
            <span className="flex items-center gap-2 lg:invisible">
              <BrandMark className="size-8 rounded-lg text-base" />
              <span className="font-heading font-bold tracking-tight">{t.meta.appName}</span>
            </span>
            <LanguageToggle />
          </div>
          <div className="flex flex-1 items-center justify-center px-4 pb-10">
            <div className="page-enter w-full max-w-sm">{children}</div>
          </div>
        </main>
      </div>
    </GuestGuard>
  )
}
