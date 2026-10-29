"use client"

import { useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { KeyRoundIcon, PencilIcon, EyeIcon, UserIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { Progress } from "@/components/ui/progress"
import {
  DateText,
  DaysLeft,
  ErrorState,
  ListSkeleton,
  MoneyText,
  ResponsiveDialog,
  SectionCard,
  SelectField,
  StatusBadge,
  TextField,
  toneFor,
} from "@/components/shared"
import type { SlotFieldsFragment, SlotStatus } from "@/gql/graphql"
import { SLOT_STATUSES } from "@/lib/enums"
import { RoleGate } from "@/lib/auth"
import { enumLabel, useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { useMasterAccount, useUpdateSlot } from "../hooks/use-inventory"
import { CredentialRevealDialog, SetCredentialDialog } from "./credential-dialogs"
import { MasterAccountFormDialog } from "./master-account-form-dialog"

const tileTone: Record<string, string> = {
  success: "border-success/40 bg-success/8",
  primary: "border-primary/40 bg-primary/8",
  info: "border-info/40 bg-info/8",
  danger: "border-destructive/40 bg-destructive/8",
  neutral: "border-border bg-muted/60 opacity-70",
  warning: "border-warning/50 bg-warning/10",
}

function SlotDialog({ slot, open, onOpenChange }: { slot: SlotFieldsFragment | null; open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [save, saving] = useUpdateSlot()
  const schema = useMemo(
    () => z.object({ label: z.string().optional().nullable(), pin: z.string().optional().nullable(), status: z.enum(SLOT_STATUSES as [SlotStatus, ...SlotStatus[]]) }),
    [],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: { label: slot?.label ?? "", pin: slot?.pin ?? "", status: slot?.status ?? "FREE" },
  })
  if (!slot) return null
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={`${t.inventory.editSlot} · ${t.inventory.slotNo(slot.slotNo)}`}
      footer={
        <Button type="submit" form="slot-form" disabled={saving}>
          {t.common.save}
        </Button>
      }
    >
      <form
        id="slot-form"
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          if (await save({ id: slot.id, label: values.label || null, pin: values.pin || null, status: values.status })) onOpenChange(false)
        })}
      >
        <FieldGroup>
          {slot.subscription ? (
            <p className="flex items-center gap-2 rounded-lg bg-muted p-3 text-sm">
              <UserIcon className="size-4" /> {slot.subscription.customer.name} · <DaysLeft days={slot.subscription.daysLeft} />
            </p>
          ) : null}
          <TextField control={form.control} name="label" label={t.inventory.slotLabel} />
          <TextField control={form.control} name="pin" label={t.inventory.pin} inputMode="numeric" />
          <SelectField
            control={form.control}
            name="status"
            label={t.common.status}
            options={SLOT_STATUSES.map((s) => ({ value: s, label: enumLabel(t, "SlotStatus", s) }))}
          />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}

/** Master account detail: summary, credential actions and a tappable slot grid. */
export function MasterAccountSlots({ accountId }: { accountId: string }) {
  const t = useT()
  const [{ data, fetching, error }, refetch] = useMasterAccount(accountId)
  const [slot, setSlot] = useState<SlotFieldsFragment | null>(null)
  const [slotOpen, setSlotOpen] = useState(false)
  const [revealOpen, setRevealOpen] = useState(false)
  const [setCredOpen, setSetCredOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const account = data?.masterAccount

  if (!account) {
    if (error) return <ErrorState error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
    return fetching ? <ListSkeleton rows={3} /> : <ErrorState error={new Error(t.errors.notFound)} />
  }
  const used = account.totalSlots - account.freeSlots
  const slots = [...account.slots].sort((a, b) => a.slotNo - b.slotNo)

  return (
    <div className="space-y-4">
      <SectionCard
        title={
          <span className="flex items-center gap-2">
            {account.label} <StatusBadge enum="MasterAccountStatus" value={account.status} />
          </span>
        }
        actions={
          <RoleGate roles={["OWNER", "ADMIN"]}>
            <Button variant="ghost" size="icon-sm" aria-label={t.common.edit} onClick={() => setEditOpen(true)}>
              <PencilIcon />
            </Button>
          </RoleGate>
        }
      >
        <div className="grid gap-3 text-sm sm:grid-cols-2">
          <div>
            <p className="text-xs text-muted-foreground">{t.common.product}</p>
            <p className="font-medium">{account.product.name}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">{t.inventory.supplier}</p>
            <p className="font-medium">{account.supplier?.name ?? "—"}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">{t.inventory.purchaseCost}</p>
            <MoneyText amount={account.purchaseCost} currency={account.currency} />
          </div>
          <div>
            <p className="text-xs text-muted-foreground">{t.inventory.expiresAt}</p>
            <DateText value={account.expiresAt} className="font-medium" />
          </div>
          <div className="sm:col-span-2">
            <div className="mb-1 flex justify-between text-xs text-muted-foreground">
              <span>{t.inventory.slots}</span>
              <span>{t.inventory.occupancy(used, account.totalSlots)}</span>
            </div>
            <Progress value={account.totalSlots ? (used / account.totalSlots) * 100 : 0} />
          </div>
          <div className="flex flex-wrap items-center gap-2 sm:col-span-2">
            <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <KeyRoundIcon className="size-3.5" />
              {account.loginHint ?? "—"} · {account.hasCredential ? t.inventory.hasCredential : t.inventory.noCredential}
            </span>
            <RoleGate roles={["OWNER", "ADMIN"]}>
              {account.hasCredential ? (
                <Button size="sm" variant="outline" onClick={() => setRevealOpen(true)}>
                  <EyeIcon /> {t.inventory.reveal}
                </Button>
              ) : null}
              <Button size="sm" variant="outline" onClick={() => setSetCredOpen(true)}>
                <KeyRoundIcon /> {t.inventory.setCredential}
              </Button>
            </RoleGate>
          </div>
        </div>
      </SectionCard>

      <ul className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4" aria-label={t.inventory.slots}>
        {slots.map((s) => (
          <li key={s.id}>
            <button
              type="button"
              onClick={() => {
                setSlot(s)
                setSlotOpen(true)
              }}
              className={cn("flex min-h-24 w-full flex-col gap-1 rounded-xl border-2 p-3 text-left transition-transform active:scale-[0.98]", tileTone[toneFor("SlotStatus", s.status)])}
            >
              <div className="flex items-center justify-between gap-1">
                <span className="text-xs font-semibold">{t.inventory.slotNo(s.slotNo)}</span>
                <StatusBadge enum="SlotStatus" value={s.status} className="h-5 px-1.5 text-[10px]" />
              </div>
              <span className="truncate text-sm font-medium">{s.subscription?.customer.name ?? s.label ?? t.inventory.slotEmpty}</span>
              {s.subscription ? <DaysLeft days={s.subscription.daysLeft} /> : s.label ? <span className="text-xs text-muted-foreground">{s.label}</span> : null}
              {s.pin ? <span className="font-mono text-[11px] text-muted-foreground">PIN {s.pin}</span> : null}
            </button>
          </li>
        ))}
      </ul>

      <SlotDialog slot={slot} open={slotOpen} onOpenChange={setSlotOpen} />
      <CredentialRevealDialog accountId={account.id} accountLabel={account.label} open={revealOpen} onOpenChange={setRevealOpen} />
      <SetCredentialDialog accountId={account.id} open={setCredOpen} onOpenChange={setSetCredOpen} />
      <MasterAccountFormDialog open={editOpen} onOpenChange={setEditOpen} account={account} />
    </div>
  )
}
