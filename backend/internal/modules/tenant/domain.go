package tenant

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Tenant is a provider business.
type Tenant struct {
	ID          uuid.UUID
	Name        string
	Slug        string
	Plan        string
	Status      string
	Currency    string
	Timezone    string
	Locale      string
	Settings    map[string]any
	TrialEndsAt *time.Time
	CreatedAt   time.Time
}

// UserSummary is the user part shown next to members / audit rows.
type UserSummary struct {
	ID              uuid.UUID
	Name            string
	Email           *string
	Phone           *string
	AvatarURL       *string
	IsPlatformAdmin bool
	CreatedAt       time.Time
}

// Member is a user's membership in this tenant.
type Member struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	UserID        uuid.UUID
	Role          string
	CommissionBps int
	Status        string
	CreatedAt     time.Time
	User          UserSummary
}

// Membership is a user's membership seen from the user (with the tenant).
type Membership struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Tenant        Tenant
	Role          string
	CommissionBps int
	Status        string
}

type Notification struct {
	ID        uuid.UUID
	Kind      string
	Title     string
	Body      *string
	Link      *string
	ReadAt    *time.Time
	CreatedAt time.Time
}

type AuditLog struct {
	ID         int64
	UserID     *uuid.UUID
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	Meta       map[string]any
	CreatedAt  time.Time
}

type UpdateInput struct {
	Name     *string
	Currency *string
	Timezone *string
	Locale   *string
	Settings map[string]any
}

type InviteInput struct {
	Email         *string
	Phone         *string
	Name          string
	Role          string
	CommissionBps int
}

// Member roles and statuses.
const (
	RoleOwner    = "owner"
	RoleAdmin    = "admin"
	RoleStaff    = "staff"
	RoleReseller = "reseller"
	RoleViewer   = "viewer"

	MemberInvited  = "invited"
	MemberActive   = "active"
	MemberDisabled = "disabled"
)

var validRoles = map[string]bool{RoleOwner: true, RoleAdmin: true, RoleStaff: true, RoleReseller: true, RoleViewer: true}

// ValidRole reports whether r is a known role.
func ValidRole(r string) bool { return validRoles[r] }

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify turns a business name into a URL slug ("Rahim's Netflix BD" -> "rahims-netflix-bd").
// Names without latin letters/digits (e.g. Bengali) fall back to "shop".
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, "'", "")
	s = nonSlug.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	if len(s) < 3 {
		if s == "" {
			return "shop"
		}
		s = "shop-" + s
	}
	return s
}
