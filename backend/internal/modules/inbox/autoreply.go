package inbox

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"provider_saas/internal/modules/social"
	"provider_saas/internal/platform/apperr"
)

// JobAutoReply evaluates auto-reply rules for one inbound message (queued by Ingest so
// webhook responses stay fast).
const JobAutoReply = "inbox.autoreply"

// AutoReplyPayload is the inbox.autoreply job payload.
type AutoReplyPayload struct {
	TenantID  uuid.UUID `json:"tenant_id"`
	MessageID uuid.UUID `json:"message_id"`
}

// Auto-reply match types (auto_reply_rules.match_type).
const (
	MatchContains = "contains"
	MatchExact    = "exact"
	MatchRegex    = "regex"
	MatchAny      = "any"
)

// AutoReplyRule is one FAQ-bot rule.
type AutoReplyRule struct {
	ID               uuid.UUID
	Name             string
	Keywords         []string
	MatchType        string
	Body             *string
	TemplateID       *uuid.UUID
	Platforms        []string
	OnlyOutsideHours bool
	CooldownMinutes  int
	Priority         int
	IsActive         bool
	Hits             int
	CreatedAt        time.Time
}

type AutoReplyInput struct {
	Name             string
	Keywords         []string
	MatchType        string
	Body             *string
	TemplateID       *uuid.UUID
	Platforms        []string
	OnlyOutsideHours bool
	CooldownMinutes  int
	Priority         int
	IsActive         bool
}

// AutoReplyTest is what testAutoReply returns.
type AutoReplyTest struct {
	Matched      bool
	Rule         *AutoReplyRule
	RenderedBody *string
}

func (in *AutoReplyInput) Validate() error {
	in.Name = strings.TrimSpace(in.Name)
	in.MatchType = strings.ToLower(strings.TrimSpace(in.MatchType))
	if in.MatchType == "" {
		in.MatchType = MatchContains
	}
	if in.Body != nil {
		b := strings.TrimSpace(*in.Body)
		in.Body = &b
		if b == "" {
			in.Body = nil
		}
	}
	kws := make([]string, 0, len(in.Keywords))
	for _, k := range in.Keywords {
		if k = strings.TrimSpace(k); k != "" && !slices.Contains(kws, k) {
			kws = append(kws, k)
		}
	}
	in.Keywords = kws
	plats := make([]string, 0, len(in.Platforms))
	for _, p := range in.Platforms {
		p = strings.ToLower(strings.TrimSpace(p))
		if !social.ValidPlatform(p) {
			return apperr.ValidationField("platforms", "unsupported platform "+p)
		}
		if !slices.Contains(plats, p) {
			plats = append(plats, p)
		}
	}
	in.Platforms = plats
	switch {
	case in.Name == "" || utf8.RuneCountInString(in.Name) > 80:
		return apperr.ValidationField("name", "1..80 characters")
	case in.MatchType != MatchContains && in.MatchType != MatchExact && in.MatchType != MatchRegex && in.MatchType != MatchAny:
		return apperr.ValidationField("matchType", "invalid match type")
	case in.MatchType != MatchAny && len(in.Keywords) == 0:
		return apperr.ValidationField("keywords", "at least one keyword is required")
	case len(in.Keywords) > 30:
		return apperr.ValidationField("keywords", "at most 30 keywords")
	case in.Body == nil && in.TemplateID == nil:
		return apperr.ValidationField("body", "a reply body or a template is required")
	case in.Body != nil && utf8.RuneCountInString(*in.Body) > 2000:
		return apperr.ValidationField("body", "max 2000 characters")
	case in.CooldownMinutes < 0 || in.CooldownMinutes > 10080:
		return apperr.ValidationField("cooldownMinutes", "must be 0..10080 (7 days)")
	case in.Priority < -1000 || in.Priority > 1000:
		return apperr.ValidationField("priority", "must be -1000..1000")
	}
	for _, k := range in.Keywords {
		if utf8.RuneCountInString(k) > 200 {
			return apperr.ValidationField("keywords", "each keyword max 200 characters")
		}
		if in.MatchType == MatchRegex {
			if _, err := regexp.Compile("(?i)" + k); err != nil {
				return apperr.ValidationField("keywords", "invalid regular expression: "+k)
			}
		}
	}
	return nil
}

// normalizeText lower-cases and collapses whitespace.
func normalizeText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// trimPunct removes surrounding punctuation (including the Bengali danda) for EXACT.
func trimPunct(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) || r == '।' || r == '॥' })
}

// Matches reports whether text triggers the rule on platform (ignoring hours/cooldown).
func (r AutoReplyRule) Matches(text, platform string) bool {
	if !r.IsActive {
		return false
	}
	if len(r.Platforms) > 0 && platform != "" && !slices.Contains(r.Platforms, platform) {
		return false
	}
	t := normalizeText(text)
	if t == "" {
		return false
	}
	switch r.MatchType {
	case MatchAny:
		return true
	case MatchContains:
		for _, k := range r.Keywords {
			if k = normalizeText(k); k != "" && strings.Contains(t, k) {
				return true
			}
		}
	case MatchExact:
		for _, k := range r.Keywords {
			if trimPunct(normalizeText(k)) == trimPunct(t) {
				return true
			}
		}
	case MatchRegex:
		for _, k := range r.Keywords {
			if re, err := regexp.Compile("(?i)" + k); err == nil && re.MatchString(text) {
				return true
			}
		}
	}
	return false
}

// FirstMatch returns the first rule (rules must be ordered by priority) that matches.
// insideHours tells whether the business is open now; onlyOutsideHours rules are
// skipped while it is.
func FirstMatch(rules []AutoReplyRule, text, platform string, insideHours bool) (AutoReplyRule, bool) {
	for _, r := range rules {
		if r.OnlyOutsideHours && insideHours {
			continue
		}
		if r.Matches(text, platform) {
			return r, true
		}
	}
	return AutoReplyRule{}, false
}

// SortRules orders by priority (high first), then oldest first.
func SortRules(rules []AutoReplyRule) {
	slices.SortStableFunc(rules, func(a, b AutoReplyRule) int {
		if a.Priority != b.Priority {
			return b.Priority - a.Priority
		}
		return a.CreatedAt.Compare(b.CreatedAt)
	})
}

// BusinessHours is tenant.settings.business_hours: {"start":"10:00","end":"22:00",
// "days":[0..6 | "sun".."sat"]} in the tenant timezone. end < start spans midnight.
type BusinessHours struct {
	Start, End int // minutes after midnight
	Days       map[time.Weekday]bool
}

// DefaultBusinessHours applies when the tenant has not configured any: 10:00-22:00 daily.
var DefaultBusinessHours = BusinessHours{Start: 10 * 60, End: 22 * 60, Days: allDays()}

func allDays() map[time.Weekday]bool {
	m := map[time.Weekday]bool{}
	for d := time.Sunday; d <= time.Saturday; d++ {
		m[d] = true
	}
	return m
}

var dayNames = map[string]time.Weekday{"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday}

func parseClock(v any) (int, bool) {
	s, ok := v.(string)
	if !ok {
		return 0, false
	}
	h, m, ok := strings.Cut(strings.TrimSpace(s), ":")
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if !ok || err1 != nil || err2 != nil || hh < 0 || hh > 24 || mm < 0 || mm > 59 || hh*60+mm > 24*60 {
		return 0, false
	}
	return hh*60 + mm, true
}

// ParseBusinessHours reads settings.business_hours; invalid or missing => default.
func ParseBusinessHours(settings map[string]any) BusinessHours {
	raw, ok := settings["business_hours"].(map[string]any)
	if !ok {
		return DefaultBusinessHours
	}
	bh := BusinessHours{Days: map[time.Weekday]bool{}}
	var ok1, ok2 bool
	bh.Start, ok1 = parseClock(raw["start"])
	bh.End, ok2 = parseClock(raw["end"])
	if !ok1 || !ok2 {
		return DefaultBusinessHours
	}
	days, _ := raw["days"].([]any)
	if days == nil {
		bh.Days = allDays()
	}
	for _, d := range days {
		switch v := d.(type) {
		case float64:
			if v >= 0 && v <= 6 {
				bh.Days[time.Weekday(int(v))] = true
			}
		case string:
			if wd, ok := dayNames[strings.ToLower(strings.TrimSpace(v))[:min(3, len(strings.TrimSpace(v)))]]; ok {
				bh.Days[wd] = true
			}
		}
	}
	return bh
}

// Open reports whether t (already in the tenant timezone) is inside business hours.
func (b BusinessHours) Open(t time.Time) bool {
	mins := t.Hour()*60 + t.Minute()
	if b.Start == b.End {
		return b.Days[t.Weekday()] // 24h on listed days
	}
	if b.Start < b.End {
		return b.Days[t.Weekday()] && mins >= b.Start && mins < b.End
	}
	// Overnight (e.g. 18:00-02:00): the early-morning part belongs to the previous day.
	if mins >= b.Start {
		return b.Days[t.Weekday()]
	}
	if mins < b.End {
		return b.Days[(t.Weekday()+6)%7]
	}
	return false
}

// TenantLocal converts now to the tenant timezone (Asia/Dhaka when unknown).
func TenantLocal(now time.Time, tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" {
		loc, err = time.LoadLocation("Asia/Dhaka")
		if err != nil {
			loc = time.FixedZone("BDT", 6*3600)
		}
	}
	return now.In(loc)
}

func (r AutoReplyRule) String() string { return fmt.Sprintf("rule %s (%s)", r.Name, r.MatchType) }
