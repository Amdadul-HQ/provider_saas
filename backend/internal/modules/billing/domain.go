package billing

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"provider_saas/internal/platform/apperr"
)

// Invoice statuses.
const (
	InvoiceDraft   = "draft"
	InvoiceSent    = "sent"
	InvoicePartial = "partial"
	InvoicePaid    = "paid"
	InvoiceOverdue = "overdue"
	InvoiceVoid    = "void"
)

// Payment statuses.
const (
	PaymentPending  = "pending"
	PaymentVerified = "verified"
	PaymentRejected = "rejected"
	PaymentRefunded = "refunded"
)

// Verification sources.
const (
	SourceManual     = "manual"
	SourceSMSMatch   = "sms_match"
	SourceAPI        = "api"
	SourceStorefront = "storefront"
)

// Ledger entry types: debit = customer owes more, credit = customer owes less.
const (
	Debit  = "debit"
	Credit = "credit"
)

var (
	validMethods      = map[string]bool{"bkash": true, "nagad": true, "rocket": true, "upay": true, "bank": true, "cash": true, "card": true, "other": true}
	mobileMoney       = map[string]bool{"bkash": true, "nagad": true, "rocket": true, "upay": true}
	validInvoiceState = map[string]bool{InvoiceDraft: true, InvoiceSent: true, InvoicePartial: true, InvoicePaid: true, InvoiceOverdue: true, InvoiceVoid: true}
)

type Invoice struct {
	ID         uuid.UUID
	Number     string
	CustomerID uuid.UUID
	Status     string
	Subtotal   int64
	Discount   int64
	Total      int64
	PaidAmount int64
	Currency   string
	IssuedAt   time.Time
	DueAt      *time.Time
	PaidAt     *time.Time
	Notes      *string
	CreatedBy  *uuid.UUID
	CreatedAt  time.Time
}

// DueAmount is what is still owed on the invoice.
func (i Invoice) DueAmount() int64 {
	if i.Status == InvoiceVoid {
		return 0
	}
	return max(0, i.Total-i.PaidAmount)
}

type InvoiceItem struct {
	ID             uuid.UUID
	InvoiceID      uuid.UUID
	SubscriptionID *uuid.UUID
	Description    string
	Quantity       int
	UnitPrice      int64
	Total          int64
}

type Payment struct {
	ID                 uuid.UUID
	CustomerID         *uuid.UUID
	InvoiceID          *uuid.UUID
	Method             string
	TrxID              *string
	SenderNumber       *string
	Amount             int64
	Currency           string
	Status             string
	VerifiedBy         *uuid.UUID
	VerifiedAt         *time.Time
	VerificationSource *string
	Notes              *string
	ReceivedAt         time.Time
	CreatedAt          time.Time
}

type InboxItem struct {
	ID               uuid.UUID
	Source           string
	Sender           *string // SMS sender id reported by the forwarder ("bKash", "16247", ...)
	Method           *string
	SenderNumber     *string
	TrxID            *string
	Amount           *int64
	RawText          string
	Status           string
	MatchedPaymentID *uuid.UUID
	ReceivedAt       time.Time
}

type LedgerEntry struct {
	ID         uuid.UUID
	CustomerID uuid.UUID
	EntryType  string
	Amount     int64
	Currency   string
	RefType    *string
	RefID      *uuid.UUID
	Note       *string
	CreatedBy  *uuid.UUID
	CreatedAt  time.Time
}

type ItemInput struct {
	SubscriptionID *uuid.UUID
	Description    string
	Quantity       int
	UnitPrice      int64
}

type CreateInvoiceInput struct {
	CustomerID uuid.UUID
	Items      []ItemInput
	Discount   int64
	DueAt      *time.Time
	Notes      *string
	Send       bool
}

type PaymentInput struct {
	Method       string
	Amount       int64
	TrxID        *string
	SenderNumber *string
	ReceivedAt   *time.Time
	Notes        *string
}

// RecordOptions controls how a payment is recorded.
type RecordOptions struct {
	// ForceVerified marks the payment verified (seller asserts money received, e.g. markPaid).
	ForceVerified bool
	// Source overrides verification_source (storefront/api); default manual.
	Source string
}

type InvoiceFilter struct {
	Search      *string
	Status      *string
	CustomerID  *uuid.UUID
	OverdueOnly bool
}

type PaymentFilter struct {
	Status     *string
	Method     *string
	CustomerID *uuid.UUID
	Search     *string
}

// Totals validates items and computes subtotal/total.
func Totals(items []ItemInput, discount int64) (subtotal, total int64, err error) {
	if len(items) == 0 {
		return 0, 0, apperr.ValidationField("items", "at least one item is required")
	}
	for i := range items {
		it := &items[i]
		it.Description = strings.TrimSpace(it.Description)
		if it.Quantity == 0 {
			it.Quantity = 1
		}
		switch {
		case it.Description == "":
			return 0, 0, apperr.ValidationField("items.description", "required")
		case it.Quantity < 0 || it.Quantity > 1000:
			return 0, 0, apperr.ValidationField("items.quantity", "must be 1..1000")
		case it.UnitPrice < 0:
			return 0, 0, apperr.ValidationField("items.unitPrice", "must be >= 0")
		}
		subtotal += int64(it.Quantity) * it.UnitPrice
	}
	if discount < 0 || discount > subtotal {
		return 0, 0, apperr.ValidationField("discount", "must be between 0 and the subtotal")
	}
	return subtotal, subtotal - discount, nil
}

// StatusAfterPayment returns the invoice status for a paid amount.
func StatusAfterPayment(current string, paid, total int64) string {
	switch {
	case current == InvoiceVoid:
		return InvoiceVoid
	case paid >= total:
		return InvoicePaid
	case paid > 0:
		return InvoicePartial
	case current == InvoiceDraft || current == InvoiceOverdue:
		return current
	default:
		return InvoiceSent
	}
}

// InvoiceNumber formats INV-YYYYMM-NNNN (NNNN from invoice_number_seq; grows past 4 digits).
func InvoiceNumber(at time.Time, seq int64) string {
	return fmt.Sprintf("INV-%s-%04d", at.In(Dhaka).Format("200601"), seq)
}

// Dhaka is Bangladesh time (UTC+6, no DST).
var Dhaka = time.FixedZone("Asia/Dhaka", 6*3600)

// ValidatePayment normalises and validates a payment input.
func (in *PaymentInput) Validate() error {
	in.Method = strings.ToLower(strings.TrimSpace(in.Method))
	if !validMethods[in.Method] {
		return apperr.ValidationField("method", "invalid payment method")
	}
	if in.Amount <= 0 {
		return apperr.ValidationField("amount", "must be > 0")
	}
	if in.TrxID != nil {
		t := strings.ToUpper(strings.TrimSpace(*in.TrxID))
		if t == "" {
			in.TrxID = nil
		} else {
			if len(t) < 6 || len(t) > 32 {
				return apperr.ValidationField("trxId", "looks invalid")
			}
			in.TrxID = &t
		}
	}
	return nil
}

// DefaultSMSSenders are the sender ids whose SMS may auto-verify a payment, per method
// (SECURITY H4). A tenant can override them with settings.payment_sms_senders
// ({"bkash": ["bKash","16247"], ...}). Personal 11-digit numbers never qualify.
var DefaultSMSSenders = map[string][]string{
	"bkash":  {"bKash", "16247"},
	"nagad":  {"NAGAD", "16167"},
	"rocket": {"Rocket", "16216"},
}

// SettingSMSSenders / SettingAllowOverpayment are tenants.settings keys used by billing.
const (
	SettingSMSSenders       = "payment_sms_senders"
	SettingAllowOverpayment = "allow_overpayment"
)

// SenderAllowed reports whether an SMS from sender may auto-verify a payment of method.
func SenderAllowed(method, sender string, settings map[string]any) bool {
	sender = normSender(sender)
	if sender == "" {
		return false
	}
	allowed := DefaultSMSSenders[method]
	if custom, ok := settings[SettingSMSSenders].(map[string]any); ok {
		if list, ok := custom[method].([]any); ok {
			allowed = allowed[:0:0]
			for _, v := range list {
				if s, ok := v.(string); ok {
					allowed = append(allowed, s)
				}
			}
		}
	}
	for _, a := range allowed {
		if a = normSender(a); a != "" && a == sender {
			return true
		}
	}
	return false
}

func normSender(s string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "+")))
}

// AllowOverpayment reads the tenant switch (default false; QA BUG-04).
func AllowOverpayment(settings map[string]any) bool {
	for _, k := range []string{SettingAllowOverpayment, "allowOverpayment"} {
		if v, ok := settings[k].(bool); ok {
			return v
		}
	}
	return false
}

// IsMobileMoney reports whether method is a mobile financial service with TrxIDs.
func IsMobileMoney(method string) bool { return mobileMoney[method] }
