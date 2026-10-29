"use client"

import { useMemo } from "react"
import { Controller, useForm, useWatch } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { Button } from "@/components/ui/button"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { ResponsiveDialog, SelectField, TextareaField, TextField } from "@/components/shared"
import type { TicketKind, TicketPriority } from "@/gql/graphql"
import { TICKET_KINDS, TICKET_PRIORITIES } from "@/lib/enums"
import { v } from "@/lib/validation"
import { CustomerPicker, useCustomer, type PickedCustomer } from "@/modules/customer"
import { enumLabel, useT } from "@/i18n"
import { useCreateTicket } from "../hooks/use-tickets"

export function CreateTicketDialog({
  open,
  onOpenChange,
  defaultCustomer,
  conversationId,
  onCreated,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  defaultCustomer?: PickedCustomer | null
  conversationId?: string | null
  onCreated?: (ticketId: string) => void
}) {
  const t = useT()
  const [create, saving] = useCreateTicket()
  const schema = useMemo(
    () =>
      z
        .object({
          customer: z.object({ id: z.string(), name: z.string(), phone: z.string().nullable().optional() }).nullable(),
          subscriptionId: z.string().nullable().optional(),
          kind: z.enum(TICKET_KINDS as [TicketKind, ...TicketKind[]]),
          priority: z.enum(TICKET_PRIORITIES as [TicketPriority, ...TicketPriority[]]),
          title: v.requiredString(t),
          description: v.optionalString(),
          reason: v.optionalString(),
        })
        .refine((d) => d.customer !== null, { path: ["customer"], message: t.errors.required }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: { customer: defaultCustomer ?? null, subscriptionId: null, kind: "REPLACEMENT", priority: "NORMAL", title: "", description: "", reason: "" },
  })
  const customer = useWatch({ control: form.control, name: "customer" })
  const [{ data: customerData }] = useCustomer(customer?.id ?? "")
  const subs = customer ? (customerData?.customer?.subscriptions ?? []) : []

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t.tickets.new}
      footer={
        <Button type="submit" form="create-ticket" disabled={saving}>
          {t.common.create}
        </Button>
      }
    >
      <form
        id="create-ticket"
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          if (!values.customer) return
          const res = await create({
            input: {
              customerId: values.customer.id,
              subscriptionId: values.subscriptionId || null,
              conversationId: conversationId ?? null,
              kind: values.kind,
              priority: values.priority,
              title: values.title,
              description: values.description || null,
              reason: values.reason || null,
            },
          })
          if (res) {
            onCreated?.(res.createTicket.id)
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
                <CustomerPicker
                  value={field.value}
                  onChange={(c) => {
                    field.onChange(c)
                    form.setValue("subscriptionId", null)
                  }}
                  invalid={fieldState.invalid}
                />
                <FieldError errors={[fieldState.error]} />
              </Field>
            )}
          />
          {customer ? (
            <SelectField
              control={form.control}
              name="subscriptionId"
              label={t.tickets.subscription}
              emptyLabel={t.common.none}
              options={subs.map((s) => ({ value: s.id, label: `${s.product.name}${s.slot ? ` · ${t.inventory.slotNo(s.slot.slotNo)}` : ""}` }))}
            />
          ) : null}
          <div className="grid grid-cols-2 gap-4">
            <SelectField control={form.control} name="kind" label={t.tickets.kind} options={TICKET_KINDS.map((k) => ({ value: k, label: enumLabel(t, "TicketKind", k) }))} />
            <SelectField
              control={form.control}
              name="priority"
              label={t.tickets.priority}
              options={TICKET_PRIORITIES.map((k) => ({ value: k, label: enumLabel(t, "TicketPriority", k) }))}
            />
          </div>
          <TextField control={form.control} name="title" label={t.tickets.ticketTitle} required />
          <TextField control={form.control} name="reason" label={t.tickets.reason} placeholder={t.tickets.reasonPlaceholder} />
          <TextareaField control={form.control} name="description" label={t.tickets.description} rows={3} />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}
