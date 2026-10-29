"use client"

import { useWatch, type Control, type FieldValues, type Path } from "react-hook-form"
import { MoneyField, SelectField, TextField } from "@/components/shared"
import { MOBILE_WALLETS, PAYMENT_METHODS } from "@/lib/enums"
import { enumLabel, useT } from "@/i18n"
import type { PaymentMethod } from "@/gql/graphql"

/** Fields that build a `PaymentInput` under `pay.*` of any form. */
export type PayValues = { method: PaymentMethod; amount: number; trxId?: string | null; senderNumber?: string | null }

export function PaymentFields<T extends FieldValues & { pay: PayValues }>({ control }: { control: Control<T> }) {
  const t = useT()
  const method = useWatch({ control, name: "pay.method" as Path<T> }) as PaymentMethod | undefined
  const wallet = method ? MOBILE_WALLETS.includes(method) : false
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <SelectField
        control={control}
        name={"pay.method" as Path<T>}
        label={t.billing.method}
        options={PAYMENT_METHODS.map((m) => ({ value: m, label: enumLabel(t, "PaymentMethod", m) }))}
      />
      <MoneyField control={control} name={"pay.amount" as Path<T>} label={t.common.amount} required />
      {wallet ? (
        <>
          <TextField control={control} name={"pay.trxId" as Path<T>} label={t.billing.trxId} placeholder={t.billing.trxIdPlaceholder} />
          <TextField control={control} name={"pay.senderNumber" as Path<T>} type="tel" inputMode="tel" label={t.billing.senderNumber} placeholder="01XXXXXXXXX" />
        </>
      ) : null}
    </div>
  )
}

export function toPaymentInput(pay: PayValues) {
  return {
    method: pay.method,
    amount: pay.amount,
    trxId: pay.trxId?.trim() || null,
    senderNumber: pay.senderNumber?.trim() || null,
  }
}
