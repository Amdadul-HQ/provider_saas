"use client"

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { MoneyText } from "@/components/shared"
import type { ProductFieldsFragment } from "@/gql/graphql"
import { useT } from "@/i18n"
import { useProducts } from "../hooks/use-catalog"

/** Active-product select. Emits the whole product so callers can read its plans. */
export function ProductSelect({
  value,
  onChange,
  id,
  invalid,
  includeAllOption,
  "aria-label": ariaLabel,
}: {
  value: string | null | undefined
  onChange: (product: ProductFieldsFragment | null) => void
  id?: string
  invalid?: boolean
  includeAllOption?: string
  /** Accessible name when there is no <label htmlFor={id}> (e.g. in a filter toolbar). */
  "aria-label"?: string
}) {
  const t = useT()
  const [{ data, fetching }] = useProducts({ isActive: true }, { first: 100 })
  if (!data && fetching) return <Skeleton className="h-10 w-full" />
  const products = data?.products ?? []
  return (
    <Select
      value={value ?? (includeAllOption ? "__all__" : "")}
      onValueChange={(v) => onChange(products.find((p) => p.id === v) ?? null)}
    >
      <SelectTrigger id={id} className="w-full" aria-invalid={invalid} aria-label={ariaLabel ?? (id ? undefined : t.common.product)}>
        <SelectValue placeholder={t.catalog.pickProduct} />
      </SelectTrigger>
      <SelectContent>
        {includeAllOption ? <SelectItem value="__all__">{includeAllOption}</SelectItem> : null}
        {products.map((p) => (
          <SelectItem key={p.id} value={p.id}>
            {p.name}
            <span className="text-xs text-muted-foreground">({t.catalog.freeSlots(p.freeSlotCount)})</span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

/** Big tappable plan chips (≤3 taps on mobile). */
export function PlanPicker({
  plans,
  value,
  onChange,
}: {
  plans: ProductFieldsFragment["plans"]
  value: string | null | undefined
  onChange: (planId: string) => void
}) {
  const t = useT()
  const active = plans.filter((p) => p.isActive).sort((a, b) => a.sortOrder - b.sortOrder || a.durationDays - b.durationDays)
  if (active.length === 0) return <p className="text-sm text-muted-foreground">{t.catalog.noPlans}</p>
  return (
    <div role="radiogroup" aria-label={t.catalog.pickPlan} className="grid grid-cols-2 gap-2 sm:grid-cols-3">
      {active.map((p) => {
        const selected = p.id === value
        return (
          <button
            key={p.id}
            type="button"
            role="radio"
            aria-checked={selected}
            aria-label={`${p.name}, ${t.common.days(p.durationDays)}`}
            onClick={() => onChange(p.id)}
            className={
              selected
                ? "rounded-xl border-2 border-primary bg-primary/5 p-3 text-left"
                : "rounded-xl border-2 border-transparent bg-muted p-3 text-left hover:border-border"
            }
          >
            <p className="text-sm font-medium">{p.name}</p>
            <p className="text-xs text-muted-foreground">{t.common.days(p.durationDays)}</p>
            <MoneyText amount={p.price} currency={p.currency} tone="money" className="mt-1 block" />
          </button>
        )
      })}
    </div>
  )
}
