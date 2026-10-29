"use client"

import { useMemo } from "react"
import { Controller, useFieldArray, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { PlusIcon, Trash2Icon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { FieldGroup, FieldLegend, FieldSet } from "@/components/ui/field"
import { ImagePicker, MoneyField, NumberField, ResponsiveDialog, SelectField, SwitchField, TextareaField, TextField } from "@/components/shared"
import type { DeliveryType, ProductFieldsFragment } from "@/gql/graphql"
import { DELIVERY_TYPES } from "@/lib/enums"
import { v } from "@/lib/validation"
import { enumLabel, useT } from "@/i18n"
import { useCreateProduct, useUpdateProduct } from "../hooks/use-catalog"

export function ProductFormDialog({
  open,
  onOpenChange,
  product,
  onSaved,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  product?: ProductFieldsFragment | null
  onSaved?: (product: ProductFieldsFragment) => void
}) {
  const t = useT()
  const [create, creating] = useCreateProduct()
  const [update, updating] = useUpdateProduct()
  const isEdit = Boolean(product)
  const schema = useMemo(
    () =>
      z.object({
        name: v.requiredString(t),
        category: v.optionalString(),
        description: v.optionalString(),
        imageUrl: v.optionalString(),
        deliveryType: z.enum(DELIVERY_TYPES as [DeliveryType, ...DeliveryType[]]),
        warrantyDays: v.optionalInt(t),
        deliveryTemplate: v.optionalString(),
        isActive: z.boolean(),
        sortOrder: v.optionalInt(t),
        plans: z.array(
          z.object({ name: v.requiredString(t), durationDays: v.positiveInt(t), price: v.money(t), cost: v.optionalMoney(t), resellerPrice: v.optionalMoney(t) }),
        ),
      }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: {
      name: product?.name ?? "",
      category: product?.category ?? "",
      description: product?.description ?? "",
      imageUrl: product?.imageUrl ?? "",
      deliveryType: product?.deliveryType ?? "SHARED_SLOT",
      warrantyDays: product?.warrantyDays ?? 0,
      deliveryTemplate: product?.deliveryTemplate ?? "",
      isActive: product?.isActive ?? true,
      sortOrder: product?.sortOrder ?? 0,
      plans: isEdit ? [] : [{ name: "", durationDays: 30, price: 0, cost: 0, resellerPrice: null }],
    },
  })
  const plans = useFieldArray({ control: form.control, name: "plans" })

  async function submit(values: Values) {
    const base = {
      name: values.name,
      category: values.category || null,
      description: values.description || null,
      imageUrl: values.imageUrl || null,
      deliveryType: values.deliveryType,
      warrantyDays: values.warrantyDays ?? 0,
      deliveryTemplate: values.deliveryTemplate || null,
    }
    if (product) {
      const res = await update({ id: product.id, input: { ...base, isActive: values.isActive, sortOrder: values.sortOrder ?? 0 } })
      if (res) {
        onSaved?.(res.updateProduct)
        onOpenChange(false)
      }
    } else {
      const res = await create({
        input: { ...base, plans: values.plans.map((p, i) => ({ ...p, cost: p.cost ?? 0, resellerPrice: p.resellerPrice ?? null, sortOrder: i, isActive: true })) },
      })
      if (res) {
        onSaved?.(res.createProduct)
        onOpenChange(false)
      }
    }
  }

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={isEdit ? t.catalog.edit : t.catalog.new}
      className="sm:max-w-2xl"
      footer={
        <>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t.common.cancel}
          </Button>
          <Button type="submit" form="product-form" disabled={creating || updating}>
            {creating || updating ? t.common.saving : t.common.save}
          </Button>
        </>
      }
    >
      <form id="product-form" noValidate onSubmit={form.handleSubmit(submit)}>
        <FieldGroup>
          <TextField control={form.control} name="name" label={t.common.name} placeholder="Netflix Premium" required />
          <div className="grid gap-4 sm:grid-cols-2">
            <TextField control={form.control} name="category" label={t.catalog.category} placeholder="Streaming" />
            <SelectField
              control={form.control}
              name="deliveryType"
              label={t.catalog.deliveryType}
              options={DELIVERY_TYPES.map((d) => ({ value: d, label: enumLabel(t, "DeliveryType", d) }))}
            />
            <NumberField control={form.control} name="warrantyDays" label={t.catalog.warrantyDays} min={0} />
          </div>
          <Controller
            control={form.control}
            name="imageUrl"
            render={({ field }) => <ImagePicker label={t.catalog.imageUrl} value={field.value} onChange={(url) => field.onChange(url ?? "")} />}
          />
          <TextareaField control={form.control} name="description" label={t.catalog.description} rows={2} />
          <TextareaField control={form.control} name="deliveryTemplate" label={t.catalog.deliveryTemplate} description={t.catalog.deliveryTemplateHint} rows={3} />
          {isEdit ? <SwitchField control={form.control} name="isActive" label={t.common.active} /> : null}
          {!isEdit ? (
            <FieldSet>
              <FieldLegend>{t.catalog.plans}</FieldLegend>
              <div className="space-y-3">
                {plans.fields.map((f, i) => (
                  <div key={f.id} className="grid grid-cols-2 items-start gap-2 rounded-lg border p-3 sm:grid-cols-[1fr_5rem_6.5rem_6.5rem_6.5rem_auto]">
                    <TextField control={form.control} name={`plans.${i}.name`} label={t.catalog.planName} placeholder={t.catalog.planNamePlaceholder} className="col-span-2 sm:col-span-1" />
                    <NumberField control={form.control} name={`plans.${i}.durationDays`} label={t.catalog.durationDays} min={1} />
                    <MoneyField control={form.control} name={`plans.${i}.price`} label={t.common.price} />
                    <MoneyField control={form.control} name={`plans.${i}.cost`} label={t.common.cost} />
                    <MoneyField control={form.control} name={`plans.${i}.resellerPrice`} label={t.catalog.resellerPrice} />
                    <Button type="button" variant="ghost" size="icon" className="self-end" aria-label={t.common.delete} onClick={() => plans.remove(i)}>
                      <Trash2Icon />
                    </Button>
                  </div>
                ))}
                <Button type="button" variant="outline" onClick={() => plans.append({ name: "", durationDays: 30, price: 0, cost: 0, resellerPrice: null })}>
                  <PlusIcon /> {t.catalog.addPlan}
                </Button>
              </div>
            </FieldSet>
          ) : null}
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}
