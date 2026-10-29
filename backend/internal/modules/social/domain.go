package social

import (
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"provider_saas/internal/platform/apperr"
)

// Platforms (social_accounts.platform, conversations.platform).
const (
	PlatformFacebook  = "facebook"
	PlatformInstagram = "instagram"
	PlatformWhatsApp  = "whatsapp"
	PlatformTelegram  = "telegram"
)

// Account statuses.
const (
	StatusConnected    = "connected"
	StatusTokenExpired = "token_expired"
	StatusDisconnected = "disconnected"
	StatusError        = "error"
)

// MetaScopes are requested by Facebook Login for pages + Instagram messaging.
var MetaScopes = []string{
	"pages_messaging", "pages_manage_metadata", "pages_show_list",
	"instagram_basic", "instagram_manage_messages", "business_management",
}

var validPlatforms = map[string]bool{PlatformFacebook: true, PlatformInstagram: true, PlatformWhatsApp: true, PlatformTelegram: true}

// ValidPlatform reports whether p is a supported social platform (lower case).
func ValidPlatform(p string) bool { return validPlatforms[p] }

// Account is a connected channel: Facebook page, Instagram business account,
// WhatsApp Cloud number or Telegram bot. Its access token lives only in the vault.
type Account struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	Platform        string
	ExternalID      string
	Name            string
	Username        *string
	AvatarURL       *string
	TokenSecretID   *uuid.UUID
	TokenExpiresAt  *time.Time
	Scopes          []string
	WebhookVerified bool
	Status          string
	LastError       *string
	LastSyncedAt    *time.Time
	ConnectedBy     *uuid.UUID
	CreatedAt       time.Time
}

// TelegramChannel returns the channel/chat id posts are published to ("channel:<id>"
// entry in scopes), if configured.
func (a Account) TelegramChannel() string {
	for _, s := range a.Scopes {
		if c, ok := strings.CutPrefix(s, "channel:"); ok {
			return c
		}
	}
	return ""
}

// Attachment is a media item on a message or post.
type Attachment struct {
	Type string `json:"type"`           // image|video|audio|file|sticker
	URL  string `json:"url,omitempty"`  // public URL when known
	ID   string `json:"id,omitempty"`   // platform media / file id
	Name string `json:"name,omitempty"` // caption or filename
}

// InboundMessage is the platform-agnostic ingest contract (docs/integrations.md).
// Webhook parsers produce it; inbox.Service.Ingest consumes it.
type InboundMessage struct {
	Platform        string // facebook|instagram|whatsapp|telegram
	AccountExternal string // page id / ig id / phone_number_id / bot id
	// AccountID pins the social account when the route already identifies it (Telegram);
	// otherwise the account is resolved by (Platform, AccountExternal).
	AccountID       *uuid.UUID
	ThreadExternal  string // PSID / IGSID / wa_id / chat id
	MessageExternal string // mid / wamid / message_id (idempotency key)
	SenderName      string
	SenderAvatar    string
	SenderPhone     string // WhatsApp wa_id is the phone number
	SenderUsername  string // Telegram @username
	Kind            string // text|image|video|audio|file|sticker
	Body            string
	Attachments     []Attachment
	SentAt          time.Time
}

// Validate checks the fields every ingest needs.
func (m InboundMessage) Validate() error {
	switch {
	case !ValidPlatform(m.Platform):
		return apperr.ValidationField("platform", "unsupported platform")
	case m.AccountExternal == "" && m.AccountID == nil:
		return apperr.ValidationField("accountExternal", "required")
	case m.ThreadExternal == "":
		return apperr.ValidationField("threadExternal", "required")
	case m.MessageExternal == "":
		return apperr.ValidationField("messageExternal", "required")
	}
	return nil
}

// OAuthURL is the Facebook Login URL plus the signed state it carries.
type OAuthURL struct {
	URL   string
	State string
}

type ConnectWhatsAppInput struct {
	PhoneNumberID     string
	BusinessAccountID string
	AccessToken       string
	DisplayName       string
}

func (in *ConnectWhatsAppInput) Validate() error {
	in.PhoneNumberID, in.BusinessAccountID = strings.TrimSpace(in.PhoneNumberID), strings.TrimSpace(in.BusinessAccountID)
	in.AccessToken, in.DisplayName = strings.TrimSpace(in.AccessToken), strings.TrimSpace(in.DisplayName)
	switch {
	case in.PhoneNumberID == "":
		return apperr.ValidationField("phoneNumberId", "required")
	case in.BusinessAccountID == "":
		return apperr.ValidationField("businessAccountId", "required")
	case len(in.AccessToken) < 20:
		return apperr.ValidationField("accessToken", "looks invalid")
	case in.DisplayName == "":
		return apperr.ValidationField("displayName", "required")
	}
	return nil
}

// MessengerWindow is how long after the customer's last inbound message a business may
// reply freely on Messenger/Instagram/WhatsApp (24h standard messaging window).
const MessengerWindow = 24 * time.Hour

// CanReply reports whether a free-form reply is allowed now. Telegram bots can always
// reply once the user has started the chat.
func CanReply(platform string, lastInboundAt *time.Time, now time.Time) bool {
	if platform == PlatformTelegram {
		return true
	}
	if lastInboundAt == nil {
		return false
	}
	return now.Sub(*lastInboundAt) < MessengerWindow
}

// MessageTagAllowed reports whether a platform supports sending outside the window with
// a Messenger message tag (Facebook/Instagram only; WhatsApp needs approved templates).
func MessageTagAllowed(platform string) bool {
	return slices.Contains([]string{PlatformFacebook, PlatformInstagram}, platform)
}
