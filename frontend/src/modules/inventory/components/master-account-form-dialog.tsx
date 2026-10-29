"use client"

import { useMemo } from "react"
import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { ShieldCheckIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { MoneyField, NumberField, ResponsiveDialog, SelectField, TextareaField, TextField } from "@/components/shared"
import type { MasterAccountFieldsFragment, MasterAccountStatus } from "@/gql/graphql"
import { MASTER_ACCOUNT_STATUSES } from "@/lib/enums"
import { dateInputToISO, isoToDateInput } from "@/lib/dates"
import { v } from "@/lib/validation"
import { ProductSelect } from "@/modules/catalog"
import { enumLabel, useT } from "@/i18n"
import { useCreateMasterAccount, useSuppliers, useUpdateMasterAccount } from "../hooks/use-inventory"

export function MasterAccountFormDialog({
  open,
  onOpenChange,
  account,
  onSaved,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  account?: MasterAccountFieldsFragment | null
  onSaved?: (a: MasterAccountFieldsFragment) => void
}) {
  const t = useT()
  const [{ data: suppliers }] = useSuppliers()
  const [create, creating] = useCreateMasterAccount()
  const [update, updating] = useUpdateMasterAccount()
  const isEdit = Boolean(account)
  const schema = useMemo(
    () =>
      z.object({
        productId: z.string({ error: t.errors.required }).min(1, t.errors.required),
        supplierId: z.string().nullable().optional(),
        label: v.requiredString(t),
        loginHint: v.optionalString(),
        username: v.optionalString(),
        password: v.optionalString(),
        totalSlots: v.positiveInt(t),
        purchaseCost: v.money(t),
        purchasedAt: v.optionalString(),
        expiresAt: v.optionalString(),
        notes: v.optionalString(),
        status: z.enum(MASTER_ACCOUNT_STATUSES as [MasterAccountStatus, ...MasterAccountStatus[]]).optional(),
      }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: {
      productId: account?.product.id ?? "",
      supplierId: account?.supplier?.id ?? null,
      label: account?.label ?? "",
      loginHint: account?.loginHint ?? "",
      username: "",
      password: "",
      totalSlots: account?.totalSlots ?? 5,
      purchaseCost: account?.purchaseCost ?? 0,
      purchasedAt: isoToDateInput(account?.purchasedAt),
      expiresAt: isoToDateInput(account?.expiresAt),
      notes: account?.notes ?? "",
      status: account?.status ?? "ACTIVE",
    },
  })

  async function submit(values: Values) {
    const common = {
      supplierId: values.supplierId || null,
      label: values.label,
      loginHint: values.loginHint || null,
      totalSlots: values.totalSlots,
      purchaseCost: values.purchaseCost,
      purchasedAt: dateInputToISO(values.purchasedAt),
      expiresAt: dateInputToISO(values.expiresAt),
      notes: values.notes || null,
    }
    const res = account
      ? (await update({ id: account.id, input: { ...common, status: values.status } }))?.updateMasterAccount
      : (
          await create({
            input: { ...common, productId: values.productId, username: values.username || null, password: values.password || null },
          })
        )?.createMasterAccount
    if (res) {
      // never keep plaintext secrets in form state longer than needed
      form.resetField("password")
      onSaved?.(res)
      onOpenChange(false)
    }
  }

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={isEdit ? t.inventory.editAccount : t.inventory.newAccount}
      className="sm:max-w-xl"
      footer={
        <Button type="submit" form="master-account-form" disabled={creating || updating}>
          {creating || updating ? t.common.saving : t.common.save}
        </Button>
      }
    >
      <form id="master-account-form" noValidate onSubmit={form.handleSubmit(submit)} autoComplete="off">
        <FieldGroup>
          <Controller
            control={form.control}
            name="productId"
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid || undefined}>
                <FieldLabel htmlFor="ma-product">
                  {t.common.product}
                  <span className="text-destructive">*</span>
                </FieldLabel>
                {isEdit ? (
                  <p className="text-sm font-medium">{account?.product.name}</p>
                ) : (
                  <ProductSelect id="ma-product" value={field.value || null} onChange={(p) => field.onChange(p?.id ?? "")} invalid={fieldState.invalid} />
                )}
                <FieldError errors={[fieldState.error]} />
              </Field>
            )}
          />
          <div className="grid gap-4 sm:grid-cols-2">
            <TextField control={form.control} name="label" label={t.inventory.label} placeholder={t.inventory.labelPlaceholder} required />
            <SelectField
              control={form.control}
              name="supplierId"
              label={t.inventory.supplier}
              emptyLabel={t.common.none}
              options={(suppliers?.suppliers ?? []).map((s) => ({ value: s.id, label: s.name }))}
            />
            <NumberField control={form.control} name="totalSlots" label={t.inventory.totalSlots} min={1} required />
            <MoneyField control={form.control} name="purchaseCost" label={t.inventory.purchaseCost} required />
            <TextField control={form.control} name="purchasedAt" type="date" label={t.inventory.purchasedAt} />
            <TextField control={form.control} name="expiresAt" type="date" label={t.inventory.expiresAt} />
          </div>
          <TextField control={form.control} name="loginHint" label={t.inventory.loginHint} placeholder={t.inventory.loginHintPlaceholder} />
          {!isEdit ? (
            <div className="space-y-3 rounded-lg border border-dashed p-3">
              <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <ShieldCheckIcon className="size-3.5 text-success" /> {t.inventory.credentialHint}
              </p>
              <div className="grid gap-4 sm:grid-cols-2">
                <TextField control={form.control} name="username" label={t.inventory.username} autoComplete="off" />
                <TextField control={form.control} name="password" type="password" label={t.inventory.password} autoComplete="new-password" />
              </div>
            </div>
          ) : (
            <SelectField
              control={form.control}
              name="status"
              label={t.common.status}
              options={MASTER_ACCOUNT_STATUSES.map((s) => ({ value: s, label: enumLabel(t, "MasterAccountStatus", s) }))}
            />
          )}
          <TextareaField control={form.control} name="notes" label={t.common.notes} rows={2} />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}
