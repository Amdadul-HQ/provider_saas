/**
 * Runtime lists of schema enums. Each list is derived from a `Record<Enum, true>` so the
 * compiler fails when the backend adds/removes an enum value — the schema stays the source of truth.
 */
import type {
  ConversationStatus,
  CustomerSource,
  CustomerStatus,
  DeliveryType,
  InvoiceStatus,
  LedgerEntryType,
  MasterAccountStatus,
  MessageKind,
  OrderStatus,
  Platform,
  PaymentMethod,
  PaymentStatus,
  PostStatus,
  ReminderChannel,
  ReminderStatus,
  ReminderTrigger,
  Role,
  SlotStatus,
  SocialAccountStatus,
  SocialPlatform,
  SubscriptionStatus,
  TemplateCategory,
  TicketKind,
  TicketPriority,
  TicketStatus,
  OrderSource,
} from "@/gql/graphql"

function keys<T extends string>(record: Record<T, true>): T[] {
  return Object.keys(record) as T[]
}

export const ROLES = keys<Role>({ OWNER: true, ADMIN: true, STAFF: true, RESELLER: true, VIEWER: true })
export const PLATFORMS = keys<Platform>({ FACEBOOK: true, INSTAGRAM: true, WHATSAPP: true, TELEGRAM: true, SMS: true, EMAIL: true })
export const SOCIAL_PLATFORMS = keys<SocialPlatform>({ FACEBOOK: true, INSTAGRAM: true, WHATSAPP: true, TELEGRAM: true })
export const SOCIAL_ACCOUNT_STATUSES = keys<SocialAccountStatus>({ CONNECTED: true, TOKEN_EXPIRED: true, DISCONNECTED: true, ERROR: true })
export const CUSTOMER_STATUSES = keys<CustomerStatus>({ LEAD: true, ACTIVE: true, INACTIVE: true, BLOCKED: true })
export const CUSTOMER_SOURCES = keys<CustomerSource>({ MANUAL: true, FACEBOOK: true, INSTAGRAM: true, WHATSAPP: true, TELEGRAM: true, STOREFRONT: true, IMPORT: true })
export const DELIVERY_TYPES = keys<DeliveryType>({ SHARED_SLOT: true, PRIVATE_ACCOUNT: true, LICENSE_KEY: true, INVITE_LINK: true, MANUAL: true })
export const MASTER_ACCOUNT_STATUSES = keys<MasterAccountStatus>({ ACTIVE: true, EXPIRING: true, EXPIRED: true, BANNED: true, REPLACED: true, RETIRED: true })
export const SLOT_STATUSES = keys<SlotStatus>({ FREE: true, RESERVED: true, ASSIGNED: true, BANNED: true, RETIRED: true })
export const SUBSCRIPTION_STATUSES = keys<SubscriptionStatus>({ PENDING: true, ACTIVE: true, EXPIRING: true, EXPIRED: true, CANCELLED: true, REPLACED: true })
export const INVOICE_STATUSES = keys<InvoiceStatus>({ DRAFT: true, SENT: true, PARTIAL: true, PAID: true, OVERDUE: true, VOID: true })
export const PAYMENT_METHODS = keys<PaymentMethod>({ BKASH: true, NAGAD: true, ROCKET: true, UPAY: true, BANK: true, CASH: true, CARD: true, OTHER: true })
export const PAYMENT_STATUSES = keys<PaymentStatus>({ PENDING: true, VERIFIED: true, REJECTED: true, REFUNDED: true })
export const LEDGER_ENTRY_TYPES = keys<LedgerEntryType>({ DEBIT: true, CREDIT: true })
export const CONVERSATION_STATUSES = keys<ConversationStatus>({ OPEN: true, PENDING: true, CLOSED: true, SPAM: true })
export const MESSAGE_KINDS = keys<MessageKind>({ TEXT: true, IMAGE: true, VIDEO: true, AUDIO: true, FILE: true, STICKER: true, TEMPLATE: true, SYSTEM: true })
export const TEMPLATE_CATEGORIES = keys<TemplateCategory>({ QUICK_REPLY: true, REMINDER: true, DELIVERY: true, INVOICE: true, WINBACK: true, OTHER: true })
export const POST_STATUSES = keys<PostStatus>({ DRAFT: true, SCHEDULED: true, PUBLISHING: true, PUBLISHED: true, PARTIAL: true, FAILED: true })
export const TICKET_KINDS = keys<TicketKind>({ REPLACEMENT: true, COMPLAINT: true, QUESTION: true, REFUND: true, OTHER: true })
export const TICKET_PRIORITIES = keys<TicketPriority>({ LOW: true, NORMAL: true, HIGH: true, URGENT: true })
export const TICKET_STATUSES = keys<TicketStatus>({ OPEN: true, IN_PROGRESS: true, WAITING_CUSTOMER: true, RESOLVED: true, CLOSED: true })
export const REMINDER_TRIGGERS = keys<ReminderTrigger>({ BEFORE_EXPIRY: true, ON_EXPIRY: true, AFTER_EXPIRY: true, INVOICE_DUE: true, INVOICE_OVERDUE: true, MASTER_EXPIRY: true })
export const REMINDER_CHANNELS = keys<ReminderChannel>({ PREFERRED: true, FACEBOOK: true, INSTAGRAM: true, WHATSAPP: true, TELEGRAM: true, SMS: true, EMAIL: true, IN_APP: true })
export const REMINDER_STATUSES = keys<ReminderStatus>({ SCHEDULED: true, SENT: true, FAILED: true, SKIPPED: true, CANCELLED: true })
export const ORDER_STATUSES = keys<OrderStatus>({ NEW: true, AWAITING_PAYMENT: true, PAID: true, DELIVERED: true, CANCELLED: true })
export const ORDER_SOURCES = keys<OrderSource>({ STOREFRONT: true, INBOX: true, MANUAL: true, PORTAL: true })

/** Mobile wallet methods that carry a TrxID. */
export const MOBILE_WALLETS: PaymentMethod[] = ["BKASH", "NAGAD", "ROCKET", "UPAY"]
