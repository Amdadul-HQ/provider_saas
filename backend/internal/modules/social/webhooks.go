package social

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"provider_saas/internal/platform/apperr"
)

// ---- payload parsers (pure, table-tested)

type metaPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		ID        string `json:"id"`
		Time      int64  `json:"time"`
		Messaging []struct {
			Sender    struct{ ID string } `json:"sender"`
			Recipient struct{ ID string } `json:"recipient"`
			Timestamp int64               `json:"timestamp"`
			Message   *struct {
				Mid         string `json:"mid"`
				Text        string `json:"text"`
				IsEcho      bool   `json:"is_echo"`
				Attachments []struct {
					Type    string `json:"type"`
					Payload struct {
						URL       string `json:"url"`
						StickerID int64  `json:"sticker_id"`
					} `json:"payload"`
				} `json:"attachments"`
			} `json:"message"`
		} `json:"messaging"`
		Changes []struct {
			Field string `json:"field"`
			Value struct {
				Metadata struct {
					PhoneNumberID string `json:"phone_number_id"`
				} `json:"metadata"`
				Contacts []struct {
					WaID    string `json:"wa_id"`
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
				} `json:"contacts"`
				Messages []map[string]any `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

func msTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Now()
	}
	return time.UnixMilli(ms)
}

// ParseMetaPayload converts a Messenger / Instagram / WhatsApp Cloud webhook body into
// inbound messages. Echoes of our own messages, delivery/read receipts and status
// updates are skipped.
func ParseMetaPayload(body []byte) ([]InboundMessage, error) {
	var p metaPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, apperr.Validation("invalid JSON payload")
	}
	var out []InboundMessage
	switch p.Object {
	case "page", "instagram":
		platform := PlatformFacebook
		if p.Object == "instagram" {
			platform = PlatformInstagram
		}
		for _, e := range p.Entry {
			for _, m := range e.Messaging {
				if m.Message == nil || m.Message.IsEcho || m.Message.Mid == "" {
					continue
				}
				in := InboundMessage{Platform: platform, AccountExternal: firstNonEmpty(m.Recipient.ID, e.ID), ThreadExternal: m.Sender.ID,
					MessageExternal: m.Message.Mid, Kind: "text", Body: m.Message.Text, SentAt: msTime(m.Timestamp)}
				for _, a := range m.Message.Attachments {
					typ := a.Type
					if a.Payload.StickerID != 0 {
						typ = "sticker"
					}
					in.Attachments = append(in.Attachments, Attachment{Type: normKind(typ), URL: a.Payload.URL})
				}
				if in.Body == "" && len(in.Attachments) > 0 {
					in.Kind = in.Attachments[0].Type
				}
				out = append(out, in)
			}
		}
	case "whatsapp_business_account":
		for _, e := range p.Entry {
			for _, ch := range e.Changes {
				if ch.Field != "messages" {
					continue
				}
				v := ch.Value
				names := map[string]string{}
				for _, c := range v.Contacts {
					names[c.WaID] = c.Profile.Name
				}
				for _, m := range v.Messages {
					in, ok := parseWhatsAppMessage(m)
					if !ok {
						continue
					}
					in.AccountExternal = v.Metadata.PhoneNumberID
					in.SenderName = names[in.ThreadExternal]
					out = append(out, in)
				}
			}
		}
	default:
		return nil, apperr.Validation("unsupported webhook object " + strconv.Quote(p.Object))
	}
	return out, nil
}

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func parseWhatsAppMessage(m map[string]any) (InboundMessage, bool) {
	from, id, typ := str(m, "from"), str(m, "id"), str(m, "type")
	if from == "" || id == "" {
		return InboundMessage{}, false
	}
	in := InboundMessage{Platform: PlatformWhatsApp, ThreadExternal: from, MessageExternal: id, SenderPhone: from, Kind: "text"}
	if ts, err := strconv.ParseInt(str(m, "timestamp"), 10, 64); err == nil {
		in.SentAt = time.Unix(ts, 0)
	} else {
		in.SentAt = time.Now()
	}
	switch typ {
	case "text":
		if t, ok := m["text"].(map[string]any); ok {
			in.Body = str(t, "body")
		}
	case "image", "video", "audio", "document", "sticker", "voice":
		media, _ := m[typ].(map[string]any)
		in.Kind = normKind(typ)
		in.Body = str(media, "caption")
		in.Attachments = []Attachment{{Type: in.Kind, ID: str(media, "id"), Name: str(media, "filename")}}
	case "button":
		if b, ok := m["button"].(map[string]any); ok {
			in.Body = str(b, "text")
		}
	case "interactive":
		if it, ok := m["interactive"].(map[string]any); ok {
			for _, k := range []string{"button_reply", "list_reply"} {
				if r, ok := it[k].(map[string]any); ok {
					in.Body = str(r, "title")
				}
			}
		}
	default:
		in.Body = "[" + typ + "]"
	}
	return in, true
}

func normKind(t string) string {
	switch t {
	case "image", "video", "audio", "sticker":
		return t
	case "voice":
		return "audio"
	case "document", "file":
		return "file"
	}
	return "file"
}

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message"`
	Edited   *telegramMessage `json:"edited_message"`
}

type telegramMessage struct {
	MessageID int64 `json:"message_id"`
	From      *struct {
		ID        int64  `json:"id"`
		IsBot     bool   `json:"is_bot"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  string `json:"username"`
	} `json:"from"`
	Chat struct {
		ID        int64  `json:"id"`
		Type      string `json:"type"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  string `json:"username"`
	} `json:"chat"`
	Date    int64  `json:"date"`
	Text    string `json:"text"`
	Caption string `json:"caption"`
	Photo   []struct {
		FileID string `json:"file_id"`
	} `json:"photo"`
	Sticker *struct {
		FileID string `json:"file_id"`
	} `json:"sticker"`
	Document *struct {
		FileID   string `json:"file_id"`
		FileName string `json:"file_name"`
	} `json:"document"`
	Voice *struct {
		FileID string `json:"file_id"`
	} `json:"voice"`
	Audio *struct {
		FileID string `json:"file_id"`
	} `json:"audio"`
	Video *struct {
		FileID string `json:"file_id"`
	} `json:"video"`
}

// ParseTelegramUpdate converts a Bot API update into an inbound message. ok=false for
// updates we ignore (no message, bot senders, group service messages).
func ParseTelegramUpdate(body []byte, botID string) (InboundMessage, bool, error) {
	var u telegramUpdate
	if err := json.Unmarshal(body, &u); err != nil {
		return InboundMessage{}, false, apperr.Validation("invalid JSON payload")
	}
	m := u.Message
	if m == nil {
		m = u.Edited
	}
	if m == nil || (m.From != nil && m.From.IsBot) {
		return InboundMessage{}, false, nil
	}
	in := InboundMessage{Platform: PlatformTelegram, AccountExternal: botID, ThreadExternal: strconv.FormatInt(m.Chat.ID, 10),
		MessageExternal: strconv.FormatInt(m.MessageID, 10), Kind: "text", Body: m.Text, SentAt: time.Unix(m.Date, 0)}
	if u.Message == nil { // edits get a distinct idempotency key per update
		in.MessageExternal += "-e" + strconv.FormatInt(u.UpdateID, 10)
	}
	first, last, user := m.Chat.FirstName, m.Chat.LastName, m.Chat.Username
	if m.From != nil {
		first, last, user = m.From.FirstName, m.From.LastName, m.From.Username
	}
	in.SenderName = strings.TrimSpace(first + " " + last)
	if in.SenderName == "" && user != "" {
		in.SenderName = "@" + user
	}
	in.SenderUsername = user
	add := func(kind, id, name string) {
		in.Attachments = append(in.Attachments, Attachment{Type: kind, ID: id, Name: name})
		if in.Body == "" {
			in.Body = m.Caption
		}
		if in.Kind == "text" {
			in.Kind = kind
		}
	}
	switch {
	case len(m.Photo) > 0:
		add("image", m.Photo[len(m.Photo)-1].FileID, "")
	case m.Sticker != nil:
		add("sticker", m.Sticker.FileID, "")
	case m.Video != nil:
		add("video", m.Video.FileID, "")
	case m.Voice != nil:
		add("audio", m.Voice.FileID, "")
	case m.Audio != nil:
		add("audio", m.Audio.FileID, "")
	case m.Document != nil:
		add("file", m.Document.FileID, m.Document.FileName)
	}
	return in, true, nil
}

// ---- HTTP handlers

// ErrUnknownAccount means a webhook arrived for a page/bot that no tenant has connected.
var ErrUnknownAccount = errors.New("social: no connected account for webhook")

// WebhookDeps are the collaborators of the webhook routes (wired in app.Container).
type WebhookDeps struct {
	Social       Service
	Ingest       func(ctx context.Context, msg InboundMessage) error
	AppSecret    string
	VerifyToken  string
	PublicAppURL string
	// SMS forwarder: Bearer token -> tenant, then billing.IngestSMS.
	SMSTenant func(ctx context.Context, token string) (uuid.UUID, error)
	// sender is the SMS sender id ("bKash", "16247", "+8801...") reported by the forwarder.
	SMSIngest func(ctx context.Context, tenantID uuid.UUID, sender, raw string, receivedAt time.Time) error
	Log       *slog.Logger
}

// Webhooks serves the REST webhook routes (ADR-003).
type Webhooks struct{ d WebhookDeps }

func NewWebhooks(d WebhookDeps) *Webhooks {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Webhooks{d: d}
}

// Mount registers the routes on r.
func (h *Webhooks) Mount(r chi.Router) {
	r.Route("/webhooks", func(r chi.Router) {
		r.Get("/meta", h.metaVerify)
		r.Post("/meta", h.metaEvent)
		r.Get("/meta/oauth", h.metaOAuth)
		r.Post("/telegram/{accountId}", h.telegram)
		r.Post("/payments/sms", h.sms)
	})
}

const maxWebhookBody = 1 << 20

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Webhooks) metaVerify(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tok := q.Get("hub.verify_token")
	if q.Get("hub.mode") != "subscribe" || h.d.VerifyToken == "" ||
		subtle.ConstantTimeCompare([]byte(tok), []byte(h.d.VerifyToken)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = io.WriteString(w, q.Get("hub.challenge"))
}

func (h *Webhooks) metaEvent(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return
	}
	if !VerifyMetaSignature(h.d.AppSecret, body, r.Header.Get("X-Hub-Signature-256")) {
		http.Error(w, "invalid signature", http.StatusForbidden)
		return
	}
	msgs, err := ParseMetaPayload(body)
	if err != nil {
		h.d.Log.Warn("meta webhook: unparseable payload", "err", err)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ingested": 0}) // never make Meta retry garbage
		return
	}
	h.ingest(w, r.Context(), msgs)
}

func (h *Webhooks) ingest(w http.ResponseWriter, ctx context.Context, msgs []InboundMessage) {
	n := 0
	for _, m := range msgs {
		err := h.d.Ingest(ctx, m)
		switch {
		case err == nil:
			n++
		case errors.Is(err, ErrUnknownAccount), errors.Is(err, apperr.ErrValidation):
			h.d.Log.Warn("webhook: message skipped", "platform", m.Platform, "account", m.AccountExternal, "err", err)
		default:
			// Transient failure: 500 makes the platform retry; ingest is idempotent on message id.
			h.d.Log.Error("webhook: ingest failed", "platform", m.Platform, "err", err)
			http.Error(w, "ingest failed", http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ingested": n})
}

// metaOAuth is the Facebook Login redirect target. It no longer connects anything
// (SECURITY M2): it only checks the state signature and forwards code+state to the
// authenticated frontend page, which calls completeMetaConnect as the signed-in user; that
// mutation consumes the single-use state bound to the user who started the flow.
func (h *Webhooks) metaOAuth(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dest := strings.TrimRight(h.d.PublicAppURL, "/") + "/social"
	if e := q.Get("error"); e != "" {
		reason := "denied"
		if q.Get("error_reason") == "user_denied" {
			reason = "user_denied"
		}
		http.Redirect(w, r, dest+"?error="+reason, http.StatusFound)
		return
	}
	code, state := q.Get("code"), q.Get("state")
	if code == "" || len(code) > 2048 || h.d.Social.CheckState(state) != nil {
		h.d.Log.Warn("meta oauth callback: invalid state or code")
		http.Redirect(w, r, dest+"?error=invalid_state", http.StatusFound)
		return
	}
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, dest+"?"+url.Values{"code": {code}, "state": {state}}.Encode(), http.StatusFound)
}

func (h *Webhooks) telegram(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "accountId"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	got := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(h.d.Social.WebhookSecret(id))) != 1 {
		http.Error(w, "invalid secret token", http.StatusForbidden)
		return
	}
	acct, err := h.d.Social.AccountByID(r.Context(), id)
	if err != nil || acct.Platform != PlatformTelegram {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return
	}
	if acct.Status == StatusDisconnected { // SECURITY I2: a disconnected bot no longer ingests
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ingested": 0})
		return
	}
	if !acct.WebhookVerified {
		_ = h.d.Social.MarkWebhookVerified(r.Context(), acct.TenantID, acct.ID)
	}
	msg, ok, err := ParseTelegramUpdate(body, acct.ExternalID)
	if err != nil || !ok {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ingested": 0})
		return
	}
	msg.AccountID = &acct.ID
	h.ingest(w, r.Context(), []InboundMessage{msg})
}

// smsBody accepts the common SMS-forwarder app shapes.
type smsBody struct {
	Text       string `json:"text"`
	Message    string `json:"message"`
	Body       string `json:"body"`
	From       string `json:"from"`
	ReceivedAt string `json:"received_at"`
	SentStamp  int64  `json:"sentStamp"`
}

func (h *Webhooks) sms(w http.ResponseWriter, r *http.Request) {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || strings.TrimSpace(tok) == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="sms"`)
		http.Error(w, "missing bearer token", http.StatusUnauthorized)
		return
	}
	tenantID, err := h.d.SMSTenant(r.Context(), strings.TrimSpace(tok))
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return
	}
	text, at := string(raw), time.Now()
	// The sender id is required (SECURITY H4): only known short codes / alphanumeric ids
	// may auto-verify payments. Plain-text bodies pass it as ?from= or X-SMS-From.
	from := firstNonEmpty(r.URL.Query().Get("from"), r.Header.Get("X-SMS-From"))
	if strings.Contains(r.Header.Get("Content-Type"), "json") {
		var b smsBody
		if err := json.Unmarshal(raw, &b); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		text = firstNonEmpty(b.Text, b.Message, b.Body)
		from = firstNonEmpty(b.From, from)
		if t, err := time.Parse(time.RFC3339, b.ReceivedAt); err == nil {
			at = t
		} else if b.SentStamp > 0 {
			at = time.UnixMilli(b.SentStamp)
		}
	}
	if strings.TrimSpace(text) == "" {
		http.Error(w, "empty sms", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(from) == "" {
		http.Error(w, "from (SMS sender) is required", http.StatusBadRequest)
		return
	}
	if err := h.d.SMSIngest(r.Context(), tenantID, strings.TrimSpace(from), text, at); err != nil {
		h.d.Log.Error("sms webhook: ingest failed", "tenant", tenantID, "err", err)
		http.Error(w, "ingest failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
