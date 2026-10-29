package reminder

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"provider_saas/internal/platform/apperr"
)

// Triggers.
const (
	BeforeExpiry   = "before_expiry"
	OnExpiry       = "on_expiry"
	AfterExpiry    = "after_expiry"
	InvoiceDue     = "invoice_due"
	InvoiceOverdue = "invoice_overdue"
	MasterExpiry   = "master_expiry"
)

// Reminder statuses.
const (
	StatusScheduled = "scheduled"
	StatusSent      = "sent"
	StatusFailed    = "failed"
	StatusSkipped   = "skipped"
	StatusCancelled = "cancelled"
)

// Channels.
const (
	ChannelPreferred = "preferred"
	ChannelInApp     = "in_app"
)

var (
	validTrigger = map[string]bool{BeforeExpiry: true, OnExpiry: true, AfterExpiry: true, InvoiceDue: true, InvoiceOverdue: true, MasterExpiry: true}
	validChannel = map[string]bool{"preferred": true, "facebook": true, "instagram": true, "whatsapp": true, "telegram": true, "sms": true, "email": true, "in_app": true}
	validStatus  = map[string]bool{StatusScheduled: true, StatusSent: true, StatusFailed: true, StatusSkipped: true, StatusCancelled: true}
)

// PlanHorizon is how far ahead the planner materialises reminders; PlanGrace is how far
// in the past a just-computed send time may be and still be sent (e.g. rule created late).
const (
	PlanHorizon = 7 * 24 * time.Hour
	PlanGrace   = 12 * time.Hour
)

type Rule struct {
	ID         uuid.UUID
	Name       string
	Trigger    string
	OffsetDays int
	SendHour   int
	Channel    string
	TemplateID *uuid.UUID
	ProductID  *uuid.UUID
	IsActive   bool
	CreatedAt  time.Time
}

type RuleInput struct {
	Name       string
	Trigger    string
	OffsetDays int
	SendHour   int
	Channel    string
	TemplateID *uuid.UUID
	ProductID  *uuid.UUID
	IsActive   bool
}

func (in *RuleInput) Validate() error {
	in.Name, in.Trigger, in.Channel = strings.TrimSpace(in.Name), strings.ToLower(in.Trigger), strings.ToLower(in.Channel)
	if in.Channel == "" {
		in.Channel = ChannelPreferred
	}
	switch {
	case in.Name == "" || utf8.RuneCountInString(in.Name) > 100:
		return apperr.ValidationField("name", "1..100 characters")
	case !validTrigger[in.Trigger]:
		return apperr.ValidationField("trigger", "invalid trigger")
	case in.OffsetDays < 0 || in.OffsetDays > 90:
		return apperr.ValidationField("offsetDays", "must be 0..90")
	case in.SendHour < 0 || in.SendHour > 23:
		return apperr.ValidationField("sendHour", "must be 0..23")
	case !validChannel[in.Channel]:
		return apperr.ValidationField("channel", "invalid channel")
	}
	if in.Trigger == OnExpiry {
		in.OffsetDays = 0
	}
	return nil
}

type Reminder struct {
	ID              uuid.UUID
	RuleID          *uuid.UUID
	CustomerID      *uuid.UUID
	SubscriptionID  *uuid.UUID
	InvoiceID       *uuid.UUID
	MasterAccountID *uuid.UUID
	Channel         string
	ScheduledAt     time.Time
	SentAt          *time.Time
	Status          string
	RenderedBody    *string
	Error           *string
	CreatedAt       time.Time
}

type Filter struct {
	Status     *string
	CustomerID *uuid.UUID
}

// ValidStatus reports whether s is a reminder status.
func ValidStatus(s string) bool { return validStatus[s] }

// SendAt computes the local send time: the calendar day of anchor in loc shifted by
// dayShift days, at hour:00 local time.
func SendAt(anchor time.Time, dayShift, hour int, loc *time.Location) time.Time {
	a := anchor.In(loc)
	return time.Date(a.Year(), a.Month(), a.Day()+dayShift, hour, 0, 0, 0, loc)
}

// DayShift is the offset applied to the anchor date for a trigger.
func DayShift(trigger string, offsetDays int) int {
	switch trigger {
	case BeforeExpiry, InvoiceDue, MasterExpiry:
		return -offsetDays
	case AfterExpiry, InvoiceOverdue:
		return offsetDays
	}
	return 0
}

// DefaultBody is used when a rule has no template.
func DefaultBody(trigger string) string {
	switch trigger {
	case BeforeExpiry:
		return "প্রিয় {{customer}}, আপনার {{product}} এর মেয়াদ {{expiry}} তারিখে শেষ হবে। রিনিউ করতে {{amount}} পাঠান: {{link}}"
	case OnExpiry:
		return "প্রিয় {{customer}}, আজ আপনার {{product}} এর মেয়াদ শেষ হচ্ছে। রিনিউ করুন: {{link}}"
	case AfterExpiry:
		return "প্রিয় {{customer}}, আপনার {{product}} এর মেয়াদ {{expiry}} তারিখে শেষ হয়েছে। আবার চালু করতে: {{link}}"
	case InvoiceDue:
		return "প্রিয় {{customer}}, আপনার বিল {{amount}} পরিশোধের শেষ তারিখ {{expiry}}। বিস্তারিত: {{link}}"
	case InvoiceOverdue:
		return "প্রিয় {{customer}}, আপনার {{amount}} বিল এখনও বাকি আছে ({{expiry}} থেকে)। পরিশোধ করুন: {{link}}"
	case MasterExpiry:
		return "মাস্টার অ্যাকাউন্ট {{product}} এর মেয়াদ {{expiry}} তারিখে শেষ হবে। সাপ্লায়ার থেকে রিনিউ করুন।"
	}
	return "প্রিয় {{customer}}, {{product}}: {{expiry}}"
}
