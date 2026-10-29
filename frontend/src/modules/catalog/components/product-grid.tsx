"use client"

import { useState } from "react"
import { PackageIcon } from "lucide-react"
import { EmptyState, ErrorState, FilterToggle, MoneyText, SearchInput, ToneBadge } from "@/components/shared"
import { Skeleton } from "@/components/ui/skeleton"
import type { ProductFieldsFragment, ProductFilter } from "@/gql/graphql"
import { enumLabel, useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { useProducts } from "../hooks/use-catalog"

export function ProductCard({ product, onClick }: { product: ProductFieldsFragment; onClick?: () => void }) {
  const t = useT()
  const active = product.plans.filter((p) => p.isActive)
  const cheapest = active.reduce<number | null>((min, p) => (min === null || p.price < min ? p.price : min), null)
  const Comp = onClick ? "button" : "div"
  return (
    <Comp
      type={onClick ? "button" : undefined}
      onClick={onClick}
      className={cn(
        "flex h-full w-full flex-col gap-3 rounded-xl border bg-card p-4 text-left transition-colors",
        onClick && "hover:border-primary/40 active:bg-muted",
        !product.isActive && "opacity-60",
      )}
    >
      <div className="flex items-start gap-3">
        {product.imageUrl ? (
          // eslint-disable-next-line @next/next/no-img-element -- arbitrary seller-hosted URLs
          <img src={product.imageUrl} alt="" className="size-11 rounded-lg object-cover" />
        ) : (
          <span className="flex size-11 items-center justify-center rounded-lg bg-secondary text-secondary-foreground">
            <PackageIcon className="size-5" />
          </span>
        )}
        <div className="min-w-0 flex-1">
          <p className="truncate font-semibold">{product.name}</p>
          <p className="truncate text-xs text-muted-foreground">
            {product.category ?? "—"} · {enumLabel(t, "DeliveryType", product.deliveryType)}
          </p>
        </div>
        {!product.isActive ? <ToneBadge>{t.common.inactive}</ToneBadge> : null}
      </div>
      <div className="mt-auto flex items-end justify-between gap-2">
        <div className="flex flex-wrap gap-1.5 text-xs">
          <ToneBadge tone={product.freeSlotCount > 0 ? "success" : "warning"}>{t.catalog.freeSlots(product.freeSlotCount)}</ToneBadge>
          <ToneBadge tone="primary">{t.catalog.activeSubs(product.activeSubscriptionCount)}</ToneBadge>
        </div>
        {cheapest !== null ? <MoneyText amount={cheapest} tone="money" className="text-lg" /> : null}
      </div>
    </Comp>
  )
}

export function ProductGrid({ onSelect, emptyAction }: { onSelect?: (p: ProductFieldsFragment) => void; emptyAction?: React.ReactNode }) {
  const t = useT()
  const [filter, setFilter] = useState<ProductFilter>({ isActive: true })
  const [{ data, fetching, error }, refetch] = useProducts(filter, { first: 100 })
  const products = data?.products

  return (
    <div className="space-y-3">
      <div className="flex flex-col gap-2 md:flex-row md:items-center">
        <SearchInput value={filter.search ?? ""} onChange={(search) => setFilter((f) => ({ ...f, search: search || undefined }))} />
        <FilterToggle
          id="prod-inactive"
          label={t.catalog.showInactive}
          checked={filter.isActive === undefined}
          onChange={(c) => setFilter((f) => ({ ...f, isActive: c ? undefined : true }))}
        />
      </div>
      {!products ? (
        error ? (
          <ErrorState error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-32 rounded-xl" />
            ))}
          </div>
        )
      ) : products.length === 0 ? (
        <EmptyState icon={PackageIcon} title={t.catalog.empty} description={t.catalog.emptyHint} action={emptyAction} />
      ) : (
        <ul className={cn("grid gap-3 sm:grid-cols-2 xl:grid-cols-3", fetching && "opacity-70")}>
          {products.map((p) => (
            <li key={p.id}>
              <ProductCard product={p} onClick={onSelect ? () => onSelect(p) : undefined} />
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
