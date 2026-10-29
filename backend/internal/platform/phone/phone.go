// Package phone normalises Bangladeshi mobile numbers and Bengali digits.
package phone

import (
	"regexp"
	"strings"
)

var bdMobile = regexp.MustCompile(`^01[3-9]\d{8}$`)

// BengaliDigits converts ০-৯ to 0-9.
func BengaliDigits(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '০' && r <= '৯' {
			return '0' + (r - '০')
		}
		return r
	}, s)
}

// Normalize returns a BD mobile in local 01XXXXXXXXX form when recognisable; otherwise
// the trimmed input with spaces/dashes removed. ok reports whether it is a valid BD mobile.
func Normalize(s string) (string, bool) {
	s = BengaliDigits(strings.TrimSpace(s))
	s = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(s)
	switch {
	case strings.HasPrefix(s, "+880"):
		s = "0" + s[4:]
	case strings.HasPrefix(s, "880") && len(s) == 13:
		s = "0" + s[3:]
	case strings.HasPrefix(s, "00880"):
		s = "0" + s[5:]
	}
	return s, bdMobile.MatchString(s)
}
