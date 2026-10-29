"use client"

import { useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { PencilIcon, PlusIcon, Trash2Icon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { ConfirmDialog, EmptyState, MoneyField, MoneyText, NumberField, ResponsiveDialog, SwitchField, TextField, ToneBadge } from "@/components/shared"
import type { PlanFieldsFragment } from "@/gql/graphql"
import { RoleGate } from "@/lib/auth"
import { v } from "@/lib/validation"
import { useT } from "@/i18n"
import { useDeletePlan, useUpsertPlan } from "../hooks/use-catalog"

function PlanDialog({
  productId,
  plan,
  open,
  onOpenChange,
}: {
  productId: string
  plan: PlanFieldsFragment | null
  open: boolean
  onOpenChange: (o: boolean) => void
}) {
  const t = useT()
  const [upsert, saving] = useUpsertPlan()
  const schema = useMemo(
    () =>
      z.object({
        name: v.requiredString(t),
        durationDays: v.positiveInt(t),
        price: v.money(t),
        cost: v.optionalMoney(t),
        isActive: z.boolean(),
        sortOrder: v.optionalInt(t),
      }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: {
      name: plan?.name ?? "",
      durationDays: plan?.durationDays ?? 30,
      price: plan?.price ?? 0,
      cost: plan?.cost ?? 0,
      isActive: plan?.isActive ?? true,
      sortOrder: plan?.sortOrder ?? 0,
    },
  })
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={plan ? `${t.common.edit}: ${plan.name}` : t.catalog.addPlan}
      footer={
        <Button type="submit" form="plan-form" disabled={saving}>
          {saving ? t.common.saving : t.common.save}
        </Button>
      }
    >
      <form
        id="plan-form"
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          const ok = await upsert({
            productId,
            id: plan?.id ?? null,
            input: { ...values, cost: values.cost ?? 0, sortOrder: values.sortOrder ?? 0 },
          })
          if (ok) onOpenChange(false)
        })}
      >
        <FieldGroup>
          <TextField control={form.control} name="name" label={t.catalog.planName} placeholder={t.catalog.planNamePlaceholder} required />
          <div className="grid grid-cols-2 gap-4">
            <NumberField control={form.control} name="durationDays" label={t.catalog.durationDays} min={1} required />
            <NumberField control={form.control} name="sortOrder" label="#" min={0} />
            <MoneyField control={form.control} name="price" label={t.common.price} required />
            <MoneyField control={form.control} name="cost" label={t.common.cost} />
          </div>
          <SwitchField control={form.control} name="isActive" label={t.common.active} />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}

/** Inline list of a product's plans with add/edit/delete (owner/admin). */
export function PlansEditor({ productId, plans }: { productId: string; plans: PlanFieldsFragment[] }) {
  const t = useT()
  const [editing, setEditing] = useState<PlanFieldsFragment | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [deleting, setDeleting] = useState<PlanFieldsFragment | null>(null)
  const [deletePlan, deletingBusy] = useDeletePlan()
  const sorted = [...plans].sort((a, b) => a.sortOrder - b.sortOrder || a.durationDays - b.durationDays)

  return (
    <div className="space-y-3">
      {sorted.length === 0 ? (
        <EmptyState compact title={t.catalog.noPlans} />
      ) : (
        <ul className="divide-y rounded-lg border">
          {sorted.map((p) => (
            <li key={p.id} className="flex items-center gap-3 p-3">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <p className="font-medium">{p.name}</p>
                  {!p.isActive ? <ToneBadge>{t.common.inactive}</ToneBadge> : null}
                </div>
                <p className="text-xs text-muted-foreground">
                  {t.common.days(p.durationDays)} · {t.common.cost}: <MoneyText amount={p.cost} currency={p.currency} tone="muted" className="font-normal" />
                </p>
              </div>
              <div className="text-right">
                <MoneyText amount={p.price} currency={p.currency} tone="money" />
                <p className="text-xs text-success">
                  +<MoneyText amount={p.price - p.cost} currency={p.currency} className="font-normal" />
                </p>
              </div>
              <RoleGate roles={["OWNER", "ADMIN"]}>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={t.common.edit}
                  onClick={() => {
                    setEditing(p)
                    setDialogOpen(true)
                  }}
                >
                  <PencilIcon />
                </Button>
                <Button variant="ghost" size="icon-sm" aria-label={t.common.delete} onClick={() => setDeleting(p)}>
                  <Trash2Icon />
                </Button>
              </RoleGate>
            </li>
          ))}
        </ul>
      )}
      <RoleGate roles={["OWNER", "ADMIN"]}>
        <Button
          variant="outline"
          onClick={() => {
            setEditing(null)
            setDialogOpen(true)
          }}
        >
          <PlusIcon /> {t.catalog.addPlan}
        </Button>
      </RoleGate>
      <PlanDialog productId={productId} plan={editing} open={dialogOpen} onOpenChange={setDialogOpen} />
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(o) => !o && setDeleting(null)}
        title={t.common.delete}
        description={deleting?.name}
        destructive
        loading={deletingBusy}
        onConfirm={async () => {
          if (deleting && (await deletePlan({ id: deleting.id }))) setDeleting(null)
        }}
      />
    </div>
  )
}
