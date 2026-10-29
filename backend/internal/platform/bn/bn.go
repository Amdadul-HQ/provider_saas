// Package bn formats dates and numbers for Bengali (bn-BD) customer messages.
package bn

import (
	"strings"
	"time"
)

var months = [...]string{"জানুয়ারি", "ফেব্রুয়ারি", "মার্চ", "এপ্রিল", "মে", "জুন", "জুলাই", "আগস্ট", "সেপ্টেম্বর", "অক্টোবর", "নভেম্বর", "ডিসেম্বর"}

// Digits converts ASCII digits to Bengali digits (০-৯), leaving other runes unchanged.
func Digits(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return '০' + (r - '0')
		}
		return r
	}, s)
}

// Date renders t in loc as "২৯ অক্টোবর ২০২৬".
func Date(t time.Time, loc *time.Location) string {
	if loc == nil {
		loc = Dhaka
	}
	t = t.In(loc)
	return Digits(t.Format("2")) + " " + months[t.Month()-1] + " " + Digits(t.Format("2006"))
}

// Dhaka is Bangladesh time (UTC+6, no DST); used when a tenant timezone is unknown.
var Dhaka = time.FixedZone("Asia/Dhaka", 6*3600)

// Location loads an IANA zone, falling back to Dhaka.
func Location(name string) *time.Location {
	if name == "" {
		return Dhaka
	}
	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}
	return Dhaka
}
