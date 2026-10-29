"use client"

import Link from "next/link"
import {
  BanknoteIcon,
  BellRingIcon,
  BoxesIcon,
  CheckIcon,
  ChevronDownIcon,
  HandCoinsIcon,
  LifeBuoyIcon,
  MessageCircleIcon,
  MessagesSquareIcon,
  PackageCheckIcon,
  UserPlusIcon,
  UsersIcon,
  type LucideIcon,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { BrandMark, PlatformIcon } from "@/components/shared"
import type { Platform } from "@/gql/graphql"
import { SampleInbox, SampleToday } from "@/modules/auth"
import { useT } from "@/i18n"
import { SiteHeader } from "@/components/layout/site-header"
import { Pricing } from "./pricing"

const CHANNELS: Platform[] = ["FACEBOOK", "INSTAGRAM", "WHATSAPP", "TELEGRAM"]
const FLOW_ICONS: LucideIcon[] = [MessageCircleIcon, UserPlusIcon, BanknoteIcon, PackageCheckIcon, BellRingIcon]
const GROUP_ICONS: LucideIcon[] = [MessagesSquareIcon, HandCoinsIcon, BoxesIcon, LifeBuoyIcon, UsersIcon]

function RiverLines({ className }: { className?: string }) {
  return (
    <svg aria-hidden className={className} viewBox="0 0 800 160" preserveAspectRatio="none">
      <path d="M0 90 C 140 30, 260 150, 400 90 S 660 30, 800 90 L 800 160 L 0 160 Z" fill="currentColor" />
      <path d="M0 120 C 160 70, 280 170, 420 120 S 680 70, 800 120 L 800 160 L 0 160 Z" fill="currentColor" />
    </svg>
  )
}

function Hero() {
  const t = useT()
  const h = t.landing.hero
  return (
    <section className="hero-surface relative overflow-hidden px-4 pt-28 pb-20 text-white md:px-6 md:pt-36 md:pb-28">
      <div className="relative mx-auto grid max-w-6xl grid-cols-1 items-center gap-12 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]">
        <div className="page-enter min-w-0">
          <span className="inline-flex items-center gap-2 rounded-full bg-white/10 px-3 py-1.5 text-sm font-medium ring-1 ring-white/15">
            <span className="size-2 rounded-full bg-hero-glow" aria-hidden />
            {h.badge}
          </span>
          <h1 className="mt-5 max-w-xl font-heading text-[2rem] leading-[1.2] font-bold tracking-tight md:text-5xl md:leading-[1.15]">{h.title}</h1>
          <p className="mt-5 max-w-lg text-base text-white/80 md:text-lg">{h.body}</p>
          <div className="mt-8 flex flex-col gap-3 sm:flex-row">
            <Button asChild size="lg" className="h-12 rounded-xl bg-white px-6 text-base text-hero-to shadow-md hover:bg-white/90">
              <Link href="/register">{h.cta}</Link>
            </Button>
            <Button asChild size="lg" variant="ghost" className="h-12 rounded-xl px-6 text-base text-white ring-1 ring-white/25 hover:bg-white/10 hover:text-white">
              <a href="#pricing">{h.secondary}</a>
            </Button>
          </div>
          <ul className="mt-6 flex flex-wrap gap-x-5 gap-y-2 text-sm text-white/75">
            {[h.noCard, h.setup, h.cancel].map((x) => (
              <li key={x} className="flex items-center gap-1.5">
                <CheckIcon className="size-4 text-hero-glow" strokeWidth={3} aria-hidden /> {x}
              </li>
            ))}
          </ul>
          <div className="mt-8 flex items-center gap-2" aria-hidden>
            {CHANNELS.map((p) => (
              <PlatformIcon key={p} platform={p} className="size-8 ring-2 ring-white/20 [&_svg]:size-4" />
            ))}
          </div>
        </div>
        <div className="mx-auto w-full max-w-md min-w-0 space-y-5 lg:mr-0 lg:ml-auto xl:max-w-lg">
          <SampleToday />
          <SampleInbox />
        </div>
      </div>
      <RiverLines className="pointer-events-none absolute inset-x-0 bottom-0 h-28 w-full text-background" />
    </section>
  )
}

/** A real sequence (message → renewal), so numbered steps are honest here. */
function Flow() {
  const t = useT()
  const f = t.landing.flow
  return (
    <section className="px-4 py-14 md:px-6 md:py-20">
      <div className="mx-auto max-w-6xl">
        <h2 className="max-w-xl font-heading text-[1.6rem] leading-tight font-bold tracking-tight md:text-3xl">{f.title}</h2>
        <ol className="mt-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
          {f.steps.map((step, i) => {
            const Icon = FLOW_ICONS[i]
            return (
              <li key={step.title} className="relative rounded-2xl border border-border/80 bg-card p-4 shadow-xs">
                <div className="flex items-center justify-between">
                  <span className="flex size-10 items-center justify-center rounded-xl bg-secondary text-primary">
                    <Icon className="size-5" aria-hidden />
                  </span>
                  <span className="tabular font-heading text-sm font-bold text-muted-foreground/60">{i + 1}</span>
                </div>
                <p className="mt-4 font-semibold">{step.title}</p>
                <p className="mt-1 text-sm text-muted-foreground">{step.body}</p>
              </li>
            )
          })}
        </ol>
      </div>
    </section>
  )
}

/** Features grouped by the job a seller does; one panel with columns, not a wall of cards. */
function Features() {
  const t = useT()
  const f = t.landing.features
  return (
    <section id="features" className="scroll-mt-20 px-4 py-14 md:px-6 md:py-20">
      <div className="mx-auto max-w-6xl">
        <div className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
          <h2 className="max-w-xl font-heading text-[1.6rem] leading-tight font-bold tracking-tight md:text-3xl">{f.title}</h2>
          <p className="text-muted-foreground">{f.subtitle}</p>
        </div>
        <div className="mt-8 grid overflow-hidden rounded-3xl border border-border/80 bg-card shadow-xs sm:grid-cols-2 lg:grid-cols-5">
          {f.groups.map((g, i) => {
            const Icon = GROUP_ICONS[i]
            return (
              <div key={g.title} className="border-border/70 p-5 not-last:border-b sm:[&:nth-child(odd)]:border-r lg:border-r lg:border-b-0 lg:last:border-r-0 lg:[&:nth-child(odd)]:border-r">
                <div className="flex items-center gap-2.5">
                  <span className="flex size-9 items-center justify-center rounded-xl bg-primary text-primary-foreground">
                    <Icon className="size-4.5" aria-hidden />
                  </span>
                  <h3 className="font-heading font-bold">{g.title}</h3>
                </div>
                <ul className="mt-4 space-y-2.5 text-sm">
                  {g.items.map((item) => (
                    <li key={item} className="flex items-start gap-2">
                      <CheckIcon className="mt-0.5 size-4 shrink-0 text-success" strokeWidth={2.5} aria-hidden />
                      {item}
                    </li>
                  ))}
                </ul>
              </div>
            )
          })}
        </div>
      </div>
    </section>
  )
}

function Faq() {
  const t = useT()
  const f = t.landing.faq
  return (
    <section id="faq" className="scroll-mt-20 px-4 py-14 md:px-6 md:py-20">
      <div className="mx-auto grid max-w-6xl grid-cols-1 gap-8 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
        <h2 className="font-heading text-[1.6rem] leading-tight font-bold tracking-tight md:text-3xl">{f.title}</h2>
        <div className="divide-y divide-border/70 rounded-3xl border border-border/80 bg-card shadow-xs">
          {f.items.map((item, i) => (
            <details key={item.q} className="group px-5 [&_summary::-webkit-details-marker]:hidden" open={i === 0}>
              <summary className="flex cursor-pointer list-none items-center justify-between gap-4 py-4 font-semibold outline-none focus-visible:text-primary">
                {item.q}
                <ChevronDownIcon className="size-5 shrink-0 text-muted-foreground transition-transform duration-(--dur) group-open:rotate-180" aria-hidden />
              </summary>
              <p className="animate-in fade-in-0 slide-in-from-top-1 pb-5 text-sm leading-relaxed text-muted-foreground duration-300">{item.a}</p>
            </details>
          ))}
        </div>
      </div>
    </section>
  )
}

function FinalCta() {
  const t = useT()
  const c = t.landing.finalCta
  return (
    <section className="px-4 pb-16 md:px-6 md:pb-24">
      <div className="hero-surface relative mx-auto max-w-6xl overflow-hidden rounded-3xl px-6 py-12 text-center text-white shadow-md md:py-16">
        <h2 className="relative mx-auto max-w-2xl font-heading text-[1.6rem] leading-tight font-bold tracking-tight md:text-4xl">{c.title}</h2>
        <p className="relative mx-auto mt-3 max-w-xl text-white/80">{c.body}</p>
        <Button asChild size="lg" className="relative mt-8 h-12 rounded-xl bg-white px-7 text-base text-hero-to shadow-md hover:bg-white/90">
          <Link href="/register">{c.cta}</Link>
        </Button>
        <RiverLines className="pointer-events-none absolute inset-x-0 bottom-0 h-24 w-full text-white/6" />
      </div>
    </section>
  )
}

function Footer() {
  const t = useT()
  return (
    <footer className="border-t border-border/70 px-4 py-8 md:px-6">
      <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-3 text-sm text-muted-foreground md:flex-row">
        <span className="flex items-center gap-2">
          <BrandMark className="size-7 rounded-lg text-sm" />
          <span className="font-heading font-bold text-foreground">{t.meta.appName}</span>
        </span>
        <span>{t.landing.footer.madeFor}</span>
        <span className="tabular">{t.landing.footer.rights(new Date().getFullYear())}</span>
      </div>
    </footer>
  )
}

/** Public marketing page at `/` (REQ-006). One bold element: the live notification stack in the hero. */
export function LandingPage() {
  return (
    <div className="min-h-dvh bg-background">
      <SiteHeader />
      <main>
        <Hero />
        <Flow />
        <Features />
        <Pricing />
        <Faq />
        <FinalCta />
      </main>
      <Footer />
    </div>
  )
}
