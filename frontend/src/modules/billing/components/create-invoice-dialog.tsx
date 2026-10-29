"use client"

import { useMemo } from "react"
import { Controller, useFieldArray, useForm, useWatch } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { PlusIcon, Trash2Icon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { MoneyField, MoneyText, NumberField, ResponsiveDialog, SwitchField, TextareaField, TextField } from "@/components/shared"
import { dateInputToISO } from "@/lib/dates"
import { v } from "@/lib/validation"
import { CustomerPicker, type PickedCustomer } from "@/modules/customer"
import { useT } from "@/i18n"
import { useCreateInvoice } from "../hooks/use-billing"

export function CreateInvoiceDialog({
  open,
  onOpenChange,
  defaultCustomer,
  onCreated,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  defaultCustomer?: PickedCustomer | null
  onCreated?: (invoiceId: string) => void
}) {
  const t = useT()
  const [create, saving] = useCreateInvoice()
  const schema = useMemo(
    () =>
      z.object({
        customer: z.object({ id: z.string(), name: z.string(), phone: z.string().nullable().optional() }).nullable(),
        items: z.array(z.object({ description: v.requiredString(t), quantity: v.positiveInt(t), unitPrice: v.money(t) })).min(1),
        discount: v.optionalMoney(t),
        dueAt: v.optionalString(),
        notes: v.optionalString(),
        send: z.boolean(),
      })
      .refine((d) => d.customer !== null, { path: ["customer"], message: t.errors.required }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: { customer: defaultCustomer ?? null, items: [{ description: "", quantity: 1, unitPrice: 0 }], discount: 0, dueAt: "", notes: "", send: false },
  })
  const items = useFieldArray({ control: form.control, name: "items" })
  const watched = useWatch({ control: form.control, name: "items" })
  const discount = useWatch({ control: form.control, name: "discount" }) ?? 0
  const total = (watched ?? []).reduce((s, i) => s + (i.quantity ?? 0) * (i.unitPrice ?? 0), 0) - discount

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t.billing.newInvoice}
      className="sm:max-w-2xl"
      footer={
        <div className="flex w-full items-center justify-between gap-2">
          <MoneyText amount={total} tone="money" className="text-lg" />
          <Button type="submit" form="create-invoice" disabled={saving}>
            {t.common.create}
          </Button>
        </div>
      }
    >
      <form
        id="create-invoice"
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          if (!values.customer) return
          const res = await create({
            input: {
              customerId: values.customer.id,
              items: values.items,
              discount: values.discount ?? 0,
              dueAt: dateInputToISO(values.dueAt),
              notes: values.notes || null,
              send: values.send,
            },
          })
          if (res) {
            onCreated?.(res.createInvoice.id)
            form.reset()
            onOpenChange(false)
          }
        })}
      >
        <FieldGroup>
          <Controller
            control={form.control}
            name="customer"
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid || undefined}>
                <FieldLabel>{t.common.customer}</FieldLabel>
                <CustomerPicker value={field.value} onChange={field.onChange} invalid={fieldState.invalid} />
                <FieldError errors={[fieldState.error]} />
              </Field>
            )}
          />
          <div className="space-y-2">
            {items.fields.map((f, i) => (
              <div key={f.id} className="grid grid-cols-[1fr_5rem] items-start gap-2 rounded-lg border p-3 sm:grid-cols-[1fr_5rem_8rem_auto]">
                <TextField control={form.control} name={`items.${i}.description`} label={t.billing.itemDescription} className="col-span-2 sm:col-span-1" />
                <NumberField control={form.control} name={`items.${i}.quantity`} label={t.billing.qty} min={1} />
                <MoneyField control={form.control} name={`items.${i}.unitPrice`} label={t.billing.unitPrice} />
                <Button type="button" variant="ghost" size="icon" className="self-end" disabled={items.fields.length === 1} aria-label={t.common.delete} onClick={() => items.remove(i)}>
                  <Trash2Icon />
                </Button>
              </div>
            ))}
            <Button type="button" variant="outline" onClick={() => items.append({ description: "", quantity: 1, unitPrice: 0 })}>
              <PlusIcon /> {t.billing.addItem}
            </Button>
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <MoneyField control={form.control} name="discount" label={t.billing.discount} />
            <TextField control={form.control} name="dueAt" type="date" label={t.billing.dueAt} />
          </div>
          <TextareaField control={form.control} name="notes" label={t.common.notes} rows={2} />
          <SwitchField control={form.control} name="send" label={t.billing.sendNow} />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}
