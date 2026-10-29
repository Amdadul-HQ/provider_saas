package storefront

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"provider_saas/internal/platform/apperr"
)

// Order statuses / sources.
const (
	OrderNew             = "new"
	OrderAwaitingPayment = "awaiting_payment"
	OrderPaid            = "paid"
	OrderDelivered       = "delivered"
	OrderCancelled       = "cancelled"

	SourceStorefront = "storefront"
	SourceInbox      = "inbox"
	SourceManual     = "manual"
	SourcePortal     = "portal"
)

// PortalTTL is how long a magic link stays valid.
const PortalTTL = 7 * 24 * time.Hour

type Storefront struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	Slug                string
	DisplayName         string
	Tagline             *string
	LogoURL             *string
	CoverURL            *string
	Theme               map[string]any
	Whatsapp            *string
	MessengerURL        *string
	Telegram            *string
	PaymentInstructions *string
	PaymentNumbers      json.RawMessage
	IsPublished         bool
}

// PaymentNumbersValue decodes payment_numbers for the API (array or object).
func (s Storefront) PaymentNumbersValue() any {
	var v any
	if len(s.PaymentNumbers) == 0 || json.Unmarshal(s.PaymentNumbers, &v) != nil || v == nil {
		return []any{}
	}
	return v
}

type Input struct {
	Slug                *string
	DisplayName         *string
	Tagline             *string
	LogoURL             *string
	CoverURL            *string
	Theme               map[string]any
	Whatsapp            *string
	MessengerURL        *string
	Telegram            *string
	PaymentInstructions *string
	PaymentNumbers      any // nil = unchanged
	IsPublished         *bool
}

var slugRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$`)
var reservedSlugs = map[string]bool{"admin": true, "api": true, "app": true, "login": true, "register": true, "dashboard": true, "www": true, "graphql": true}

// Validate normalises the input and returns the encoded payment numbers (nil = unchanged).
func (in *Input) Validate() ([]byte, error) {
	if in.Slug != nil {
		v := strings.ToLower(strings.TrimSpace(*in.Slug))
		if !slugRe.MatchString(v) || reservedSlugs[v] {
			return nil, apperr.ValidationField("slug", "3-40 lowercase letters, digits or dashes")
		}
		in.Slug = &v
	}
	if in.DisplayName != nil {
		v := strings.TrimSpace(*in.DisplayName)
		if v == "" {
			return nil, apperr.ValidationField("displayName", "must not be empty")
		}
		in.DisplayName = &v
	}
	if in.PaymentNumbers == nil {
		return nil, nil
	}
	switch v := in.PaymentNumbers.(type) {
	case []any:
		for _, x := range v {
			m, ok := x.(map[string]any)
			if !ok {
				return nil, apperr.ValidationField("paymentNumbers", "must be a list of {method, number, type}")
			}
			if n, _ := m["number"].(string); strings.TrimSpace(n) == "" {
				return nil, apperr.ValidationField("paymentNumbers", "every entry needs a number")
			}
		}
	case map[string]any:
	default:
		return nil, apperr.ValidationField("paymentNumbers", "must be a list of {method, number, type}")
	}
	return json.Marshal(in.PaymentNumbers)
}

type Order struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	Number         int
	CustomerID     *uuid.UUID
	ConversationID *uuid.UUID
	ProductID      uuid.UUID
	PlanID         *uuid.UUID
	Quantity       int
	Amount         int64
	Currency       string
	Source         string
	Status         string
	PaymentID      *uuid.UUID
	SubscriptionID *uuid.UUID
	CustomerName   *string
	CustomerPhone  *string
	CustomerNote   *string
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
}

type OrderFilter struct {
	Status *string
	Source *string
	Search *string
}

type PlaceOrderInput struct {
	Slug          string
	ProductID     uuid.UUID
	PlanID        uuid.UUID
	Quantity      int
	CustomerName  string
	CustomerPhone string
	CustomerNote  *string
	PaymentMethod *string
	TrxID         *string
	SenderNumber  *string
}

// Public views (no internal ids beyond product/plan).
type PublicPlan struct {
	ID           uuid.UUID
	Name         string
	DurationDays int
	Price        int64
	Currency     string
}

type PublicProduct struct {
	ID          uuid.UUID
	Name        string
	Category    *string
	Description *string
	ImageURL    *string
	Plans       []PublicPlan
	InStock     bool
}

type PublicStorefront struct {
	Storefront
	Products []PublicProduct
}

type PublicOrder struct {
	Number              int
	Status              string
	Amount              int64
	Currency            string
	ProductName         string
	PaymentInstructions *string
	PaymentNumbers      any
}

type PortalSubscription struct {
	ID           uuid.UUID
	ProductName  string
	PlanName     *string
	StartsAt     time.Time
	ExpiresAt    time.Time
	DaysLeft     int
	Status       string
	DeliveryNote *string
}

type PortalInvoice struct {
	ID         uuid.UUID
	Number     string
	Total      int64
	PaidAmount int64
	Status     string
	IssuedAt   time.Time
	DueAt      *time.Time
}

type PortalView struct {
	CustomerName  string
	CustomerPhone *string
	DueBalance    int64
	Subscriptions []PortalSubscription
	Invoices      []PortalInvoice
	Storefront    PublicStorefront
}

// NewPortalToken returns a random URL-safe token and its sha256 hash (only the hash is stored).
func NewPortalToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
