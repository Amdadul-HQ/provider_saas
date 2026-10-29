package billing

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"provider_saas/internal/platform/money"
	"provider_saas/internal/platform/phone"
)

// ParsedSMS is a payment notification parsed from a bKash/Nagad SMS.
type ParsedSMS struct {
	Method     string // bkash | nagad
	Amount     int64  // minor units
	Fee        int64
	Balance    *int64
	Sender     string // 01XXXXXXXXX
	TrxID      string
	ReceivedAt *time.Time // Asia/Dhaka
}

var (
	ErrNotPaymentSMS = errors.New("sms: not a recognised incoming payment")
	ErrSMSIncomplete = errors.New("sms: amount or transaction id missing")

	reAmountLabeled = regexp.MustCompile(`(?i)(?:amount|পরিমাণ)\s*:?\s*(?:tk\.?|৳|bdt)\s*([\d,]+(?:\.\d{1,2})?)`)
	reAmountAny     = regexp.MustCompile(`(?i)(?:tk\.?|৳|bdt)\s*([\d,]+(?:\.\d{1,2})?)`)
	reFee           = regexp.MustCompile(`(?i)(?:fee|ফি|চার্জ)\s*:?\s*(?:tk\.?|৳|bdt)\s*([\d,]+(?:\.\d{1,2})?)`)
	reBalance       = regexp.MustCompile(`(?i)(?:balance|ব্যালেন্স)\s*:?\s*(?:tk\.?|৳|bdt)\s*([\d,]+(?:\.\d{1,2})?)`)
	reSender        = regexp.MustCompile(`(?:\+?88)?(01[3-9]\d{8})`)
	reTrxBkash      = regexp.MustCompile(`(?i)trx\s*id\s*:?\s*([A-Z0-9]{8,12})`)
	reTrxNagad      = regexp.MustCompile(`(?i)(?:txn\s*id|trx\s*id)\s*:?\s*([A-Z0-9]{6,12})`)
	reWhen          = regexp.MustCompile(`(\d{1,2})/(\d{1,2})/(\d{4})\s+(\d{1,2}):(\d{2})`)

	incomingWords = []string{"received", "cash in", "money received", "পেয়েছেন", "গ্রহণ", "জমা হয়েছে", "এসেছে"}
	outgoingWords = []string{"send money tk", "sent tk", "payment tk", "cash out", "ক্যাশ আউট", "পাঠিয়েছেন"}
)

func isIncoming(lower string) bool {
	for _, w := range outgoingWords {
		if strings.Contains(lower, w) && !strings.Contains(lower, "received") {
			return false
		}
	}
	for _, w := range incomingWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

func parseCommon(text string, trx *regexp.Regexp, method string) (ParsedSMS, error) {
	t := phone.BengaliDigits(text)
	lower := strings.ToLower(t)
	if !isIncoming(lower) {
		return ParsedSMS{}, ErrNotPaymentSMS
	}
	p := ParsedSMS{Method: method}
	amt := reAmountLabeled.FindStringSubmatch(t)
	if amt == nil {
		amt = reAmountAny.FindStringSubmatch(t)
	}
	if amt != nil {
		v, err := money.Parse(amt[1])
		if err == nil {
			p.Amount = v
		}
	}
	if m := reFee.FindStringSubmatch(t); m != nil {
		p.Fee, _ = money.Parse(m[1])
	}
	if m := reBalance.FindStringSubmatch(t); m != nil {
		if v, err := money.Parse(m[1]); err == nil {
			p.Balance = &v
		}
	}
	if m := reSender.FindStringSubmatch(t); m != nil {
		p.Sender = m[1]
	}
	if m := trx.FindStringSubmatch(t); m != nil {
		p.TrxID = strings.ToUpper(m[1])
	}
	if m := reWhen.FindStringSubmatch(t); m != nil {
		if ts, err := time.ParseInLocation("2/1/2006 15:04", m[1]+"/"+m[2]+"/"+m[3]+" "+m[4]+":"+m[5], Dhaka); err == nil {
			p.ReceivedAt = &ts
		}
	}
	if p.Amount <= 0 || p.TrxID == "" {
		return p, ErrSMSIncomplete
	}
	return p, nil
}

// ParseBkashSMS parses bKash "received"/"Cash In" notifications (English or Bengali).
// Example: "You have received Tk 350.00 from 01712345678. Fee Tk 0.00. Balance Tk 1,250.00. TrxID 9AB12CD34E at 29/10/2026 10:15"
func ParseBkashSMS(text string) (ParsedSMS, error) {
	return parseCommon(text, reTrxBkash, "bkash")
}

// ParseNagadSMS parses Nagad "Money Received" notifications (English or Bengali).
// Example: "Money Received.\nAmount: Tk 500.00\nSender: 01812345678\nRef: N/A\nTxnID: 7XYZ8ABC\nBalance: Tk 2,000.00\n29/10/2026 11:30"
func ParseNagadSMS(text string) (ParsedSMS, error) {
	return parseCommon(text, reTrxNagad, "nagad")
}

// ParsePaymentSMS detects the provider and parses.
func ParsePaymentSMS(text string) (ParsedSMS, error) {
	l := strings.ToLower(text)
	if strings.Contains(l, "nagad") || strings.Contains(l, "নগদ") || strings.Contains(l, "txnid") || strings.Contains(l, "txn id") {
		return ParseNagadSMS(text)
	}
	return ParseBkashSMS(text)
}
