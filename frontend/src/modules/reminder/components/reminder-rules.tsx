"use client"

import { useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import type { ColumnDef } from "@tanstack/react-table"
import { BellRingIcon, PlusIcon, Trash2Icon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { ConfirmDialog, DataTable, EmptyState, NumberField, ResponsiveDialog, SelectField, SwitchField, TextField, ToneBadge } from "@/components/shared"
import type { ReminderChannel, ReminderRuleFieldsFragment, ReminderTrigger } from "@/gql/graphql"
import { REMINDER_CHANNELS, REMINDER_TRIGGERS } from "@/lib/enums"
import { RoleGate } from "@/lib/auth"
import { v } from "@/lib/validation"
import { useProducts } from "@/modules/catalog"
import { useMessageTemplates } from "@/modules/inbox"
import { enumLabel, useT } from "@/i18n"
import { useDeleteReminderRule, useReminderRules, useUpsertReminderRule } from "../hooks/use-reminders"

function RuleDialog({ rule, open, onOpenChange }: { rule: ReminderRuleFieldsFragment | null; open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [upsert, saving] = useUpsertReminderRule()
  const [{ data: templates }] = useMessageTemplates()
  const [{ data: products }] = useProducts({ isActive: true }, { first: 100 })
  const schema = useMemo(
    () =>
      z.object({
        name: v.requiredString(t),
        trigger: z.enum(REMINDER_TRIGGERS as [ReminderTrigger, ...ReminderTrigger[]]),
        offsetDays: v.optionalInt(t),
        sendHour: z.number().int().min(0).max(23),
        channel: z.enum(REMINDER_CHANNELS as [ReminderChannel, ...ReminderChannel[]]),
        templateId: z.string().nullable().optional(),
        productId: z.string().nullable().optional(),
        isActive: z.boolean(),
      }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: {
      name: rule?.name ?? "",
      trigger: rule?.trigger ?? "BEFORE_EXPIRY",
      offsetDays: rule?.offsetDays ?? 3,
      sendHour: rule?.sendHour ?? 10,
      channel: rule?.channel ?? "PREFERRED",
      templateId: rule?.template?.id ?? null,
      productId: rule?.productId ?? null,
      isActive: rule?.isActive ?? true,
    },
  })
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={rule ? t.reminders.editRule : t.reminders.newRule}
      footer={
        <Button type="submit" form="rule-form" disabled={saving}>
          {t.common.save}
        </Button>
      }
    >
      <form
        id="rule-form"
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          const ok = await upsert({
            id: rule?.id ?? null,
            input: { ...values, offsetDays: values.offsetDays ?? 0, templateId: values.templateId || null, productId: values.productId || null },
          })
          if (ok) onOpenChange(false)
        })}
      >
        <FieldGroup>
          <TextField control={form.control} name="name" label={t.common.name} required />
          <SelectField
            control={form.control}
            name="trigger"
            label={t.reminders.trigger}
            options={REMINDER_TRIGGERS.map((x) => ({ value: x, label: enumLabel(t, "ReminderTrigger", x) }))}
          />
          <div className="grid grid-cols-2 gap-4">
            <NumberField control={form.control} name="offsetDays" label={t.reminders.offsetDays} min={0} />
            <NumberField control={form.control} name="sendHour" label={t.reminders.sendHour} min={0} max={23} />
          </div>
          <SelectField
            control={form.control}
            name="channel"
            label={t.reminders.channel}
            options={REMINDER_CHANNELS.map((x) => ({ value: x, label: enumLabel(t, "ReminderChannel", x) }))}
          />
          <SelectField
            control={form.control}
            name="templateId"
            label={t.reminders.template}
            emptyLabel={t.common.none}
            options={(templates?.messageTemplates ?? []).map((tpl) => ({ value: tpl.id, label: tpl.name }))}
          />
          <SelectField
            control={form.control}
            name="productId"
            label={t.reminders.productScope}
            emptyLabel={t.reminders.allProducts}
            options={(products?.products ?? []).map((p) => ({ value: p.id, label: p.name }))}
          />
          <SwitchField control={form.control} name="isActive" label={t.common.active} />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}

export function ReminderRules() {
  const t = useT()
  const [{ data, fetching, error }, refetch] = useReminderRules()
  const [editing, setEditing] = useState<ReminderRuleFieldsFragment | null>(null)
  const [open, setOpen] = useState(false)
  const [deleting, setDeleting] = useState<ReminderRuleFieldsFragment | null>(null)
  const [del, delBusy] = useDeleteReminderRule()
  const openEditor = (r: ReminderRuleFieldsFragment | null) => {
    setEditing(r)
    setOpen(true)
  }
  const columns = useMemo<ColumnDef<ReminderRuleFieldsFragment>[]>(
    () => [
      { id: "name", header: t.common.name, cell: ({ row }) => <span className="font-medium">{row.original.name}</span> },
      {
        id: "trigger",
        header: t.reminders.trigger,
        cell: ({ row }) => (
          <div>
            <p>{enumLabel(t, "ReminderTrigger", row.original.trigger)}</p>
            <p className="text-xs text-muted-foreground">{t.reminders.ruleSummary(row.original.offsetDays, row.original.sendHour)}</p>
          </div>
        ),
      },
      { id: "channel", header: t.reminders.channel, cell: ({ row }) => enumLabel(t, "ReminderChannel", row.original.channel) },
      { id: "template", header: t.reminders.template, cell: ({ row }) => row.original.template?.name ?? "—" },
      { id: "active", header: t.common.status, cell: ({ row }) => <ToneBadge tone={row.original.isActive ? "success" : "neutral"}>{row.original.isActive ? t.common.active : t.common.inactive}</ToneBadge> },
      {
        id: "actions",
        header: "",
        cell: ({ row }) => (
          <RoleGate roles={["OWNER", "ADMIN"]}>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={t.common.delete}
              onClick={(e) => {
                e.stopPropagation()
                setDeleting(row.original)
              }}
            >
              <Trash2Icon />
            </Button>
          </RoleGate>
        ),
      },
    ],
    [t],
  )
  return (
    <div className="space-y-3">
      <RoleGate roles={["OWNER", "ADMIN"]}>
        <Button onClick={() => openEditor(null)}>
          <PlusIcon /> {t.reminders.newRule}
        </Button>
      </RoleGate>
      <DataTable
        columns={columns}
        data={data?.reminderRules}
        fetching={fetching}
        error={error}
        onRetry={() => refetch({ requestPolicy: "network-only" })}
        onRowClick={openEditor}
        empty={<EmptyState icon={BellRingIcon} title={t.reminders.rulesEmpty} description={t.reminders.rulesEmptyHint} />}
        mobileCard={(r) => (
          <div className="space-y-1">
            <div className="flex items-center justify-between gap-2">
              <p className="font-medium">{r.name}</p>
              <ToneBadge tone={r.isActive ? "success" : "neutral"}>{r.isActive ? t.common.active : t.common.inactive}</ToneBadge>
            </div>
            <p className="text-xs text-muted-foreground">
              {enumLabel(t, "ReminderTrigger", r.trigger)} · {t.reminders.ruleSummary(r.offsetDays, r.sendHour)} · {enumLabel(t, "ReminderChannel", r.channel)}
            </p>
          </div>
        )}
      />
      <RuleDialog rule={editing} open={open} onOpenChange={setOpen} />
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(o) => !o && setDeleting(null)}
        title={t.common.delete}
        description={t.reminders.deleteConfirm}
        destructive
        loading={delBusy}
        onConfirm={async () => {
          if (deleting && (await del({ id: deleting.id }))) setDeleting(null)
        }}
      />
    </div>
  )
}
