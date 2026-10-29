"use client"

import type { ReactNode } from "react"
import { SiteHeader } from "./site-header"
import { GuestGuard } from "@/lib/auth"
import { AuthSwitch, SampleInbox, SampleToday } from "@/modules/auth"
import { useT } from "@/i18n"

function RiverLines({ className }: { className?: string }) {
  return (
    <svg aria-hidden className={className} viewBox="0 0 800 160" preserveAspectRatio="none">
      <path d="M0 90 C 140 30, 260 150, 400 90 S 660 30, 800 90 L 800 160 L 0 160 Z" fill="currentColor" />
      <path d="M0 120 C 160 70, 280 170, 420 120 S 680 70, 800 120 L 800 160 L 0 160 Z" fill="currentColor" />
    </svg>
  )
}

/**
 * Auth layout (ADR-009). Desktop: hero panel with the sample inbox, form on the right.
 * Phones: a compact hero strip with the form card overlapping it, so the first screen still
 * feels like the product and the form sits within thumb reach.
 */
export function AuthFrame({ children }: { children: ReactNode }) {
  const t = useT()
  return (
    <GuestGuard>
      <SiteHeader variant="solid" />
      <div className="grid min-h-[calc(100dvh-4rem)] grid-cols-1 lg:grid-cols-2">
        {/* content is centred vertically so tall screens never show an empty band (user feedback 2026-10-29) */}
        <aside className="hero-surface relative hidden flex-col overflow-hidden px-10 py-6 text-white lg:flex xl:px-14">
          {/* centred in its half and a little larger on wide screens, so hero and form balance (user feedback 2026-10-29) */}
          <div className="relative z-10 mx-auto my-auto w-full max-w-md space-y-6 py-4 2xl:max-w-lg">
            <h2 className="font-heading text-[2.1rem] leading-[1.25] font-bold tracking-tight 2xl:text-[2.5rem] [@media(max-height:700px)]:text-[1.7rem]">{t.meta.tagline}</h2>
            <SampleToday className="[@media(max-height:880px)]:hidden" />
            <SampleInbox />
          </div>
          <RiverLines className="pointer-events-none absolute inset-x-0 bottom-0 h-40 w-full text-white/6" />
        </aside>

        <main className="flex flex-col">
          {/* phones: hero strip */}
          <div className="hero-surface relative overflow-hidden px-4 pt-2 pb-14 text-white lg:hidden">
            {/* same width as the form card below, so text, feed and card share one column on tablets */}
            <div className="mx-auto w-full max-w-md">
              <p className="mt-4 max-w-xs font-heading text-xl leading-snug font-bold sm:max-w-sm sm:text-2xl">{t.meta.tagline}</p>
              {/* tall tablets have room for the same "today" card as desktop */}
              <SampleToday className="mt-5 hidden sm:block [@media(max-height:960px)]:hidden" />
              <SampleInbox compact className="mt-4 max-w-none [@media(max-height:700px)]:hidden" />
            </div>
            <RiverLines className="pointer-events-none absolute inset-x-0 bottom-0 h-16 w-full text-white/8" />
          </div>

          <div className="relative -mt-8 flex flex-1 items-start justify-center px-4 pb-10 sm:px-8 lg:mt-0 lg:items-center lg:py-10">
            <div className="page-enter w-full max-w-md min-w-0 rounded-3xl bg-card p-5 shadow-md ring-1 ring-border/60 sm:p-7 lg:bg-transparent lg:p-0 lg:shadow-none lg:ring-0 2xl:max-w-lg">
              <AuthSwitch className="mb-6" />
              {children}
            </div>
          </div>
        </main>
      </div>
    </GuestGuard>
  )
}
