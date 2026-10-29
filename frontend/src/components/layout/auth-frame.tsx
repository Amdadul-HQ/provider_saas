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
 * Auth layout (ADR-009).
 * The navbar is the landing page's transparent overlay header, so there is always hero colour
 * under it: on desktop the hero gradient fills the whole page and the form sits in a card; on
 * phones/tablets the hero strip starts behind the header and the form card overlaps it.
 * Content uses the header's `max-w-6xl px-6` container, so text starts under the logo and the
 * card ends under the header's last button (user feedback 2026-10-29).
 */
export function AuthFrame({ children }: { children: ReactNode }) {
  const t = useT()
  return (
    <GuestGuard>
      <SiteHeader />
      <div className="relative">
        {/* desktop: full-page hero background */}
        <div aria-hidden className="hero-surface absolute inset-0 hidden overflow-hidden lg:block">
          <RiverLines className="pointer-events-none absolute inset-x-0 bottom-0 h-40 w-full text-white/6" />
        </div>

        <div className="relative mx-auto grid min-h-dvh max-w-6xl grid-cols-1 lg:grid-cols-2 lg:px-6 lg:pt-16">
          <aside className="hidden min-w-0 flex-col justify-center py-10 pr-12 text-white lg:flex xl:pr-20">
            <div className="w-full max-w-lg space-y-6">
              <h2 className="font-heading text-[2.1rem] leading-[1.25] font-bold tracking-tight xl:text-[2.4rem] [@media(max-height:700px)]:text-[1.7rem]">{t.meta.tagline}</h2>
              <SampleToday className="[@media(max-height:880px)]:hidden" />
              <SampleInbox />
            </div>
          </aside>

          <main className="flex min-w-0 flex-col">
            {/* phones/tablets: hero strip that starts behind the transparent header */}
            <div className="hero-surface relative overflow-hidden px-4 pt-[calc(4rem+env(safe-area-inset-top))] pb-14 text-white lg:hidden">
              {/* same width as the form card below, so text, feed and card share one column on tablets */}
              <div className="mx-auto w-full max-w-md">
                <p className="mt-2 max-w-xs font-heading text-xl leading-snug font-bold sm:max-w-sm sm:text-2xl">{t.meta.tagline}</p>
                {/* tall tablets have room for the same "today" card as desktop */}
                <SampleToday className="mt-5 hidden sm:block [@media(max-height:960px)]:hidden" />
                <SampleInbox compact className="mt-4 max-w-none [@media(max-height:700px)]:hidden" />
              </div>
              <RiverLines className="pointer-events-none absolute inset-x-0 bottom-0 h-16 w-full text-white/8" />
            </div>

            <div className="relative -mt-8 flex flex-1 items-start justify-center px-4 pb-10 sm:px-8 lg:mt-0 lg:items-center lg:justify-end lg:py-10 lg:pr-0 lg:pl-12 xl:pl-20">
              <div className="page-enter w-full max-w-md min-w-0 rounded-3xl bg-card p-5 text-card-foreground shadow-md ring-1 ring-border/60 sm:p-7 lg:p-8 dark:ring-white/10">
                <AuthSwitch className="mb-6" />
                {children}
              </div>
            </div>
          </main>
        </div>
      </div>
    </GuestGuard>
  )
}
