package inbox

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"provider_saas/internal/modules/social"
	"provider_saas/internal/platform/apperr"
)

// Conversation statuses.
const (
	StatusOpen    = "open"
	StatusPending = "pending"
	StatusClosed  = "closed"
	StatusSpam    = "spam"
)

// Message directions / statuses.
const (
	Inbound  = "inbound"
	Outbound = "outbound"

	MsgQueued   = "queued"
	MsgSent     = "sent"
	MsgFailed   = "failed"
	MsgReceived = "received"
)

// Template categories.
var validCategories = map[string]bool{"quick_reply": true, "reminder": true, "delivery": true, "invoice": true, "winback": true, "other": true}
var validStatus = map[string]bool{StatusOpen: true, StatusPending: true, StatusClosed: true, StatusSpam: true}

type Conversation struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	SocialAccountID    uuid.UUID
	CustomerID         *uuid.UUID
	IdentityID         *uuid.UUID
	Platform           string
	ExternalThreadID   string
	Status             string
	AssignedTo         *uuid.UUID
	UnreadCount        int
	LastMessageAt      *time.Time
	LastMessagePreview *string
	LastInboundAt      *time.Time
	Labels             []string
	CreatedAt          time.Time
}

// ActivityAt is the inbox sort key (last message, else creation).
func (c Conversation) ActivityAt() time.Time {
	if c.LastMessageAt != nil {
		return *c.LastMessageAt
	}
	return c.CreatedAt
}

// CanReply applies the platform messaging-window policy.
func (c Conversation) CanReply(now time.Time) bool {
	return social.CanReply(c.Platform, c.LastInboundAt, now)
}

type Message struct {
	ID             uuid.UUID
	ConversationID uuid.UUID
	Direction      string
	ExternalID     *string
	Kind           string
	Body           *string
	Attachments    []social.Attachment
	SentBy         *uuid.UUID
	Status         string
	Error          *string
	SentAt         time.Time
	CreatedAt      time.Time
}

type Template struct {
	ID       uuid.UUID
	Key      string
	Name     string
	Body     string
	Category string
	IsActive bool
}

type TemplateInput struct {
	Key      string
	Name     string
	Body     string
	Category string
	IsActive bool
}

func (in *TemplateInput) Validate() error {
	in.Key, in.Name = strings.TrimSpace(strings.ToLower(in.Key)), strings.TrimSpace(in.Name)
	in.Body, in.Category = strings.TrimSpace(in.Body), strings.ToLower(in.Category)
	switch {
	case in.Key == "" || len(in.Key) > 80:
		return apperr.ValidationField("key", "required (max 80 chars)")
	case in.Name == "":
		return apperr.ValidationField("name", "required")
	case in.Body == "":
		return apperr.ValidationField("body", "required")
	case utf8.RuneCountInString(in.Body) > 4000:
		return apperr.ValidationField("body", "max 4000 characters")
	case !validCategories[in.Category]:
		return apperr.ValidationField("category", "invalid category")
	}
	return nil
}

type Filter struct {
	Status          *string
	Platform        *string
	SocialAccountID *uuid.UUID
	AssignedTo      *uuid.UUID
	CustomerID      *uuid.UUID
	Unassigned      *bool
	Search          *string
	Label           *string
}

type Counts struct {
	Open       int
	Unassigned int
	Mine       int
	Unread     int
}

type SendInput struct {
	ConversationID uuid.UUID
	Body           *string
	TemplateID     *uuid.UUID
	Variables      map[string]string
	Attachments    []social.Attachment
}

// IngestResult reports what one inbound message did.
type IngestResult struct {
	ConversationID  uuid.UUID
	CustomerID      uuid.UUID
	MessageID       uuid.UUID
	Duplicate       bool
	CustomerCreated bool
}

// TemplateVars are the placeholders every template supports.
var TemplateVars = []string{"customer", "product", "expiry", "amount", "link"}

var placeholder = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)

// Render replaces {{name}} placeholders. Known variables without a value become empty;
// unknown placeholders are left untouched so a typo is visible to the seller.
func Render(body string, vars map[string]string) string {
	known := map[string]bool{}
	for _, k := range TemplateVars {
		known[k] = true
	}
	return placeholder.ReplaceAllStringFunc(body, func(m string) string {
		k := strings.ToLower(placeholder.FindStringSubmatch(m)[1])
		if v, ok := vars[k]; ok {
			return v
		}
		if known[k] {
			return ""
		}
		return m
	})
}

// Preview shortens a body for the conversation list.
func Preview(kind string, body *string) string {
	if body != nil && strings.TrimSpace(*body) != "" {
		s := strings.Join(strings.Fields(*body), " ")
		if utf8.RuneCountInString(s) > 120 {
			s = string([]rune(s)[:117]) + "..."
		}
		return s
	}
	switch kind {
	case "image":
		return "📷 Photo"
	case "video":
		return "🎬 Video"
	case "audio":
		return "🎤 Audio"
	case "sticker":
		return "Sticker"
	case "file":
		return "📎 File"
	}
	return ""
}

// NormalizeLabel trims and lower-cases a label.
func NormalizeLabel(l string) (string, error) {
	l = strings.ToLower(strings.TrimSpace(l))
	if l == "" || utf8.RuneCountInString(l) > 30 {
		return "", apperr.ValidationField("label", "1..30 characters")
	}
	return l, nil
}

// ValidStatus reports whether s is a conversation status.
func ValidStatus(s string) bool { return validStatus[s] }

// DefaultName is used when a platform does not tell us the sender's name.
func DefaultName(platform, thread string) string {
	tail := thread
	if len(tail) > 4 {
		tail = tail[len(tail)-4:]
	}
	names := map[string]string{"facebook": "Facebook", "instagram": "Instagram", "whatsapp": "WhatsApp", "telegram": "Telegram"}
	p, ok := names[platform]
	if !ok {
		p = platform
	}
	return p + " user " + tail
}
