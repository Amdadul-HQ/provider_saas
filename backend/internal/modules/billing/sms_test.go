package billing

import (
	"errors"
	"testing"
	"time"
)

func TestParseBkashSMS(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    ParsedSMS
		wantErr error
	}{
		{
			name: "english received",
			text: "You have received Tk 350.00 from 01712345678. Fee Tk 0.00. Balance Tk 1,250.00. TrxID 9AB12CD34E at 29/10/2026 10:15",
			want: ParsedSMS{Method: "bkash", Amount: 35000, Sender: "01712345678", TrxID: "9AB12CD34E"},
		},
		{
			name: "cash in",
			text: "Cash In Tk 1,500.50 from 01812345678 successful. Fee Tk 0.00. Balance Tk 3,250.50. TrxID BCD45EF678 at 30/10/2026 09:05",
			want: ParsedSMS{Method: "bkash", Amount: 150050, Sender: "01812345678", TrxID: "BCD45EF678"},
		},
		{
			name: "payment received merchant",
			text: "You have received payment Tk 900.00 from 01912345678. Ref netflix. Fee Tk 13.05. Balance Tk 10,886.95. TrxID AK7Q2LMN0P at 01/11/2026 21:40",
			want: ParsedSMS{Method: "bkash", Amount: 90000, Fee: 1305, Sender: "01912345678", TrxID: "AK7Q2LMN0P"},
		},
		{
			name: "bengali digits",
			text: "আপনি ০১৭১২৩৪৫৬৭৮ থেকে ৳৩৫০.০০ পেয়েছেন। ফি ৳০.০০। ব্যালেন্স ৳১,২৫০.০০। TrxID 9AB12CD34E সময় ২৯/১০/২০২৬ ১০:১৫",
			want: ParsedSMS{Method: "bkash", Amount: 35000, Sender: "01712345678", TrxID: "9AB12CD34E"},
		},
		{
			name:    "outgoing send money",
			text:    "Send Money Tk 500.00 to 01712345678 successful. Fee Tk 5.00. Balance Tk 745.00. TrxID 9AB12CD34Z at 29/10/2026 10:15",
			wantErr: ErrNotPaymentSMS,
		},
		{
			name:    "otp",
			text:    "Your bKash verification code is 123456",
			wantErr: ErrNotPaymentSMS,
		},
		{
			name:    "missing trx",
			text:    "You have received Tk 350.00 from 01712345678.",
			wantErr: ErrSMSIncomplete,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBkashSMS(tt.text)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Method != tt.want.Method || got.Amount != tt.want.Amount || got.Sender != tt.want.Sender ||
				got.TrxID != tt.want.TrxID || got.Fee != tt.want.Fee {
				t.Fatalf("got %+v want %+v", got, tt.want)
			}
		})
	}
	p, _ := ParseBkashSMS(tests[0].text)
	want := time.Date(2026, 10, 29, 10, 15, 0, 0, Dhaka)
	if p.ReceivedAt == nil || !p.ReceivedAt.Equal(want) || p.Balance == nil || *p.Balance != 125000 {
		t.Fatalf("time/balance = %v %v", p.ReceivedAt, p.Balance)
	}
}

func TestParseNagadSMS(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		amount  int64
		sender  string
		trx     string
		wantErr error
	}{
		{"english", "Money Received.\nAmount: Tk 500.00\nSender: 01812345678\nRef: N/A\nTxnID: 7XYZ8ABC\nBalance: Tk 2,000.00\n29/10/2026 11:30", 50000, "01812345678", "7XYZ8ABC", nil},
		{"bengali", "টাকা গ্রহণ সফল। পরিমাণ: ৳৫০০.০০ প্রেরক: ০১৮১২৩৪৫৬৭৮ TxnID: 7XYZ8ABD ব্যালেন্স: ৳২,০০০.০০ ২৯/১০/২০২৬ ১১:৩০", 50000, "01812345678", "7XYZ8ABD", nil},
		{"cash out is outgoing", "Cash Out Tk 1,000.00 to agent 01712345678. TxnID 7XYZ8ABE", 0, "", "", ErrNotPaymentSMS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseNagadSMS(tt.text)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil || got.Amount != tt.amount || got.Sender != tt.sender || got.TrxID != tt.trx || got.Method != "nagad" {
				t.Fatalf("got %+v err %v", got, err)
			}
		})
	}
}

func TestParsePaymentSMSDetects(t *testing.T) {
	p, err := ParsePaymentSMS("Money Received.\nAmount: Tk 500.00\nSender: 01812345678\nTxnID: 7XYZ8ABC")
	if err != nil || p.Method != "nagad" {
		t.Fatalf("%+v %v", p, err)
	}
	p, err = ParsePaymentSMS("You have received Tk 350.00 from 01712345678. TrxID 9AB12CD34E")
	if err != nil || p.Method != "bkash" {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestSenderAllowed(t *testing.T) {
	custom := map[string]any{SettingSMSSenders: map[string]any{"bkash": []any{"bKash-Merchant"}}}
	cases := []struct {
		method, sender string
		settings       map[string]any
		want           bool
	}{
		{"bkash", "bKash", nil, true},
		{"bkash", "BKASH", nil, true},
		{"bkash", "16247", nil, true},
		{"bkash", "01712345678", nil, false}, // a personal number copying bKash wording
		{"bkash", "+8801712345678", nil, false},
		{"bkash", "NAGAD", nil, false}, // wrong provider
		{"nagad", "NAGAD", nil, true},
		{"nagad", "16167", nil, true},
		{"rocket", "16216", nil, true},
		{"bkash", "", nil, false},
		{"bkash", "bKash", custom, false}, // tenant override replaces the default list
		{"bkash", "bkash-merchant", custom, true},
		{"nagad", "NAGAD", custom, true}, // other methods keep their defaults
	}
	for _, c := range cases {
		if got := SenderAllowed(c.method, c.sender, c.settings); got != c.want {
			t.Errorf("SenderAllowed(%s, %q) = %v, want %v", c.method, c.sender, got, c.want)
		}
	}
	if AllowOverpayment(nil) || !AllowOverpayment(map[string]any{"allow_overpayment": true}) || !AllowOverpayment(map[string]any{"allowOverpayment": true}) {
		t.Fatal("AllowOverpayment")
	}
}
