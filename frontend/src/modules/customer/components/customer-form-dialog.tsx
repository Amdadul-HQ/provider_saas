"use client"

import { useMemo } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { ResponsiveDialog, SelectField, TextareaField, TextField } from "@/components/shared"
import type { CustomerDetailFieldsFragment, CustomerListFieldsFragment, CustomerStatus, Platform } from "@/gql/graphql"
import { CUSTOMER_STATUSES, PLATFORMS } from "@/lib/enums"
import { normalizePhone, v } from "@/lib/validation"
import { enumLabel, useT } from "@/i18n"
import { useCreateCustomer, useUpdateCustomer } from "../hooks/use-customers"

type Editable = Partial<CustomerDetailFieldsFragment> & Pick<CustomerListFieldsFragment, "id" | "name">

export function CustomerFormDialog({
  open,
  onOpenChange,
  customer,
  onSaved,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Pass to edit; omit to create. */
  customer?: Editable | null
  onSaved?: (customer: { id: string; name: string }) => void
}) {
  const t = useT()
  const [create, creating] = useCreateCustomer()
  const [update, updating] = useUpdateCustomer()
  const schema = useMemo(
    () =>
      z.object({
        name: v.requiredString(t),
        phone: v.optionalPhone(t),
        email: v.optionalEmail(t),
        whatsapp: v.optionalPhone(t),
        telegram: v.optionalString(),
        address: v.optionalString(),
        tags: v.optionalString(),
        notes: v.optionalString(),
        preferredChannel: z.enum(PLATFORMS as [Platform, ...Platform[]]).nullable().optional(),
        status: z.enum(CUSTOMER_STATUSES as [CustomerStatus, ...CustomerStatus[]]).optional(),
      }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: {
      name: customer?.name ?? "",
      phone: customer?.phone ?? "",
      email: customer?.email ?? "",
      whatsapp: customer?.whatsapp ?? "",
      telegram: customer?.telegram ?? "",
      address: customer?.address ?? "",
      tags: customer?.tags?.join(", ") ?? "",
      notes: customer?.notes ?? "",
      preferredChannel: customer?.preferredChannel ?? null,
      status: customer?.status ?? "ACTIVE",
    },
  })
  const saving = creating || updating

  async function submit(values: Values) {
    const common = {
      name: values.name,
      phone: values.phone ? normalizePhone(values.phone) : null,
      email: values.email || null,
      whatsapp: values.whatsapp ? normalizePhone(values.whatsapp) : null,
      telegram: values.telegram || null,
      address: values.address || null,
      notes: values.notes || null,
      tags: (values.tags ?? "")
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean),
      preferredChannel: values.preferredChannel ?? null,
    }
    if (customer?.id) {
      const res = await update({ id: customer.id, input: { ...common, status: values.status } })
      if (res) {
        onSaved?.(res.updateCustomer)
        onOpenChange(false)
      }
    } else {
      const res = await create({ input: common })
      if (res) {
        onSaved?.(res.createCustomer)
        form.reset()
        onOpenChange(false)
      }
    }
  }

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={customer?.id ? t.customers.edit : t.customers.new}
      footer={
        <>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t.common.cancel}
          </Button>
          <Button type="submit" form="customer-form" disabled={saving}>
            {saving ? t.common.saving : t.common.save}
          </Button>
        </>
      }
    >
      <form id="customer-form" noValidate onSubmit={form.handleSubmit(submit)}>
        <FieldGroup>
          <TextField control={form.control} name="name" label={t.common.name} required autoComplete="off" />
          <div className="grid gap-4 sm:grid-cols-2">
            <TextField control={form.control} name="phone" type="tel" inputMode="tel" label={t.common.phone} placeholder="01XXXXXXXXX" />
            <TextField control={form.control} name="whatsapp" type="tel" inputMode="tel" label={t.customers.whatsapp} placeholder="01XXXXXXXXX" />
            <TextField control={form.control} name="email" type="email" label={t.common.email} />
            <TextField control={form.control} name="telegram" label={t.customers.telegram} placeholder="@username" />
          </div>
          <SelectField
            control={form.control}
            name="preferredChannel"
            label={t.customers.preferredChannel}
            emptyLabel={t.common.none}
            options={PLATFORMS.map((p) => ({ value: p, label: enumLabel(t, "Platform", p) }))}
          />
          {customer?.id ? (
            <SelectField
              control={form.control}
              name="status"
              label={t.common.status}
              options={CUSTOMER_STATUSES.map((s) => ({ value: s, label: enumLabel(t, "CustomerStatus", s) }))}
            />
          ) : null}
          <TextField control={form.control} name="tags" label={t.customers.tags} description={t.customers.tagsHint} placeholder="vip, netflix" />
          <TextField control={form.control} name="address" label={t.customers.address} />
          <TextareaField control={form.control} name="notes" label={t.common.notes} rows={2} />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}
