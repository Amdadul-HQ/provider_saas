package model

// Hand-written GraphQL models (bound in gqlgen.yml). They carry the foreign
// keys that field resolvers (@goField(forceResolver: true)) need to load
// related objects through dataloaders. Everything else is generated in
// models_gen.go.

import (
	"time"

	"github.com/google/uuid"
)

type Subscription struct {
	ID            uuid.UUID          `json:"id"`
	CustomerID    uuid.UUID          `json:"-"`
	ProductID     uuid.UUID          `json:"-"`
	PlanID        *uuid.UUID         `json:"-"`
	SlotID        *uuid.UUID         `json:"-"`
	SoldByID      *uuid.UUID         `json:"-"`
	StartsAt      time.Time          `json:"startsAt"`
	ExpiresAt     time.Time          `json:"expiresAt"`
	DaysLeft      int                `json:"daysLeft"`
	Price         int64              `json:"price"`
	Cost          int64              `json:"cost"`
	Currency      string             `json:"currency"`
	Status        SubscriptionStatus `json:"status"`
	AutoRenew     bool               `json:"autoRenew"`
	DeliveredAt   *time.Time         `json:"deliveredAt,omitempty"`
	DeliveryNote  *string            `json:"deliveryNote,omitempty"`
	RenewedFromID *uuid.UUID         `json:"renewedFromId,omitempty"`
	Notes         *string            `json:"notes,omitempty"`
	CreatedAt     time.Time          `json:"createdAt"`
}

type MasterAccount struct {
	ID            uuid.UUID           `json:"id"`
	ProductID     uuid.UUID           `json:"-"`
	SupplierID    *uuid.UUID          `json:"-"`
	Label         string              `json:"label"`
	LoginHint     *string             `json:"loginHint,omitempty"`
	HasCredential bool                `json:"hasCredential"`
	TotalSlots    int                 `json:"totalSlots"`
	PurchaseCost  int64               `json:"purchaseCost"`
	Currency      string              `json:"currency"`
	PurchasedAt   *time.Time          `json:"purchasedAt,omitempty"`
	ExpiresAt     *time.Time          `json:"expiresAt,omitempty"`
	Status        MasterAccountStatus `json:"status"`
	Notes         *string             `json:"notes,omitempty"`
	CreatedAt     time.Time           `json:"createdAt"`
}

type Slot struct {
	ID              uuid.UUID  `json:"id"`
	MasterAccountID uuid.UUID  `json:"masterAccountId"`
	SlotNo          int        `json:"slotNo"`
	Label           *string    `json:"label,omitempty"`
	Pin             *string    `json:"pin,omitempty"`
	Status          SlotStatus `json:"status"`
	SubscriptionID  *uuid.UUID `json:"-"`
}

type Invoice struct {
	ID         uuid.UUID     `json:"id"`
	Number     string        `json:"number"`
	CustomerID uuid.UUID     `json:"-"`
	Status     InvoiceStatus `json:"status"`
	Subtotal   int64         `json:"subtotal"`
	Discount   int64         `json:"discount"`
	Total      int64         `json:"total"`
	PaidAmount int64         `json:"paidAmount"`
	DueAmount  int64         `json:"dueAmount"`
	Currency   string        `json:"currency"`
	IssuedAt   time.Time     `json:"issuedAt"`
	DueAt      *time.Time    `json:"dueAt,omitempty"`
	PaidAt     *time.Time    `json:"paidAt,omitempty"`
	Notes      *string       `json:"notes,omitempty"`
	CreatedAt  time.Time     `json:"createdAt"`
}

type Payment struct {
	ID                 uuid.UUID     `json:"id"`
	CustomerID         *uuid.UUID    `json:"-"`
	InvoiceID          *uuid.UUID    `json:"-"`
	Method             PaymentMethod `json:"method"`
	TrxID              *string       `json:"trxId,omitempty"`
	SenderNumber       *string       `json:"senderNumber,omitempty"`
	Amount             int64         `json:"amount"`
	Currency           string        `json:"currency"`
	Status             PaymentStatus `json:"status"`
	VerifiedByID       *uuid.UUID    `json:"-"`
	VerifiedAt         *time.Time    `json:"verifiedAt,omitempty"`
	VerificationSource *string       `json:"verificationSource,omitempty"`
	Notes              *string       `json:"notes,omitempty"`
	ReceivedAt         time.Time     `json:"receivedAt"`
	CreatedAt          time.Time     `json:"createdAt"`
}

type LedgerEntry struct {
	ID          uuid.UUID       `json:"id"`
	EntryType   LedgerEntryType `json:"entryType"`
	Amount      int64           `json:"amount"`
	RefType     *string         `json:"refType,omitempty"`
	RefID       *uuid.UUID      `json:"refId,omitempty"`
	Note        *string         `json:"note,omitempty"`
	CreatedByID *uuid.UUID      `json:"-"`
	CreatedAt   time.Time       `json:"createdAt"`
}

type ProductStat struct {
	ProductID           uuid.UUID `json:"-"`
	ActiveSubscriptions int       `json:"activeSubscriptions"`
	Revenue             int64     `json:"revenue"`
	Cost                int64     `json:"cost"`
	Profit              int64     `json:"profit"`
	RenewalRate         float64   `json:"renewalRate"`
}

type AtRiskCustomer struct {
	CustomerID uuid.UUID  `json:"-"`
	Reason     string     `json:"reason"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	DueBalance int64      `json:"dueBalance"`
}

// ---- second-engineer modules (inbox, content, ticket, reminder, storefront)
// JSON fields that may hold arrays are typed `any` (bound to graphql.Any via the second
// model listed for the JSON scalar in gqlgen.yml); object-only JSON stays map[string]any.

type Conversation struct {
	ID                 uuid.UUID          `json:"id"`
	SocialAccountID    uuid.UUID          `json:"-"`
	CustomerID         *uuid.UUID         `json:"-"`
	IdentityID         *uuid.UUID         `json:"-"`
	AssignedToID       *uuid.UUID         `json:"-"`
	Platform           SocialPlatform     `json:"platform"`
	ExternalThreadID   string             `json:"externalThreadId"`
	Status             ConversationStatus `json:"status"`
	UnreadCount        int                `json:"unreadCount"`
	LastMessageAt      *time.Time         `json:"lastMessageAt,omitempty"`
	LastMessagePreview *string            `json:"lastMessagePreview,omitempty"`
	LastInboundAt      *time.Time         `json:"lastInboundAt,omitempty"`
	CanReply           bool               `json:"canReply"`
	Labels             []string           `json:"labels"`
	CreatedAt          time.Time          `json:"createdAt"`
}

type Message struct {
	ID             uuid.UUID        `json:"id"`
	ConversationID uuid.UUID        `json:"conversationId"`
	Direction      MessageDirection `json:"direction"`
	ExternalID     *string          `json:"externalId,omitempty"`
	Kind           MessageKind      `json:"kind"`
	Body           *string          `json:"body,omitempty"`
	Attachments    any              `json:"attachments"`
	SentByID       *uuid.UUID       `json:"-"`
	Status         string           `json:"status"`
	Error          *string          `json:"error,omitempty"`
	SentAt         time.Time        `json:"sentAt"`
}

type SendMessageInput struct {
	ConversationID uuid.UUID      `json:"conversationId"`
	Body           *string        `json:"body,omitempty"`
	TemplateID     *uuid.UUID     `json:"templateId,omitempty"`
	Variables      map[string]any `json:"variables,omitempty"`
	Attachments    any            `json:"attachments,omitempty"`
}

type Post struct {
	ID          uuid.UUID  `json:"id"`
	Body        string     `json:"body"`
	Media       any        `json:"media"`
	ScheduledAt *time.Time `json:"scheduledAt,omitempty"`
	Status      PostStatus `json:"status"`
	CreatedByID *uuid.UUID `json:"-"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type PostTarget struct {
	ID              uuid.UUID  `json:"id"`
	SocialAccountID uuid.UUID  `json:"-"`
	ExternalPostID  *string    `json:"externalPostId,omitempty"`
	Status          string     `json:"status"`
	Error           *string    `json:"error,omitempty"`
	PublishedAt     *time.Time `json:"publishedAt,omitempty"`
}

type PostInput struct {
	Body             string      `json:"body"`
	Media            any         `json:"media,omitempty"`
	ScheduledAt      *time.Time  `json:"scheduledAt,omitempty"`
	SocialAccountIds []uuid.UUID `json:"socialAccountIds"`
}

type Ticket struct {
	ID             uuid.UUID      `json:"id"`
	Number         int            `json:"number"`
	CustomerID     uuid.UUID      `json:"-"`
	SubscriptionID *uuid.UUID     `json:"-"`
	ConversationID *uuid.UUID     `json:"conversationId,omitempty"`
	Kind           TicketKind     `json:"kind"`
	Priority       TicketPriority `json:"priority"`
	Status         TicketStatus   `json:"status"`
	Title          string         `json:"title"`
	Description    *string        `json:"description,omitempty"`
	Reason         *string        `json:"reason,omitempty"`
	OldSlotID      *uuid.UUID     `json:"-"`
	NewSlotID      *uuid.UUID     `json:"-"`
	AssignedToID   *uuid.UUID     `json:"-"`
	ResolvedAt     *time.Time     `json:"resolvedAt,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

type TicketComment struct {
	ID         uuid.UUID  `json:"id"`
	AuthorID   *uuid.UUID `json:"-"`
	Body       string     `json:"body"`
	IsInternal bool       `json:"isInternal"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type ReminderRule struct {
	ID         uuid.UUID       `json:"id"`
	Name       string          `json:"name"`
	Trigger    ReminderTrigger `json:"trigger"`
	OffsetDays int             `json:"offsetDays"`
	SendHour   int             `json:"sendHour"`
	Channel    ReminderChannel `json:"channel"`
	TemplateID *uuid.UUID      `json:"-"`
	ProductID  *uuid.UUID      `json:"productId,omitempty"`
	IsActive   bool            `json:"isActive"`
	CreatedAt  time.Time       `json:"createdAt"`
}

type Reminder struct {
	ID              uuid.UUID      `json:"id"`
	RuleID          *uuid.UUID     `json:"-"`
	CustomerID      *uuid.UUID     `json:"-"`
	SubscriptionID  *uuid.UUID     `json:"-"`
	InvoiceID       *uuid.UUID     `json:"invoiceId,omitempty"`
	MasterAccountID *uuid.UUID     `json:"masterAccountId,omitempty"`
	Channel         string         `json:"channel"`
	ScheduledAt     time.Time      `json:"scheduledAt"`
	SentAt          *time.Time     `json:"sentAt,omitempty"`
	Status          ReminderStatus `json:"status"`
	RenderedBody    *string        `json:"renderedBody,omitempty"`
	Error           *string        `json:"error,omitempty"`
}

type Order struct {
	ID             uuid.UUID   `json:"id"`
	Number         int         `json:"number"`
	CustomerID     *uuid.UUID  `json:"-"`
	ProductID      uuid.UUID   `json:"-"`
	PlanID         *uuid.UUID  `json:"-"`
	PaymentID      *uuid.UUID  `json:"-"`
	Quantity       int         `json:"quantity"`
	Amount         int64       `json:"amount"`
	Currency       string      `json:"currency"`
	Source         OrderSource `json:"source"`
	Status         OrderStatus `json:"status"`
	SubscriptionID *uuid.UUID  `json:"subscriptionId,omitempty"`
	CustomerName   *string     `json:"customerName,omitempty"`
	CustomerPhone  *string     `json:"customerPhone,omitempty"`
	CustomerNote   *string     `json:"customerNote,omitempty"`
	CreatedAt      time.Time   `json:"createdAt"`
}

type Storefront struct {
	ID                  uuid.UUID      `json:"id"`
	Slug                string         `json:"slug"`
	DisplayName         string         `json:"displayName"`
	Tagline             *string        `json:"tagline,omitempty"`
	LogoURL             *string        `json:"logoUrl,omitempty"`
	CoverURL            *string        `json:"coverUrl,omitempty"`
	Theme               map[string]any `json:"theme"`
	Whatsapp            *string        `json:"whatsapp,omitempty"`
	MessengerURL        *string        `json:"messengerUrl,omitempty"`
	Telegram            *string        `json:"telegram,omitempty"`
	PaymentInstructions *string        `json:"paymentInstructions,omitempty"`
	PaymentNumbers      any            `json:"paymentNumbers"`
	IsPublished         bool           `json:"isPublished"`
	PublicURL           string         `json:"publicUrl"`
}

type PublicStorefront struct {
	Slug                string          `json:"slug"`
	DisplayName         string          `json:"displayName"`
	Tagline             *string         `json:"tagline,omitempty"`
	LogoURL             *string         `json:"logoUrl,omitempty"`
	CoverURL            *string         `json:"coverUrl,omitempty"`
	Theme               map[string]any  `json:"theme"`
	Whatsapp            *string         `json:"whatsapp,omitempty"`
	MessengerURL        *string         `json:"messengerUrl,omitempty"`
	Telegram            *string         `json:"telegram,omitempty"`
	PaymentInstructions *string         `json:"paymentInstructions,omitempty"`
	PaymentNumbers      any             `json:"paymentNumbers"`
	Products            []PublicProduct `json:"products"`
}

type PublicOrder struct {
	Number              int         `json:"number"`
	Status              OrderStatus `json:"status"`
	Amount              int64       `json:"amount"`
	Currency            string      `json:"currency"`
	ProductName         string      `json:"productName"`
	PaymentInstructions *string     `json:"paymentInstructions,omitempty"`
	PaymentNumbers      any         `json:"paymentNumbers"`
}

type StorefrontInput struct {
	Slug                *string        `json:"slug,omitempty"`
	DisplayName         *string        `json:"displayName,omitempty"`
	Tagline             *string        `json:"tagline,omitempty"`
	LogoURL             *string        `json:"logoUrl,omitempty"`
	CoverURL            *string        `json:"coverUrl,omitempty"`
	Theme               map[string]any `json:"theme,omitempty"`
	Whatsapp            *string        `json:"whatsapp,omitempty"`
	MessengerURL        *string        `json:"messengerUrl,omitempty"`
	Telegram            *string        `json:"telegram,omitempty"`
	PaymentInstructions *string        `json:"paymentInstructions,omitempty"`
	PaymentNumbers      any            `json:"paymentNumbers,omitempty"`
	IsPublished         *bool          `json:"isPublished,omitempty"`
}
