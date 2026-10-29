"use client"

import { useMemo } from "react"
import { useForm, useWatch } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { MoneyField, ResponsiveDialog, SelectField, TextareaField, TextField } from "@/components/shared"
import type { PaymentMethod } from "@/gql/graphql"
import { MOBILE_WALLETS, PAYMENT_METHODS } from "@/lib/enums"
import { dateTimeInputToISO } from "@/lib/dates"
import { v } from "@/lib/validation"
import { enumLabel, useT } from "@/i18n"
import { useRecordPayment } from "../hooks/use-billing"

export function RecordPaymentDialog({
  open,
  onOpenChange,
  invoiceId,
  customerId,
  defaultAmount = 0,
  description,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  invoiceId?: string | null
  customerId?: string | null
  defaultAmount?: number
  description?: string
}) {
  const t = useT()
  const [record, saving] = useRecordPayment()
  const schema = useMemo(
    () =>
      z
        .object({
          method: z.enum(PAYMENT_METHODS as [PaymentMethod, ...PaymentMethod[]]),
          amount: v.positiveMoney(t),
          trxId: v.optionalString(),
          senderNumber: v.optionalPhone(t),
          receivedAt: v.optionalString(),
          notes: v.optionalString(),
        })
        .refine((d) => !MOBILE_WALLETS.includes(d.method) || Boolean(d.trxId?.trim()), { path: ["trxId"], message: t.errors.required }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: { method: "BKASH", amount: defaultAmount, trxId: "", senderNumber: "", receivedAt: "", notes: "" },
  })
  const method = useWatch({ control: form.control, name: "method" })
  const wallet = MOBILE_WALLETS.includes(method)

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t.billing.recordPayment}
      description={description}
      footer={
        <Button type="submit" form="record-payment" disabled={saving}>
          {saving ? t.common.saving : t.billing.recordPayment}
        </Button>
      }
    >
      <form
        id="record-payment"
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          const ok = await record({
            invoiceId: invoiceId ?? null,
            customerId: customerId ?? null,
            input: {
              method: values.method,
              amount: values.amount,
              trxId: values.trxId?.trim().toUpperCase() || null,
              senderNumber: values.senderNumber || null,
              receivedAt: dateTimeInputToISO(values.receivedAt),
              notes: values.notes || null,
            },
          })
          if (ok) {
            form.reset()
            onOpenChange(false)
          }
        })}
      >
        <FieldGroup>
          <div className="grid grid-cols-2 gap-4">
            <SelectField
              control={form.control}
              name="method"
              label={t.billing.method}
              options={PAYMENT_METHODS.map((m) => ({ value: m, label: enumLabel(t, "PaymentMethod", m) }))}
            />
            <MoneyField control={form.control} name="amount" label={t.common.amount} required />
          </div>
          {wallet ? (
            <div className="grid gap-4 sm:grid-cols-2">
              <TextField control={form.control} name="trxId" label={t.billing.trxId} placeholder={t.billing.trxIdPlaceholder} required />
              <TextField control={form.control} name="senderNumber" type="tel" inputMode="tel" label={t.billing.senderNumber} placeholder="01XXXXXXXXX" />
            </div>
          ) : null}
          <TextField control={form.control} name="receivedAt" type="datetime-local" label={t.billing.receivedAt} />
          <TextareaField control={form.control} name="notes" label={t.common.notes} rows={2} />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}
