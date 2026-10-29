"use client"

import { cn } from "@/lib/utils"
import { enumLabel, useT, type EnumName, type EnumValue } from "@/i18n"

export type Tone = "success" | "warning" | "danger" | "info" | "neutral" | "primary"

const toneClass: Record<Tone, string> = {
  success: "bg-success/12 text-success ring-success/25",
  warning: "bg-warning/18 text-warning-foreground ring-warning/40 dark:text-warning",
  danger: "bg-destructive/10 text-destructive ring-destructive/25",
  info: "bg-info/10 text-info ring-info/25",
  neutral: "bg-muted text-muted-foreground ring-border",
  primary: "bg-primary/10 text-primary ring-primary/25",
}

/** Enum value → semantic tone. Unknown values fall back to neutral. */
const TONES: { [E in EnumName]?: Partial<Record<EnumValue<E>, Tone>> } = {
  CustomerStatus: { LEAD: "info", ACTIVE: "success", INACTIVE: "neutral", BLOCKED: "danger" },
  SubscriptionStatus: { PENDING: "info", ACTIVE: "success", EXPIRING: "warning", EXPIRED: "danger", CANCELLED: "neutral", REPLACED: "neutral" },
  InvoiceStatus: { DRAFT: "neutral", SENT: "info", PARTIAL: "warning", PAID: "success", OVERDUE: "danger", VOID: "neutral" },
  PaymentStatus: { PENDING: "warning", VERIFIED: "success", REJECTED: "danger", REFUNDED: "neutral" },
  MasterAccountStatus: { ACTIVE: "success", EXPIRING: "warning", EXPIRED: "danger", BANNED: "danger", REPLACED: "neutral", RETIRED: "neutral" },
  SlotStatus: { FREE: "success", RESERVED: "info", ASSIGNED: "primary", BANNED: "danger", RETIRED: "neutral" },
  SocialAccountStatus: { CONNECTED: "success", TOKEN_EXPIRED: "warning", DISCONNECTED: "neutral", ERROR: "danger" },
  ConversationStatus: { OPEN: "success", PENDING: "warning", CLOSED: "neutral", SPAM: "danger" },
  PostStatus: { DRAFT: "neutral", SCHEDULED: "info", PUBLISHING: "info", PUBLISHED: "success", PARTIAL: "warning", FAILED: "danger" },
  TicketStatus: { OPEN: "info", IN_PROGRESS: "primary", WAITING_CUSTOMER: "warning", RESOLVED: "success", CLOSED: "neutral" },
  TicketPriority: { LOW: "neutral", NORMAL: "info", HIGH: "warning", URGENT: "danger" },
  ReminderStatus: { SCHEDULED: "info", SENT: "success", FAILED: "danger", SKIPPED: "neutral", CANCELLED: "neutral" },
  OrderStatus: { NEW: "info", AWAITING_PAYMENT: "warning", PAID: "primary", DELIVERED: "success", CANCELLED: "neutral" },
  LedgerEntryType: { DEBIT: "danger", CREDIT: "success" },
}

export function toneFor<E extends EnumName>(name: E, value: EnumValue<E>): Tone {
  const map = TONES[name] as Partial<Record<string, Tone>> | undefined
  return map?.[value] ?? "neutral"
}

export function ToneBadge({ tone = "neutral", children, className }: { tone?: Tone; children: React.ReactNode; className?: string }) {
  return (
    <span
      className={cn(
        "inline-flex h-6 items-center gap-1 rounded-full px-2.5 text-xs font-medium whitespace-nowrap ring-1 ring-inset",
        toneClass[tone],
        className,
      )}
    >
      {children}
    </span>
  )
}

/** `<StatusBadge enum="InvoiceStatus" value={invoice.status} />` — label from i18n, colour from TONES. */
export function StatusBadge<E extends EnumName>({
  enum: name,
  value,
  className,
}: {
  enum: E
  value: EnumValue<E> | null | undefined
  className?: string
}) {
  const t = useT()
  if (!value) return null
  return (
    <ToneBadge tone={toneFor(name, value)} className={className}>
      <span className="size-1.5 rounded-full bg-current opacity-70" aria-hidden />
      {enumLabel(t, name, value)}
    </ToneBadge>
  )
}
