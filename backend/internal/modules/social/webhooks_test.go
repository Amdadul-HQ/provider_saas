package social

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Real-shaped sample payloads (trimmed from Meta / Telegram docs).
const (
	messengerText = `{"object":"page","entry":[{"id":"100200300400500","time":1730190000000,"messaging":[
		{"sender":{"id":"6543210987654321"},"recipient":{"id":"100200300400500"},"timestamp":1730190000123,
		 "message":{"mid":"m_AbCdEf123","text":"Netflix 1 month price koto?"}}]}]}`
	messengerImageAndEcho = `{"object":"page","entry":[{"id":"100200300400500","time":1730190000000,"messaging":[
		{"sender":{"id":"6543210987654321"},"recipient":{"id":"100200300400500"},"timestamp":1730190001000,
		 "message":{"mid":"m_img1","attachments":[{"type":"image","payload":{"url":"https://scontent.xx.fbcdn.net/v/t1.jpg"}}]}},
		{"sender":{"id":"100200300400500"},"recipient":{"id":"6543210987654321"},"timestamp":1730190002000,
		 "message":{"mid":"m_echo","text":"our reply","is_echo":true}},
		{"sender":{"id":"6543210987654321"},"recipient":{"id":"100200300400500"},"timestamp":1730190003000,
		 "delivery":{"mids":["m_echo"],"watermark":1730190002000}}]}]}`
	instagramText = `{"object":"instagram","entry":[{"id":"17841400000000001","time":1730190000000,"messaging":[
		{"sender":{"id":"998877665544"},"recipient":{"id":"17841400000000001"},"timestamp":1730190000000,
		 "message":{"mid":"aWdfZAG1faWQ","text":"Spotify family slot ache?"}}]}]}`
	whatsappText = `{"object":"whatsapp_business_account","entry":[{"id":"WABA_ID_1","changes":[{"field":"messages","value":{
		"messaging_product":"whatsapp","metadata":{"display_phone_number":"8801700000000","phone_number_id":"109876543210"},
		"contacts":[{"profile":{"name":"Karim Hossain"},"wa_id":"8801811000002"}],
		"messages":[{"from":"8801811000002","id":"wamid.HBgNODgwMTgxMTAwMDAwMhUCABIYFjNFQjA","timestamp":"1730190000","type":"text","text":{"body":"TrxID 9AB12CD34E pathalam"}},
		            {"from":"8801811000002","id":"wamid.IMG","timestamp":"1730190005","type":"image","image":{"id":"MEDIA_1","mime_type":"image/jpeg","caption":"screenshot"}}]}}]}]}`
	whatsappStatusOnly = `{"object":"whatsapp_business_account","entry":[{"id":"WABA_ID_1","changes":[{"field":"messages","value":{
		"messaging_product":"whatsapp","metadata":{"phone_number_id":"109876543210"},
		"statuses":[{"id":"wamid.OUT","status":"delivered","timestamp":"1730190010","recipient_id":"8801811000002"}]}}]}]}`
	telegramText = `{"update_id":10000,"message":{"message_id":1365,"from":{"id":111222333,"is_bot":false,"first_name":"Tanvir","last_name":"Ahmed","username":"tanvir_bd"},
		"chat":{"id":111222333,"first_name":"Tanvir","type":"private"},"date":1730190000,"text":"/start"}}`
	telegramPhoto = `{"update_id":10001,"message":{"message_id":1366,"from":{"id":111222333,"is_bot":false,"first_name":"Tanvir"},
		"chat":{"id":111222333,"type":"private"},"date":1730190100,"photo":[{"file_id":"small"},{"file_id":"BIG_FILE"}],"caption":"payment proof"}}`
	telegramBot = `{"update_id":10002,"message":{"message_id":5,"from":{"id":42,"is_bot":true,"first_name":"Other bot"},"chat":{"id":-100,"type":"group"},"date":1730190200,"text":"hi"}}`
)

func TestParseMetaPayload(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantN     int
		platform  string
		account   string
		thread    string
		mid       string
		kind      string
		body0     string
		sender0   string
		wantError bool
	}{
		{name: "messenger text", body: messengerText, wantN: 1, platform: PlatformFacebook, account: "100200300400500",
			thread: "6543210987654321", mid: "m_AbCdEf123", kind: "text", body0: "Netflix 1 month price koto?"},
		{name: "messenger image, echo + delivery skipped", body: messengerImageAndEcho, wantN: 1, platform: PlatformFacebook,
			account: "100200300400500", thread: "6543210987654321", mid: "m_img1", kind: "image"},
		{name: "instagram", body: instagramText, wantN: 1, platform: PlatformInstagram, account: "17841400000000001",
			thread: "998877665544", mid: "aWdfZAG1faWQ", kind: "text", body0: "Spotify family slot ache?"},
		{name: "whatsapp text + image", body: whatsappText, wantN: 2, platform: PlatformWhatsApp, account: "109876543210",
			thread: "8801811000002", mid: "wamid.HBgNODgwMTgxMTAwMDAwMhUCABIYFjNFQjA", kind: "text", body0: "TrxID 9AB12CD34E pathalam", sender0: "Karim Hossain"},
		{name: "whatsapp statuses only", body: whatsappStatusOnly, wantN: 0},
		{name: "garbage", body: `{"object":"user"}`, wantError: true},
		{name: "not json", body: `nope`, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs, err := ParseMetaPayload([]byte(tt.body))
			if (err != nil) != tt.wantError {
				t.Fatalf("err = %v", err)
			}
			if len(msgs) != tt.wantN {
				t.Fatalf("got %d messages: %+v", len(msgs), msgs)
			}
			if tt.wantN == 0 {
				return
			}
			m := msgs[0]
			if m.Platform != tt.platform || m.AccountExternal != tt.account || m.ThreadExternal != tt.thread ||
				m.MessageExternal != tt.mid || m.Kind != tt.kind || m.Body != tt.body0 || m.SenderName != tt.sender0 {
				t.Fatalf("message = %+v", m)
			}
			if err := m.Validate(); err != nil {
				t.Fatalf("validate: %v", err)
			}
			if m.SentAt.IsZero() || m.SentAt.Year() != 2024 {
				t.Fatalf("sentAt = %v", m.SentAt)
			}
		})
	}
	msgs, _ := ParseMetaPayload([]byte(whatsappText))
	if img := msgs[1]; img.Kind != "image" || img.Attachments[0].ID != "MEDIA_1" || img.Body != "screenshot" || img.SenderPhone != "8801811000002" {
		t.Fatalf("whatsapp image = %+v", img)
	}
	msgs, _ = ParseMetaPayload([]byte(messengerImageAndEcho))
	if a := msgs[0].Attachments; len(a) != 1 || a[0].Type != "image" || !strings.HasPrefix(a[0].URL, "https://") {
		t.Fatalf("messenger attachments = %+v", a)
	}
}

func TestParseTelegramUpdate(t *testing.T) {
	m, ok, err := ParseTelegramUpdate([]byte(telegramText), "7000000001")
	if err != nil || !ok {
		t.Fatalf("text: ok=%v err=%v", ok, err)
	}
	if m.ThreadExternal != "111222333" || m.MessageExternal != "1365" || m.SenderName != "Tanvir Ahmed" || m.SenderUsername != "tanvir_bd" ||
		m.Body != "/start" || m.AccountExternal != "7000000001" {
		t.Fatalf("text = %+v", m)
	}
	m, ok, _ = ParseTelegramUpdate([]byte(telegramPhoto), "7000000001")
	if !ok || m.Kind != "image" || m.Body != "payment proof" || m.Attachments[0].ID != "BIG_FILE" {
		t.Fatalf("photo = %+v", m)
	}
	if _, ok, _ := ParseTelegramUpdate([]byte(telegramBot), "7000000001"); ok {
		t.Fatal("bot sender must be ignored")
	}
	if _, _, err := ParseTelegramUpdate([]byte("{"), "x"); err == nil {
		t.Fatal("bad json accepted")
	}
}

func TestStateSignAndVerify(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	now := time.Now()
	st := State{TenantID: uuid.New(), UserID: uuid.New(), Platform: PlatformFacebook, Exp: now.Add(StateTTL).Unix()}
	s := SignState(secret, st)
	got, err := VerifyState(secret, s, now)
	if err != nil || got.TenantID != st.TenantID || got.UserID != st.UserID || got.Platform != PlatformFacebook {
		t.Fatalf("verify: %+v %v", got, err)
	}
	if _, err := VerifyState(secret, s, now.Add(11*time.Minute)); err == nil {
		t.Fatal("expired state accepted")
	}
	if _, err := VerifyState([]byte("another-secret-another-secret-xx"), s, now); err == nil {
		t.Fatal("state signed with another key accepted")
	}
	tampered := "x" + s[1:]
	if _, err := VerifyState(secret, tampered, now); err == nil {
		t.Fatal("tampered state accepted")
	}
	if WebhookSecret(secret, st.TenantID) != WebhookSecret(secret, st.TenantID) || len(WebhookSecret(secret, st.TenantID)) != 64 {
		t.Fatal("webhook secret must be deterministic hex")
	}
}

func TestVerifyMetaSignature(t *testing.T) {
	body := []byte(messengerText)
	sig := SignMetaBody("app-secret", body)
	if !VerifyMetaSignature("app-secret", body, sig) {
		t.Fatal("valid signature rejected")
	}
	for _, tc := range []struct{ secret, header string }{
		{"app-secret", ""}, {"app-secret", "sha256=00"}, {"app-secret", strings.Replace(sig, "sha256=", "sha1=", 1)},
		{"other", sig}, {"", sig},
	} {
		if VerifyMetaSignature(tc.secret, body, tc.header) {
			t.Fatalf("accepted secret=%q header=%q", tc.secret, tc.header)
		}
	}
}

func TestCanReply(t *testing.T) {
	now := time.Now()
	recent, old := now.Add(-23*time.Hour), now.Add(-25*time.Hour)
	tests := []struct {
		platform string
		last     *time.Time
		want     bool
	}{
		{PlatformFacebook, &recent, true}, {PlatformFacebook, &old, false}, {PlatformFacebook, nil, false},
		{PlatformWhatsApp, &recent, true}, {PlatformWhatsApp, &old, false},
		{PlatformInstagram, &old, false},
		{PlatformTelegram, nil, true}, {PlatformTelegram, &old, true},
	}
	for _, tt := range tests {
		if got := CanReply(tt.platform, tt.last, now); got != tt.want {
			t.Errorf("CanReply(%s, %v) = %v", tt.platform, tt.last, got)
		}
	}
}

func TestWebhookHandlersRejectUnsigned(t *testing.T) {
	ingested := 0
	h := NewWebhooks(WebhookDeps{
		Ingest:      func(context.Context, InboundMessage) error { ingested++; return nil },
		AppSecret:   "app-secret",
		VerifyToken: "verify-me",
		SMSTenant:   func(context.Context, string) (uuid.UUID, error) { return uuid.Nil, errBadToken },
		SMSIngest:   func(context.Context, uuid.UUID, string, string, time.Time) error { return nil },
	})
	mux := http.NewServeMux()
	r := newTestRouter(h)
	mux.Handle("/", r)

	do := func(method, path, body string, hdr map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	if rec := do("GET", "/webhooks/meta?hub.mode=subscribe&hub.verify_token=verify-me&hub.challenge=42", "", nil); rec.Code != 200 || rec.Body.String() != "42" {
		t.Fatalf("verify: %d %q", rec.Code, rec.Body.String())
	}
	if rec := do("GET", "/webhooks/meta?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=42", "", nil); rec.Code != 403 {
		t.Fatalf("bad verify token: %d", rec.Code)
	}
	if rec := do("POST", "/webhooks/meta", messengerText, nil); rec.Code != 403 {
		t.Fatalf("unsigned: %d", rec.Code)
	}
	if rec := do("POST", "/webhooks/meta", messengerText, map[string]string{"X-Hub-Signature-256": SignMetaBody("wrong", []byte(messengerText))}); rec.Code != 403 {
		t.Fatalf("wrong signature: %d", rec.Code)
	}
	if ingested != 0 {
		t.Fatal("rejected payload was ingested")
	}
	if rec := do("POST", "/webhooks/meta", messengerText, map[string]string{"X-Hub-Signature-256": SignMetaBody("app-secret", []byte(messengerText))}); rec.Code != 200 || ingested != 1 {
		t.Fatalf("signed: %d ingested=%d", rec.Code, ingested)
	}
	if rec := do("POST", "/webhooks/payments/sms", "You have received Tk 100", nil); rec.Code != 401 {
		t.Fatalf("sms without token: %d", rec.Code)
	}
	if rec := do("POST", "/webhooks/payments/sms", "x", map[string]string{"Authorization": "Bearer nope"}); rec.Code != 401 {
		t.Fatalf("sms bad token: %d", rec.Code)
	}
}

func TestSMSWebhookRequiresSender(t *testing.T) {
	tid := uuid.New()
	var gotFrom string
	h := NewWebhooks(WebhookDeps{
		SMSTenant: func(context.Context, string) (uuid.UUID, error) { return tid, nil },
		SMSIngest: func(_ context.Context, _ uuid.UUID, from, _ string, _ time.Time) error { gotFrom = from; return nil },
	})
	r := newTestRouter(h)
	do := func(path, body, ctype string) int {
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer sms_x")
		req.Header.Set("Content-Type", ctype)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := do("/webhooks/payments/sms", `{"text":"You have received Tk 100"}`, "application/json"); c != 400 {
		t.Fatalf("missing from: %d", c)
	}
	if c := do("/webhooks/payments/sms", `{"text":"You have received Tk 100","from":"bKash"}`, "application/json"); c != 200 || gotFrom != "bKash" {
		t.Fatalf("json from: %d %q", c, gotFrom)
	}
	if c := do("/webhooks/payments/sms?from=16247", "You have received Tk 100", "text/plain"); c != 200 || gotFrom != "16247" {
		t.Fatalf("query from: %d %q", c, gotFrom)
	}
}

func TestSafeErrorNeverLeaksURL(t *testing.T) {
	secretURL := "https://graph.facebook.com/v21.0/oauth/access_token?client_secret=APPSECRET&access_token=TOKEN123"
	ue := &url.Error{Op: "Get", URL: secretURL, Err: context.DeadlineExceeded}
	for _, err := range []error{ue, &TransportError{Op: "GET", Host: "graph.facebook.com", Err: ue}, fmt.Errorf("wrapped: %w", ue)} {
		msg := SafeError(err)
		if strings.Contains(msg, "APPSECRET") || strings.Contains(msg, "TOKEN123") || strings.Contains(msg, "client_secret") {
			t.Fatalf("leak: %q", msg)
		}
	}
	if got := SafeError(&TransportError{Op: "GET", Host: "graph.facebook.com", Err: ue}); got != "social api GET graph.facebook.com: timeout" {
		t.Fatalf("got %q", got)
	}
}

func TestHTTPClientSendsTokensInHeaderNotURL(t *testing.T) {
	var gotURL, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL, gotAuth = r.URL.String(), r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = io.WriteString(w, `{"data":[{"id":"PN1"}],"access_token":"x"}`)
	}))
	defer srv.Close()
	c := NewHTTPClient("app", "app-secret", "v21.0")
	c.GraphBase = srv.URL
	nums, err := c.WhatsAppPhoneNumbers(context.Background(), "WABA1", "TOKEN-ABC")
	if err != nil || len(nums) != 1 || nums[0] != "PN1" {
		t.Fatalf("nums=%v err=%v", nums, err)
	}
	if strings.Contains(gotURL, "TOKEN-ABC") || gotAuth != "Bearer TOKEN-ABC" {
		t.Fatalf("url=%s auth=%s", gotURL, gotAuth)
	}
	if !strings.Contains(gotURL, "appsecret_proof="+AppSecretProof("TOKEN-ABC", "app-secret")) {
		t.Fatalf("missing appsecret_proof: %s", gotURL)
	}
	if _, err := c.ExchangeCode(context.Background(), "CODE", "https://x/cb"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(gotURL, "app-secret") || !strings.Contains(gotBody, "client_secret=app-secret") {
		t.Fatalf("app secret must be in the POST body only: url=%s body=%s", gotURL, gotBody)
	}
	// Transport failure: the error must not contain the token or the URL query.
	c.GraphBase = "http://127.0.0.1:1"
	err = c.VerifyToken(context.Background(), "TOKEN-ABC")
	if err == nil || strings.Contains(err.Error(), "TOKEN-ABC") || strings.Contains(err.Error(), "appsecret_proof") {
		t.Fatalf("transport error leaks: %v", err)
	}
}
