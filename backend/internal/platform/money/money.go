// Package money handles amounts in minor units (paisa) — never floats (ADR-004).
package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const DefaultCurrency = "BDT"

// Format renders minor units as "1,250.00 BDT" (or "৳1,250.00" for BDT when symbol=true).
func Format(amount int64, currency string, symbol bool) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	major, minor := amount/100, amount%100
	s := groupThousands(strconv.FormatInt(major, 10)) + fmt.Sprintf(".%02d", minor)
	if neg {
		s = "-" + s
	}
	if symbol && (currency == "" || currency == DefaultCurrency) {
		return "৳" + s
	}
	if currency == "" {
		currency = DefaultCurrency
	}
	return s + " " + currency
}

func groupThousands(s string) string {
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

var ErrBadAmount = errors.New("invalid amount")

// Parse converts "1,250.00", "350", "350.5" (major units) into minor units.
func Parse(s string) (int64, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" {
		return 0, ErrBadAmount
	}
	whole, frac, _ := strings.Cut(s, ".")
	if len(frac) > 2 {
		return 0, ErrBadAmount
	}
	for len(frac) < 2 {
		frac += "0"
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || w < 0 {
		return 0, ErrBadAmount
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, ErrBadAmount
	}
	return w*100 + f, nil
}

// FromMajor converts whole taka to paisa.
func FromMajor(taka int64) int64 { return taka * 100 }
