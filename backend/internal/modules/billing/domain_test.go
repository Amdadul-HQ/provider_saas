package billing

import (
	"testing"
	"time"
)

func TestTotals(t *testing.T) {
	tests := []struct {
		name       string
		items      []ItemInput
		discount   int64
		sub, total int64
		wantErr    bool
	}{
		{"single", []ItemInput{{Description: "Netflix 1M", Quantity: 1, UnitPrice: 35000}}, 0, 35000, 35000, false},
		{"qty default+discount", []ItemInput{{Description: "a", UnitPrice: 10000}, {Description: "b", Quantity: 2, UnitPrice: 5000}}, 5000, 20000, 15000, false},
		{"empty", nil, 0, 0, 0, true},
		{"discount too big", []ItemInput{{Description: "a", Quantity: 1, UnitPrice: 100}}, 101, 0, 0, true},
		{"negative price", []ItemInput{{Description: "a", Quantity: 1, UnitPrice: -1}}, 0, 0, 0, true},
		{"no description", []ItemInput{{Description: " ", Quantity: 1, UnitPrice: 1}}, 0, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub, total, err := Totals(tt.items, tt.discount)
			if (err != nil) != tt.wantErr || sub != tt.sub || total != tt.total {
				t.Fatalf("Totals = %d,%d,%v", sub, total, err)
			}
		})
	}
}

func TestStatusAfterPayment(t *testing.T) {
	tests := []struct {
		cur         string
		paid, total int64
		want        string
	}{
		{InvoiceSent, 0, 100, InvoiceSent}, {InvoiceSent, 50, 100, InvoicePartial}, {InvoicePartial, 100, 100, InvoicePaid},
		{InvoiceDraft, 150, 100, InvoicePaid}, {InvoiceVoid, 100, 100, InvoiceVoid}, {InvoiceOverdue, 0, 100, InvoiceOverdue},
		{InvoicePaid, 0, 100, InvoiceSent},
	}
	for _, tt := range tests {
		if got := StatusAfterPayment(tt.cur, tt.paid, tt.total); got != tt.want {
			t.Errorf("StatusAfterPayment(%s,%d,%d) = %s want %s", tt.cur, tt.paid, tt.total, got, tt.want)
		}
	}
}

func TestInvoiceNumber(t *testing.T) {
	at := time.Date(2026, 10, 31, 20, 0, 0, 0, time.UTC) // already November in Dhaka
	if got := InvoiceNumber(at, 7); got != "INV-202611-0007" {
		t.Fatalf("got %s", got)
	}
	if got := InvoiceNumber(at, 123456); got != "INV-202611-123456" {
		t.Fatalf("got %s", got)
	}
}

func TestPaymentInputValidate(t *testing.T) {
	sp := func(s string) *string { return &s }
	tests := []struct {
		in      PaymentInput
		wantErr bool
		trx     *string
	}{
		{PaymentInput{Method: "BKASH", Amount: 100, TrxID: sp(" 9ab12cd34e ")}, false, sp("9AB12CD34E")},
		{PaymentInput{Method: "cash", Amount: 100, TrxID: sp("")}, false, nil},
		{PaymentInput{Method: "paypal", Amount: 100}, true, nil},
		{PaymentInput{Method: "cash", Amount: 0}, true, nil},
		{PaymentInput{Method: "bkash", Amount: 1, TrxID: sp("abc")}, true, nil},
	}
	for i, tt := range tests {
		err := tt.in.Validate()
		if (err != nil) != tt.wantErr {
			t.Fatalf("case %d: %v", i, err)
		}
		if err == nil && ((tt.trx == nil) != (tt.in.TrxID == nil) || (tt.trx != nil && *tt.trx != *tt.in.TrxID)) {
			t.Fatalf("case %d: trx = %v", i, tt.in.TrxID)
		}
	}
	if (Invoice{Total: 100, PaidAmount: 30}).DueAmount() != 70 || (Invoice{Total: 100, Status: InvoiceVoid}).DueAmount() != 0 {
		t.Fatal("DueAmount")
	}
}
