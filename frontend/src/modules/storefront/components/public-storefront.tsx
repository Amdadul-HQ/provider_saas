"use client"

import { useMemo, useState } from "react"
import { useForm, useWatch } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { Loader2Icon, PackageIcon, SearchIcon, StoreIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { EmptyState, ErrorState, LanguageToggle, MoneyText, ResponsiveDialog, SelectField, TextareaField, TextField, ToneBadge } from "@/components/shared"
import type { PaymentMethod, PublicStorefrontFieldsFragment } from "@/gql/graphql"
import { MOBILE_WALLETS } from "@/lib/enums"
import { normalizePhone, v } from "@/lib/validation"
import { enumLabel, useFormat, useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { usePlaceOrder, usePublicOrder, usePublicStorefront } from "../hooks/use-storefront"
import { themeStyle } from "../lib"
import { ContactLinks, PaymentNumbersList, PublicOrderSummary } from "./payment-instructions"

type PublicProduct = PublicStorefrontFieldsFragment["products"][number]

function OrderDialog({ slug, product, storefront, onOpenChange }: { slug: string; product: PublicProduct | null; storefront: PublicStorefrontFieldsFragment; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const f = useFormat()
  const [place, placing] = usePlaceOrder()
  const [placed, setPlaced] = useState<Awaited<ReturnType<typeof place>>>(null)
  const schema = useMemo(
    () =>
      z.object({
        planId: z.string().min(1, t.catalog.pickPlan),
        customerName: v.requiredString(t),
        customerPhone: v.phone(t),
        customerNote: v.optionalString(),
        paymentMethod: z.string().nullable().optional(),
        trxId: v.optionalString(),
        senderNumber: v.optionalPhone(t),
      }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const plans = [...(product?.plans ?? [])].sort((a, b) => a.durationDays - b.durationDays)
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: { planId: plans[0]?.id ?? "", customerName: "", customerPhone: "", customerNote: "", paymentMethod: null, trxId: "", senderNumber: "" },
  })
  const planId = useWatch({ control: form.control, name: "planId" })
  const method = useWatch({ control: form.control, name: "paymentMethod" })
  const plan = plans.find((p) => p.id === planId)

  const close = (o: boolean) => {
    if (!o) setPlaced(null)
    onOpenChange(o)
  }

  return (
    <ResponsiveDialog
      open={product !== null}
      onOpenChange={close}
      title={placed ? t.storefront.orderPlaced : t.storefront.orderTitle(product?.name ?? "")}
      footer={
        placed ? (
          <Button onClick={() => close(false)}>{t.common.close}</Button>
        ) : (
          <Button type="submit" form="place-order" size="lg" disabled={placing}>
            {placing ? <Loader2Icon className="animate-spin" /> : null}
            {placing ? t.storefront.placing : t.storefront.placeOrder}
            {plan ? <MoneyText amount={plan.price} currency={plan.currency} className="ml-1" /> : null}
          </Button>
        )
      }
    >
      {placed ? (
        <PublicOrderSummary order={placed.placeOrder} />
      ) : product ? (
        <form
          id="place-order"
          noValidate
          onSubmit={form.handleSubmit(async (values) => {
            const res = await place({
              input: {
                slug,
                productId: product.id,
                planId: values.planId,
                quantity: 1,
                customerName: values.customerName,
                customerPhone: normalizePhone(values.customerPhone),
                customerNote: values.customerNote || null,
                paymentMethod: (values.paymentMethod as PaymentMethod | null) ?? null,
                trxId: values.trxId?.trim().toUpperCase() || null,
                senderNumber: values.senderNumber ? normalizePhone(values.senderNumber) : null,
              },
            })
            if (res) setPlaced(res)
          })}
        >
          <FieldGroup>
            <div role="radiogroup" aria-label={t.catalog.pickPlan} className="grid grid-cols-2 gap-2">
              {plans.map((p) => (
                <button
                  key={p.id}
                  type="button"
                  role="radio"
                  aria-checked={p.id === planId}
                  aria-label={`${p.name}, ${t.common.days(p.durationDays)}, ${f.money(p.price, { currency: p.currency })}`}
                  onClick={() => form.setValue("planId", p.id, { shouldValidate: true })}
                  className={cn("rounded-xl border-2 p-3 text-left", p.id === planId ? "border-primary bg-primary/5" : "border-transparent bg-muted")}
                >
                  <p className="text-sm font-medium">{p.name}</p>
                  <p className="text-xs text-muted-foreground">{t.common.days(p.durationDays)}</p>
                  <MoneyText amount={p.price} currency={p.currency} tone="money" className="mt-1 block text-base" />
                </button>
              ))}
            </div>
            <TextField control={form.control} name="customerName" label={t.storefront.yourName} autoComplete="name" required />
            <TextField control={form.control} name="customerPhone" type="tel" inputMode="tel" label={t.storefront.yourPhone} placeholder="01XXXXXXXXX" autoComplete="tel" required />
            <TextareaField control={form.control} name="customerNote" label={t.storefront.note} rows={2} />
            <PaymentNumbersList paymentNumbers={storefront.paymentNumbers} instructions={storefront.paymentInstructions} />
            <p className="text-sm font-medium">{t.storefront.paymentOptional}</p>
            <SelectField
              control={form.control}
              name="paymentMethod"
              label={t.billing.method}
              emptyLabel={t.common.none}
              options={MOBILE_WALLETS.map((m) => ({ value: m, label: enumLabel(t, "PaymentMethod", m) }))}
            />
            {method ? (
              <div className="grid grid-cols-2 gap-3">
                <TextField control={form.control} name="trxId" label={t.billing.trxId} placeholder={t.billing.trxIdPlaceholder} />
                <TextField control={form.control} name="senderNumber" type="tel" inputMode="tel" label={t.billing.senderNumber} />
              </div>
            ) : null}
          </FieldGroup>
        </form>
      ) : null}
    </ResponsiveDialog>
  )
}

export function OrderTracker({ slug }: { slug: string }) {
  const t = useT()
  const [number, setNumber] = useState("")
  const [phone, setPhone] = useState("")
  const [query, setQuery] = useState<{ slug: string; number: number; phone: string } | null>(null)
  const [{ data, fetching, error }] = usePublicOrder(query)
  return (
    <section className="space-y-3 rounded-2xl border bg-card p-4">
      <div>
        <h2 className="font-semibold">{t.storefront.trackOrder}</h2>
        <p className="text-xs text-muted-foreground">{t.storefront.trackHint}</p>
      </div>
      <form
        className="grid grid-cols-[1fr_1fr_auto] items-end gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          const n = Number(number.replace(/\D/g, ""))
          if (n > 0 && phone.trim()) setQuery({ slug, number: n, phone: normalizePhone(phone) })
        }}
      >
        <div className="space-y-1">
          <Label htmlFor="track-no" className="text-xs">
            {t.storefront.orderNo}
          </Label>
          <Input id="track-no" inputMode="numeric" value={number} onChange={(e) => setNumber(e.target.value)} placeholder="1024" />
        </div>
        <div className="space-y-1">
          <Label htmlFor="track-phone" className="text-xs">
            {t.common.phone}
          </Label>
          <Input id="track-phone" type="tel" inputMode="tel" value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="01XXXXXXXXX" />
        </div>
        <Button type="submit" size="icon" aria-label={t.storefront.track} disabled={fetching}>
          {fetching ? <Loader2Icon className="animate-spin" /> : <SearchIcon />}
        </Button>
      </form>
      {query && !fetching ? (
        error ? (
          <ErrorState compact error={error} />
        ) : data?.publicOrder ? (
          <PublicOrderSummary order={data.publicOrder} />
        ) : data ? (
          <p className="text-sm text-muted-foreground">{t.storefront.orderNotFound}</p>
        ) : null
      ) : null}
    </section>
  )
}

function ProductCard({ product, onOrder }: { product: PublicProduct; onOrder: () => void }) {
  const t = useT()
  const cheapest = product.plans.reduce<PublicProduct["plans"][number] | null>((min, p) => (!min || p.price < min.price ? p : min), null)
  return (
    <article className="flex h-full flex-col overflow-hidden rounded-2xl border bg-card">
      {product.imageUrl ? (
        // eslint-disable-next-line @next/next/no-img-element -- seller-hosted images
        <img src={product.imageUrl} alt="" className="aspect-[16/9] w-full object-cover" loading="lazy" />
      ) : (
        <div className="flex aspect-[16/9] items-center justify-center bg-secondary text-secondary-foreground">
          <PackageIcon className="size-8" />
        </div>
      )}
      <div className="flex flex-1 flex-col gap-2 p-4">
        <div className="flex items-start justify-between gap-2">
          <h3 className="font-semibold">{product.name}</h3>
          <ToneBadge tone={product.inStock ? "success" : "danger"}>{product.inStock ? t.storefront.inStock : t.storefront.outOfStock}</ToneBadge>
        </div>
        {product.description ? <p className="line-clamp-3 text-sm text-muted-foreground">{product.description}</p> : null}
        <div className="mt-auto flex items-end justify-between gap-2 pt-2">
          {cheapest ? (
            <div>
              <p className="text-xs text-muted-foreground">{t.storefront.from}</p>
              <MoneyText amount={cheapest.price} currency={cheapest.currency} tone="money" className="text-xl" />
            </div>
          ) : (
            <span />
          )}
          <Button size="lg" disabled={!product.inStock || product.plans.length === 0} onClick={onOrder}>
            {t.storefront.order}
          </Button>
        </div>
      </div>
    </article>
  )
}

/** Public `/s/[slug]` storefront: no session, mobile-first, brandable via theme.primaryColor. */
export function PublicStorefrontView({ slug }: { slug: string }) {
  const t = useT()
  const [{ data, fetching, error }, refetch] = usePublicStorefront(slug)
  const [ordering, setOrdering] = useState<PublicProduct | null>(null)
  const sf = data?.publicStorefront

  if (!data) {
    if (error)
      return (
        <div className="mx-auto max-w-3xl p-4">
          <ErrorState error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
        </div>
      )
    return (
      <div className="mx-auto max-w-3xl space-y-4 p-4" aria-busy={fetching}>
        <Skeleton className="h-40 w-full rounded-2xl" />
        <div className="grid gap-4 sm:grid-cols-2">
          <Skeleton className="h-64 rounded-2xl" />
          <Skeleton className="h-64 rounded-2xl" />
        </div>
      </div>
    )
  }
  if (!sf) {
    return (
      <div className="mx-auto max-w-md p-4 pt-20">
        <EmptyState icon={StoreIcon} title={t.storefront.notFoundTitle} description={t.storefront.notFoundHint} />
      </div>
    )
  }
  const categories = Array.from(new Set(sf.products.map((p) => p.category ?? "")))
  return (
    <div style={themeStyle(sf.theme)} className="min-h-dvh bg-background pb-10">
      <header className="relative">
        {sf.coverUrl ? (
          // eslint-disable-next-line @next/next/no-img-element -- seller-hosted images
          <img src={sf.coverUrl} alt="" className="h-36 w-full object-cover md:h-52" />
        ) : (
          <div className="h-28 w-full bg-gradient-to-br from-primary to-primary/60 md:h-40" />
        )}
        <div className="absolute top-3 right-3">
          <LanguageToggle className="bg-card/80 backdrop-blur" />
        </div>
        <div className="mx-auto -mt-10 max-w-3xl px-4 text-center">
          {sf.logoUrl ? (
            // eslint-disable-next-line @next/next/no-img-element -- seller-hosted images
            <img src={sf.logoUrl} alt="" className="mx-auto size-20 rounded-2xl border-4 border-background object-cover shadow" />
          ) : (
            <div className="mx-auto flex size-20 items-center justify-center rounded-2xl border-4 border-background bg-primary text-2xl font-bold text-primary-foreground shadow">
              {sf.displayName.slice(0, 1)}
            </div>
          )}
          <h1 className="mt-2 font-heading text-2xl font-bold">{sf.displayName}</h1>
          {sf.tagline ? <p className="text-sm text-muted-foreground">{sf.tagline}</p> : null}
          <div className="mt-3">
            <ContactLinks whatsapp={sf.whatsapp} messengerUrl={sf.messengerUrl} telegram={sf.telegram} />
          </div>
        </div>
      </header>
      <main className="mx-auto mt-6 max-w-3xl space-y-8 px-4">
        {sf.products.length === 0 ? <EmptyState icon={PackageIcon} title={t.catalog.empty} /> : null}
        {categories.map((cat) => (
          <section key={cat || "_"} className="space-y-3">
            {cat ? <h2 className="text-sm font-semibold tracking-wide text-muted-foreground uppercase">{cat}</h2> : null}
            <div className="grid gap-4 sm:grid-cols-2">
              {sf.products
                .filter((p) => (p.category ?? "") === cat)
                .map((p) => (
                  <ProductCard key={p.id} product={p} onOrder={() => setOrdering(p)} />
                ))}
            </div>
          </section>
        ))}
        <OrderTracker slug={sf.slug} />
        <p className="text-center text-xs text-muted-foreground">{t.storefront.poweredBy}</p>
      </main>
      <OrderDialog slug={sf.slug} product={ordering} storefront={sf} onOpenChange={(o) => !o && setOrdering(null)} />
    </div>
  )
}
