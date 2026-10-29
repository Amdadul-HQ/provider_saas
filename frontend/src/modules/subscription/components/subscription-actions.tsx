"use client"

import { useMemo, useState } from "react"
import { useForm, useWatch } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { BellRingIcon, CalendarPlusIcon, CheckCheckIcon, MoreVerticalIcon, RefreshCwIcon, XCircleIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { ConfirmDialog, NumberField, ResponsiveDialog, SelectField, SwitchField, TextareaField, TextField } from "@/components/shared"
import type { PaymentMethod, SubscriptionListFieldsFragment } from "@/gql/graphql"
import { PAYMENT_METHODS } from "@/lib/enums"
import { RoleGate } from "@/lib/auth"
import { v } from "@/lib/validation"
import { useProduct } from "@/modules/catalog"
import { useSendReminderNow } from "@/modules/reminder"
import { useT } from "@/i18n"
import { useCancelSubscription, useExtendSubscription, useMarkDelivered, useRenewSubscription } from "../hooks/use-subscriptions"
import { PaymentFields, toPaymentInput } from "./payment-fields"

type Sub = SubscriptionListFieldsFragment

function RenewDialog({ sub, open, onOpenChange }: { sub: Sub; open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [{ data }] = useProduct(open ? sub.product.id : null)
  const [renew, renewing] = useRenewSubscription()
  const schema = useMemo(
    () =>
      z.object({
        planId: z.string().nullable(),
        createInvoice: z.boolean(),
        markPaid: z.boolean(),
        pay: z.object({
          method: z.enum(PAYMENT_METHODS as [PaymentMethod, ...PaymentMethod[]]),
          amount: z.number().int().min(0),
          trxId: z.string().nullable().optional(),
          senderNumber: z.string().nullable().optional(),
        }),
      }),
    [],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: { planId: sub.plan?.id ?? null, createInvoice: true, markPaid: false, pay: { method: "BKASH", amount: sub.price, trxId: "", senderNumber: "" } },
  })
  const markPaid = useWatch({ control: form.control, name: "markPaid" })
  const plans = data?.product?.plans.filter((p) => p.isActive) ?? []
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t.subscriptions.renewTitle}
      description={`${sub.customer.name} · ${sub.product.name}`}
      footer={
        <Button type="submit" form="renew-form" disabled={renewing}>
          <RefreshCwIcon /> {t.subscriptions.renew}
        </Button>
      }
    >
      <form
        id="renew-form"
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          const ok = await renew({
            id: sub.id,
            input: {
              planId: values.planId,
              createInvoice: values.createInvoice,
              markPaid: values.markPaid ? toPaymentInput(values.pay) : null,
            },
          })
          if (ok) onOpenChange(false)
        })}
      >
        <FieldGroup>
          <SelectField
            control={form.control}
            name="planId"
            label={t.common.plan}
            options={plans.map((p) => ({ value: p.id, label: `${p.name} · ${t.common.days(p.durationDays)}` }))}
          />
          <SwitchField control={form.control} name="createInvoice" label={t.subscriptions.createInvoice} />
          <SwitchField control={form.control} name="markPaid" label={t.subscriptions.markPaid} />
          {markPaid ? <PaymentFields control={form.control} /> : null}
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}

function ExtendDialog({ sub, open, onOpenChange }: { sub: Sub; open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [extend, busy] = useExtendSubscription()
  const schema = useMemo(() => z.object({ days: v.positiveInt(t), reason: v.optionalString() }), [t])
  const form = useForm<z.infer<typeof schema>>({ resolver: zodResolver(schema), defaultValues: { days: 3, reason: "" } })
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t.subscriptions.extend}
      footer={
        <Button type="submit" form="extend-form" disabled={busy}>
          {t.subscriptions.extend}
        </Button>
      }
    >
      <form
        id="extend-form"
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          if (await extend({ id: sub.id, days: values.days, reason: values.reason || null })) onOpenChange(false)
        })}
      >
        <FieldGroup>
          <NumberField control={form.control} name="days" label={t.subscriptions.extendDays} min={1} required />
          <TextField control={form.control} name="reason" label={t.subscriptions.reason} />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}

function DeliverDialog({ sub, open, onOpenChange }: { sub: Sub; open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [deliver, busy] = useMarkDelivered()
  const form = useForm<{ deliveryNote: string }>({ values: { deliveryNote: sub.deliveryNote ?? "" } })
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t.subscriptions.markDelivered}
      footer={
        <Button type="submit" form="deliver-form" disabled={busy}>
          <CheckCheckIcon /> {t.subscriptions.markDelivered}
        </Button>
      }
    >
      <form
        id="deliver-form"
        onSubmit={form.handleSubmit(async (values) => {
          if (await deliver({ id: sub.id, deliveryNote: values.deliveryNote || null })) onOpenChange(false)
        })}
      >
        <TextareaField control={form.control} name="deliveryNote" label={t.subscriptions.deliveryNote} rows={3} />
      </form>
    </ResponsiveDialog>
  )
}

type DialogKind = "renew" | "extend" | "deliver" | "cancel" | null

/** Row-level actions menu for a subscription (renew, deliver, extend, remind, cancel). */
export function SubscriptionActions({ subscription }: { subscription: Sub }) {
  const t = useT()
  const [dialog, setDialog] = useState<DialogKind>(null)
  const [cancel, cancelling] = useCancelSubscription()
  const [remind] = useSendReminderNow()
  const close = (o: boolean) => !o && setDialog(null)
  const closed = subscription.status === "CANCELLED" || subscription.status === "REPLACED"
  return (
    <div onClick={(e) => e.stopPropagation()}>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon-sm" aria-label={t.common.actions}>
            <MoreVerticalIcon />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem disabled={closed} onSelect={() => setDialog("renew")}>
            <RefreshCwIcon /> {t.subscriptions.renew}
          </DropdownMenuItem>
          {!subscription.deliveredAt ? (
            <DropdownMenuItem onSelect={() => setDialog("deliver")}>
              <CheckCheckIcon /> {t.subscriptions.markDelivered}
            </DropdownMenuItem>
          ) : null}
          <DropdownMenuItem disabled={closed} onSelect={() => void remind({ subscriptionId: subscription.id })}>
            <BellRingIcon /> {t.subscriptions.sendReminder}
          </DropdownMenuItem>
          <RoleGate roles={["OWNER", "ADMIN"]}>
            <DropdownMenuItem disabled={closed} onSelect={() => setDialog("extend")}>
              <CalendarPlusIcon /> {t.subscriptions.extend}
            </DropdownMenuItem>
          </RoleGate>
          <RoleGate roles={["OWNER", "ADMIN", "STAFF"]}>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" disabled={closed} onSelect={() => setDialog("cancel")}>
              <XCircleIcon /> {t.subscriptions.cancel}
            </DropdownMenuItem>
          </RoleGate>
        </DropdownMenuContent>
      </DropdownMenu>
      {dialog === "renew" ? <RenewDialog sub={subscription} open onOpenChange={close} /> : null}
      {dialog === "extend" ? <ExtendDialog sub={subscription} open onOpenChange={close} /> : null}
      {dialog === "deliver" ? <DeliverDialog sub={subscription} open onOpenChange={close} /> : null}
      <ConfirmDialog
        open={dialog === "cancel"}
        onOpenChange={close}
        title={t.subscriptions.cancel}
        description={t.subscriptions.cancelConfirm}
        destructive
        loading={cancelling}
        onConfirm={async () => {
          if (await cancel({ id: subscription.id })) setDialog(null)
        }}
      />
    </div>
  )
}
