"use client"

import { useMemo, useState } from "react"
import Link from "next/link"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { LockIcon, ReplaceIcon, SendIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { DateText, DetailRow, EmptyState, ErrorState, NumberField, PageSkeleton, ResponsiveDialog, SectionCard, SelectField, StatusBadge } from "@/components/shared"
import type { SlotStatus, TicketPriority, TicketQuery, TicketStatus } from "@/gql/graphql"
import { SLOT_STATUSES, TICKET_PRIORITIES, TICKET_STATUSES } from "@/lib/enums"
import { SlotSelect } from "@/modules/inventory"
import { v } from "@/lib/validation"
import { enumLabel, useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { useAddTicketComment, useResolveReplacement, useTicket, useUpdateTicket } from "../hooks/use-tickets"

type Ticket = NonNullable<TicketQuery["ticket"]>

function ResolveReplacementDialog({ ticket, open, onOpenChange }: { ticket: Ticket; open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT()
  const [resolve, resolving] = useResolveReplacement()
  const [newSlotId, setNewSlotId] = useState<string | null>(null)
  const schema = useMemo(
    () => z.object({ markOldSlot: z.enum(SLOT_STATUSES as [SlotStatus, ...SlotStatus[]]), extendDays: v.optionalInt(t) }),
    [t],
  )
  type Values = z.infer<typeof schema>
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { markOldSlot: "BANNED", extendDays: 0 } })
  const productId = ticket.subscription?.product.id
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t.tickets.resolveTitle}
      description={t.tickets.resolveHint}
      footer={
        <Button type="submit" form="resolve-replacement" disabled={resolving}>
          <ReplaceIcon /> {t.tickets.resolveReplacement}
        </Button>
      }
    >
      <form
        id="resolve-replacement"
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          const ok = await resolve({ ticketId: ticket.id, newSlotId, markOldSlot: values.markOldSlot, extendDays: values.extendDays ?? 0 })
          if (ok) onOpenChange(false)
        })}
      >
        <FieldGroup>
          {ticket.subscription?.slot ? (
            <p className="rounded-lg bg-muted p-3 text-sm">
              {t.tickets.oldSlot}: <strong>{t.inventory.slotNo(ticket.subscription.slot.slotNo)}</strong> {ticket.subscription.slot.label ?? ""}
            </p>
          ) : null}
          <Field>
            <FieldLabel htmlFor="resolve-slot">{t.tickets.newSlot}</FieldLabel>
            <SlotSelect id="resolve-slot" productId={productId} value={newSlotId} onChange={setNewSlotId} />
          </Field>
          <SelectField
            control={form.control}
            name="markOldSlot"
            label={t.tickets.markOldSlot}
            options={SLOT_STATUSES.map((s) => ({ value: s, label: enumLabel(t, "SlotStatus", s) }))}
          />
          <NumberField control={form.control} name="extendDays" label={t.tickets.extendDays} min={0} />
        </FieldGroup>
      </form>
    </ResponsiveDialog>
  )
}

function Comments({ ticket }: { ticket: Ticket }) {
  const t = useT()
  const [add, adding] = useAddTicketComment()
  const [body, setBody] = useState("")
  const [internal, setInternal] = useState(true)
  return (
    <SectionCard title={t.tickets.comments}>
      <div className="space-y-3">
        {ticket.comments.length === 0 ? <p className="text-sm text-muted-foreground">{t.tickets.noComments}</p> : null}
        <ul className="space-y-2">
          {ticket.comments.map((c) => (
            <li key={c.id} className={cn("rounded-lg border p-3", c.isInternal && "border-dashed bg-muted/40")}>
              <p className="text-sm whitespace-pre-wrap">{c.body}</p>
              <p className="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
                {c.isInternal ? <LockIcon className="size-3" /> : null}
                {c.author?.name ?? "—"} · <DateText value={c.createdAt} withTime />
              </p>
            </li>
          ))}
        </ul>
        <form
          className="space-y-2"
          onSubmit={async (e) => {
            e.preventDefault()
            if (body.trim() && (await add({ ticketId: ticket.id, body: body.trim(), isInternal: internal }))) setBody("")
          }}
        >
          <Textarea value={body} onChange={(e) => setBody(e.target.value)} placeholder={t.tickets.addComment} aria-label={t.tickets.addComment} rows={2} />
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Checkbox id="comment-internal" checked={internal} onCheckedChange={(c) => setInternal(c === true)} />
              <Label htmlFor="comment-internal" className="text-sm font-normal">
                {t.tickets.internal}
              </Label>
            </div>
            <Button type="submit" size="sm" disabled={adding || !body.trim()}>
              <SendIcon /> {t.common.send}
            </Button>
          </div>
        </form>
      </div>
    </SectionCard>
  )
}

export function TicketDetail({ ticketId }: { ticketId: string }) {
  const t = useT()
  const [{ data, fetching, error }, refetch] = useTicket(ticketId)
  const [update] = useUpdateTicket()
  const [resolveOpen, setResolveOpen] = useState(false)
  const ticket = data?.ticket
  if (!ticket) {
    if (error) return <ErrorState error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
    return fetching ? <PageSkeleton /> : <EmptyState title={t.tickets.notFound} />
  }
  const open = ticket.status !== "RESOLVED" && ticket.status !== "CLOSED"
  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="space-y-1">
          <h1 className="font-heading text-xl font-semibold">
            <span className="text-muted-foreground">#{ticket.number}</span> {ticket.title}
          </h1>
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge enum="TicketStatus" value={ticket.status} />
            <StatusBadge enum="TicketPriority" value={ticket.priority} />
            <span className="text-sm text-muted-foreground">{enumLabel(t, "TicketKind", ticket.kind)}</span>
          </div>
        </div>
        {ticket.kind === "REPLACEMENT" && open ? (
          <Button onClick={() => setResolveOpen(true)}>
            <ReplaceIcon /> {t.tickets.resolveReplacement}
          </Button>
        ) : null}
      </div>
      <div className="grid gap-4 lg:grid-cols-[1fr_20rem]">
        <div className="space-y-4">
          {ticket.description || ticket.reason ? (
            <SectionCard title={t.tickets.description}>
              {ticket.reason ? (
                <p className="mb-2 text-sm">
                  <span className="text-muted-foreground">{t.tickets.reason}:</span> {ticket.reason}
                </p>
              ) : null}
              {ticket.description ? <p className="text-sm whitespace-pre-wrap">{ticket.description}</p> : null}
            </SectionCard>
          ) : null}
          <Comments ticket={ticket} />
        </div>
        <SectionCard>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="ticket-status">{t.common.status}</Label>
              <Select value={ticket.status} onValueChange={(s) => void update({ id: ticket.id, input: { status: s as TicketStatus } })}>
                <SelectTrigger id="ticket-status" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TICKET_STATUSES.map((s) => (
                    <SelectItem key={s} value={s}>
                      {enumLabel(t, "TicketStatus", s)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="ticket-priority">{t.tickets.priority}</Label>
              <Select value={ticket.priority} onValueChange={(p) => void update({ id: ticket.id, input: { priority: p as TicketPriority } })}>
                <SelectTrigger id="ticket-priority" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TICKET_PRIORITIES.map((p) => (
                    <SelectItem key={p} value={p}>
                      {enumLabel(t, "TicketPriority", p)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <dl className="divide-y pt-2">
              <DetailRow label={t.common.customer}>
                <Link href={`/customers/${ticket.customer.id}`} className="hover:underline">
                  {ticket.customer.name}
                </Link>
              </DetailRow>
              {ticket.subscription ? (
                <DetailRow label={t.tickets.subscription}>
                  {ticket.subscription.product.name}
                  {ticket.subscription.slot ? ` · ${t.inventory.slotNo(ticket.subscription.slot.slotNo)}` : ""}
                </DetailRow>
              ) : null}
              {ticket.oldSlot ? <DetailRow label={t.tickets.oldSlot}>{t.inventory.slotNo(ticket.oldSlot.slotNo)}</DetailRow> : null}
              {ticket.newSlot ? <DetailRow label={t.tickets.newSlot}>{t.inventory.slotNo(ticket.newSlot.slotNo)}</DetailRow> : null}
              <DetailRow label={t.tickets.assignedTo}>{ticket.assignedTo?.name ?? "—"}</DetailRow>
              <DetailRow label={t.common.createdAt}>
                <DateText value={ticket.createdAt} withTime />
              </DetailRow>
              {ticket.resolvedAt ? (
                <DetailRow label={enumLabel(t, "TicketStatus", "RESOLVED")}>
                  <DateText value={ticket.resolvedAt} withTime />
                </DetailRow>
              ) : null}
            </dl>
          </div>
        </SectionCard>
      </div>
      {resolveOpen ? <ResolveReplacementDialog ticket={ticket} open={resolveOpen} onOpenChange={setResolveOpen} /> : null}
    </div>
  )
}
