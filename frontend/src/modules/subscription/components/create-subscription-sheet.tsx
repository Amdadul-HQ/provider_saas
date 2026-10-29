"use client"

import { useMemo, useState } from "react"
import { Controller, useForm, useWatch } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { ChevronDownIcon, Loader2Icon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Field, FieldError, FieldGroup, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { MoneyField, MoneyText, NumberField, SwitchField, TextareaField, TextField } from "@/components/shared"
import type { PaymentMethod, ProductFieldsFragment } from "@/gql/graphql"
import { useIsDesktop } from "@/hooks/use-media-query"
import { PAYMENT_METHODS } from "@/lib/enums"
import { dateInputToISO, isoToDateInput } from "@/lib/dates"
import { v } from "@/lib/validation"
import { CustomerPicker, type PickedCustomer } from "@/modules/customer"
import { PlanPicker, ProductSelect } from "@/modules/catalog"
import { SlotSelect } from "@/modules/inventory"
import { useT } from "@/i18n"
import { useCreateSubscription } from "../hooks/use-subscriptions"
import { PaymentFields, toPaymentInput } from "./payment-fields"

/**
 * New sale in one sheet: customer → product/plan → (auto) slot → optional payment.
 * Mobile: bottom sheet; desktop: right sheet.
 */
export function CreateSubscriptionSheet({
  open,
  onOpenChange,
  defaultCustomer,
  onCreated,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  defaultCustomer?: PickedCustomer | null
  onCreated?: (subscriptionId: string) => void
}) {
  const t = useT()
  const isDesktop = useIsDesktop()
  const [create, creating] = useCreateSubscription()
  const [product, setProduct] = useState<ProductFieldsFragment | null>(null)
  const [showAdvanced, setShowAdvanced] = useState(false)

  const schema = useMemo(
    () =>
      z
        .object({
          customer: z.object({ id: z.string(), name: z.string(), phone: z.string().nullable().optional() }, { error: t.errors.required }).nullable(),
          productId: z.string().min(1, t.errors.required),
          planId: z.string().nullable(),
          slotId: z.string().nullable(),
          startsAt: z.string(),
          durationDays: v.optionalInt(t),
          price: v.optionalMoney(t),
          autoRenew: z.boolean(),
          notes: v.optionalString(),
          createInvoice: z.boolean(),
          markPaid: z.boolean(),
          pay: z.object({
            method: z.enum(PAYMENT_METHODS as [PaymentMethod, ...PaymentMethod[]]),
            amount: z.number().int().min(0),
            trxId: z.string().nullable().optional(),
            senderNumber: z.string().nullable().optional(),
          }),
        })
        .refine((d) => d.customer !== null, { path: ["customer"], message: t.errors.required })
        .refine((d) => d.planId !== null || (d.durationDays ?? 0) > 0, { path: ["planId"], message: t.catalog.pickPlan }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: {
      customer: defaultCustomer ?? null,
      productId: "",
      planId: null,
      slotId: null,
      startsAt: isoToDateInput(new Date().toISOString()),
      durationDays: null,
      price: null,
      autoRenew: false,
      notes: "",
      createInvoice: true,
      markPaid: false,
      pay: { method: "BKASH", amount: 0, trxId: "", senderNumber: "" },
    },
  })
  const planId = useWatch({ control: form.control, name: "planId" })
  const markPaid = useWatch({ control: form.control, name: "markPaid" })
  const priceOverride = useWatch({ control: form.control, name: "price" })
  const plan = product?.plans.find((p) => p.id === planId) ?? null
  const effectivePrice = priceOverride ?? plan?.price ?? 0

  async function submit(values: Values) {
    if (!values.customer) return
    const res = await create({
      input: {
        customerId: values.customer.id,
        productId: values.productId,
        planId: values.planId,
        slotId: values.slotId,
        startsAt: dateInputToISO(values.startsAt),
        durationDays: values.durationDays ?? null,
        price: values.price ?? null,
        autoRenew: values.autoRenew,
        notes: values.notes || null,
        createInvoice: values.createInvoice,
        markPaid: values.markPaid ? toPaymentInput({ ...values.pay, amount: values.pay.amount || effectivePrice }) : null,
      },
    })
    if (res) {
      onCreated?.(res.createSubscription.id)
      form.reset()
      setProduct(null)
      onOpenChange(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side={isDesktop ? "right" : "bottom"} className={isDesktop ? "w-full sm:max-w-lg" : "max-h-[94dvh] rounded-t-2xl"}>
        <SheetHeader>
          <SheetTitle>{t.subscriptions.new}</SheetTitle>
          <SheetDescription>{t.subscriptions.emptyHint}</SheetDescription>
        </SheetHeader>
        <form id="create-subscription" noValidate onSubmit={form.handleSubmit(submit)} className="flex-1 overflow-y-auto px-4">
          <FieldGroup>
            <FieldSet>
              <FieldLegend variant="label">1 · {t.subscriptions.steps.customer}</FieldLegend>
              <Controller
                control={form.control}
                name="customer"
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid || undefined}>
                    <CustomerPicker value={field.value ?? null} onChange={field.onChange} invalid={fieldState.invalid} />
                    <FieldError errors={[fieldState.error]} />
                  </Field>
                )}
              />
            </FieldSet>

            <FieldSet>
              <FieldLegend variant="label">2 · {t.subscriptions.steps.product}</FieldLegend>
              <Controller
                control={form.control}
                name="productId"
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid || undefined}>
                    <ProductSelect
                      value={field.value || null}
                      invalid={fieldState.invalid}
                      onChange={(p) => {
                        setProduct(p)
                        field.onChange(p?.id ?? "")
                        const first = p?.plans.find((pl) => pl.isActive)
                        form.setValue("planId", first?.id ?? null)
                        form.setValue("slotId", null)
                        form.setValue("pay.amount", first?.price ?? 0)
                      }}
                    />
                    <FieldError errors={[fieldState.error]} />
                  </Field>
                )}
              />
              {product ? (
                <Controller
                  control={form.control}
                  name="planId"
                  render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid || undefined}>
                      <PlanPicker
                        plans={product.plans}
                        value={field.value}
                        onChange={(id) => {
                          field.onChange(id)
                          form.setValue("pay.amount", product.plans.find((p) => p.id === id)?.price ?? 0)
                        }}
                      />
                      <FieldError errors={[fieldState.error]} />
                    </Field>
                  )}
                />
              ) : null}
              {product?.deliveryType === "SHARED_SLOT" ? (
                <Controller
                  control={form.control}
                  name="slotId"
                  render={({ field }) => (
                    <Field>
                      <FieldLabel htmlFor="sub-slot">{t.subscriptions.slot}</FieldLabel>
                      <SlotSelect id="sub-slot" productId={product.id} value={field.value} onChange={field.onChange} />
                    </Field>
                  )}
                />
              ) : null}
              <TextField control={form.control} name="startsAt" type="date" label={t.subscriptions.startsAt} />
              <button
                type="button"
                className="flex items-center gap-1 text-sm font-medium text-primary"
                onClick={() => setShowAdvanced((s) => !s)}
                aria-expanded={showAdvanced}
              >
                <ChevronDownIcon className={showAdvanced ? "size-4 rotate-180 transition-transform" : "size-4 transition-transform"} />
                {t.common.more}
              </button>
              {showAdvanced ? (
                <div className="space-y-4">
                  <div className="grid grid-cols-2 gap-4">
                    <NumberField control={form.control} name="durationDays" label={t.subscriptions.durationOverride} min={1} />
                    <MoneyField control={form.control} name="price" label={t.subscriptions.priceOverride} />
                  </div>
                  <SwitchField control={form.control} name="autoRenew" label={t.subscriptions.autoRenew} />
                  <TextareaField control={form.control} name="notes" label={t.common.notes} rows={2} />
                </div>
              ) : null}
            </FieldSet>

            <FieldSet>
              <FieldLegend variant="label">3 · {t.subscriptions.steps.payment}</FieldLegend>
              <SwitchField control={form.control} name="createInvoice" label={t.subscriptions.createInvoice} />
              <SwitchField control={form.control} name="markPaid" label={t.subscriptions.markPaid} />
              {markPaid ? <PaymentFields control={form.control} /> : null}
            </FieldSet>
          </FieldGroup>
        </form>
        <SheetFooter className="border-t pb-[max(1rem,env(safe-area-inset-bottom))]">
          <div className="flex items-center justify-between text-sm">
            <span className="text-muted-foreground">{t.common.total}</span>
            <MoneyText amount={effectivePrice} tone="money" className="text-lg" />
          </div>
          <Button type="submit" form="create-subscription" size="lg" disabled={creating}>
            {creating ? <Loader2Icon className="animate-spin" /> : null}
            {t.common.create}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
