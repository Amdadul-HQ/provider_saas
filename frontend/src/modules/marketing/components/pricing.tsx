"use client"

import { useState } from "react"
import Link from "next/link"
import { CheckIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useFormat, useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { PLANS, planPrice, yearlySaving, type Cycle } from "../plans"

/** REQ-006 plans with a monthly/yearly switch; the price re-pops when the cycle changes. */
export function Pricing() {
  const t = useT()
  const f = useFormat()
  const [cycle, setCycle] = useState<Cycle>("monthly")
  const p = t.landing.pricing
  return (
    <section id="pricing" className="scroll-mt-20 px-4 py-16 md:px-6 md:py-24">
      <div className="mx-auto max-w-6xl">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="font-heading text-[1.75rem] leading-tight font-bold tracking-tight md:text-4xl">{p.title}</h2>
          <p className="mt-3 text-muted-foreground">{p.subtitle}</p>
        </div>

        <div role="radiogroup" aria-label={p.title} className="relative mx-auto mt-8 grid h-11 w-fit grid-cols-2 rounded-full bg-muted p-1">
          <span
            aria-hidden
            className={cn(
              "absolute inset-y-1 left-1 w-[calc(50%-0.25rem)] rounded-full bg-card shadow-sm transition-transform duration-(--dur) ease-(--ease-smooth)",
              cycle === "yearly" && "translate-x-full",
            )}
          />
          {(["monthly", "yearly"] as const).map((c) => (
            <button
              key={c}
              type="button"
              role="radio"
              aria-checked={cycle === c}
              onClick={() => setCycle(c)}
              className={cn(
                "relative z-10 flex items-center justify-center gap-1.5 rounded-full px-5 text-sm font-semibold transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
                cycle === c ? "text-foreground" : "text-muted-foreground hover:text-foreground",
              )}
            >
              {c === "monthly" ? p.monthly : p.yearly}
              {c === "yearly" ? <span className="rounded-full bg-success/12 px-1.5 text-[11px] text-success">{p.yearlyBonus}</span> : null}
            </button>
          ))}
        </div>

        <div className="mt-10 grid gap-4 lg:grid-cols-3 lg:items-stretch">
          {PLANS.map((plan) => {
            const copy = p.plans[plan.key]
            const dark = plan.suggested
            return (
              <article
                key={plan.key}
                className={cn(
                  "relative flex flex-col rounded-3xl p-6 md:p-7",
                  dark ? "hero-surface text-white shadow-md lg:-my-3 lg:py-10" : "border border-border/80 bg-card shadow-xs",
                )}
              >
                {dark ? (
                  <span className="absolute top-6 right-6 rounded-full bg-hero-glow px-2.5 py-1 text-xs font-bold text-sidebar-primary-foreground">{p.suggested}</span>
                ) : null}
                <h3 className="font-heading text-xl font-bold">{copy.name}</h3>
                <p className={cn("mt-1 text-sm", dark ? "text-white/75" : "text-muted-foreground")}>{copy.for}</p>
                <p className="mt-6 flex items-baseline gap-1">
                  <span key={cycle} className="animate-pop tabular font-heading text-4xl font-bold tracking-tight">
                    {f.money(planPrice(plan.monthly, cycle))}
                  </span>
                  <span className={cn("text-sm", dark ? "text-white/70" : "text-muted-foreground")}>{cycle === "monthly" ? p.perMonth : p.perYear}</span>
                </p>
                <p className={cn("mt-1 h-5 text-sm font-medium", dark ? "text-hero-glow" : "text-success")}>
                  {cycle === "yearly" ? p.savePerYear(f.money(yearlySaving(plan.monthly))) : null}
                </p>
                <ul className="mt-6 flex-1 space-y-2.5 text-sm">
                  {copy.features.map((feat) => (
                    <li key={feat} className="flex items-start gap-2.5">
                      <span className={cn("mt-0.5 flex size-4.5 shrink-0 items-center justify-center rounded-full", dark ? "bg-white/15 text-white" : "bg-success/12 text-success")}>
                        <CheckIcon className="size-3" strokeWidth={3} aria-hidden />
                      </span>
                      {feat}
                    </li>
                  ))}
                </ul>
                <Button
                  asChild
                  size="lg"
                  variant={dark ? "default" : "outline"}
                  className={cn("mt-8 h-12 rounded-xl text-base", dark && "bg-white text-hero-to hover:bg-white/90")}
                >
                  <Link href="/register">{p.cta}</Link>
                </Button>
                <p className={cn("mt-2 text-center text-xs", dark ? "text-white/65" : "text-muted-foreground")}>{p.trialNote}</p>
              </article>
            )
          })}
        </div>
      </div>
    </section>
  )
}
