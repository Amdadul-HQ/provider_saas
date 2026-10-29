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
