"use client"

import { TrendingUpIcon } from "lucide-react"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"

// relative heights of the last 7 days; today (last) is the tallest
const BARS = [38, 52, 44, 61, 57, 72, 90]

/**
 * Sample "today" card for the auth hero: what the dashboard tells a seller each morning.
 * Glass surface on the hero gradient; bars grow once on arrival. Labelled as a sample.
 */
export function SampleToday({ className }: { className?: string }) {
  const t = useT()
  const s = t.auth.sampleToday
  const stats = [
    { label: s.renewals, value: s.renewalsValue },
    { label: s.collected, value: s.collectedValue },
  ]
  return (
    <figure className={cn("w-full rounded-3xl bg-white/10 p-5 ring-1 ring-white/15 backdrop-blur-md", className)} aria-label={s.title}>
      <figcaption className="flex items-center justify-between gap-2 text-sm">
        <span className="font-semibold text-white">{s.title}</span>
        <span className="rounded-full bg-white/12 px-2 py-0.5 text-[11px] text-white/70">{s.sample}</span>
      </figcaption>
      <div className="mt-4 flex items-end justify-between gap-4">
        <div className="min-w-0">
          <p className="text-xs text-white/65">{s.sales}</p>
          <p className="tabular mt-1 flex items-center gap-2 font-heading text-3xl leading-none font-bold text-white">
            {s.salesValue}
            <TrendingUpIcon className="size-5 text-hero-glow" aria-hidden />
          </p>
        </div>
        <div className="flex h-14 items-end gap-1.5" role="img" aria-label={s.trend}>
          {BARS.map((h, i) => (
            <span
              key={i}
              className={cn(
                "w-2.5 origin-bottom animate-grow rounded-full",
                i === BARS.length - 1 ? "bg-hero-glow" : "bg-white/30",
              )}
              style={{ height: `${h}%`, animationDelay: `${150 + i * 60}ms` }}
            />
          ))}
        </div>
      </div>
      <dl className="mt-4 grid grid-cols-2 gap-2">
        {stats.map((x) => (
          <div key={x.label} className="rounded-2xl bg-white/8 px-3 py-2.5">
            <dt className="text-xs text-white/65">{x.label}</dt>
            <dd className="tabular mt-0.5 font-heading text-lg font-bold text-white">{x.value}</dd>
          </div>
        ))}
      </dl>
    </figure>
  )
}
