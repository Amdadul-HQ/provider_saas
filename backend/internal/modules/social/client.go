package social

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"provider_saas/internal/platform/apperr"
)

// MetaPage is a Facebook page the user manages (from /me/accounts), with its linked
// Instagram business account when present.
type MetaPage struct {
	ID          string
	Name        string
	AccessToken string
	PictureURL  string
	Instagram   *MetaInstagram
}

type MetaInstagram struct {
	ID         string
	Username   string
	PictureURL string
}

type TelegramBot struct {
	ID        int64
	Username  string
	FirstName string
}

// Client is every outbound HTTP call to Meta Graph API and Telegram Bot API. The real
// implementation is HTTPClient; Fake is used in tests and in dev when META_APP_ID is empty.
type Client interface {
	// Meta OAuth + pages
	ExchangeCode(ctx context.Context, code, redirectURI string) (string, error)
	LongLivedToken(ctx context.Context, token string) (string, *time.Time, error)
	Pages(ctx context.Context, userToken string) ([]MetaPage, error)
	SubscribePage(ctx context.Context, pageID, pageToken string) error
	VerifyToken(ctx context.Context, token string) error
	// WhatsAppPhoneNumbers lists the phone number ids of a WABA visible to token.
	WhatsAppPhoneNumbers(ctx context.Context, wabaID, token string) ([]string, error)
	// Telegram bot setup
	TelegramGetMe(ctx context.Context, botToken string) (TelegramBot, error)
	TelegramSetWebhook(ctx context.Context, botToken, webhookURL, secret string) error
	// Messaging
	SendMessenger(ctx context.Context, pageToken, psid, text, tag string, att []Attachment) (string, error)
	SendInstagram(ctx context.Context, igUserID, token, igsid, text, tag string, att []Attachment) (string, error)
	SendWhatsApp(ctx context.Context, phoneNumberID, token, to, text string, att []Attachment) (string, error)
	SendTelegram(ctx context.Context, botToken, chatID, text string, att []Attachment) (string, error)
	// Publishing
	PublishFacebook(ctx context.Context, pageID, pageToken, message, imageURL string) (string, error)
	PublishInstagram(ctx context.Context, igUserID, token, caption, imageURL string) (string, error)
	PublishTelegram(ctx context.Context, botToken, chatID, text, photoURL string) (string, error)
}

// APIError is a non-2xx response from Meta/Telegram.
type APIError struct {
	Status int
	Msg    string
}

func (e *APIError) Error() string { return fmt.Sprintf("social api %d: %s", e.Status, e.Msg) }

// IsAuthError reports whether err means the stored token is no longer valid.
func IsAuthError(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && (ae.Status == http.StatusUnauthorized || strings.Contains(strings.ToLower(ae.Msg), "access token"))
}

// TransportError is a network-level failure talking to Meta/Telegram. It carries only the
// operation and host: *url.Error would include the full URL, and with it access tokens,
// the app secret or the Telegram bot token (SECURITY M3).
type TransportError struct {
	Op   string
	Host string
	Err  error
}

func (e *TransportError) Error() string {
	return fmt.Sprintf("social api %s %s: %s", e.Op, e.Host, transportReason(e.Err))
}

func (e *TransportError) Unwrap() error { return e.Err }

// transportReason keeps the error class but never the URL.
func transportReason(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "timeout"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns lookup failed"
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return "connection failed (" + opErr.Op + ")"
	}
	return "request failed"
}

// SafeError renders an upstream error for users, logs and stored last_error fields: API
// errors keep the provider message, transport errors only op + host, anything else is
// replaced by a generic text (it might embed a URL).
func SafeError(err error) string {
	var ae *APIError
	var te *TransportError
	switch {
	case err == nil:
		return ""
	case errors.As(err, &ae):
		return ae.Error()
	case errors.As(err, &te):
		return te.Error()
	case errors.Is(err, ErrUnsupported), apperr.Code(err) != "":
		return err.Error() // our own domain errors carry no URLs
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		return "social api: " + transportReason(err)
	}
	return "social api: request failed"
}

// ---- real implementation

type HTTPClient struct {
	AppID, AppSecret string
	GraphBase        string // https://graph.facebook.com/v21.0
	TelegramBase     string // https://api.telegram.org
	HTTP             *http.Client
}

func NewHTTPClient(appID, appSecret, graphVersion string) *HTTPClient {
	return &HTTPClient{AppID: appID, AppSecret: appSecret, GraphBase: "https://graph.facebook.com/" + graphVersion,
		TelegramBase: "https://api.telegram.org", HTTP: &http.Client{Timeout: 20 * time.Second}}
}

// req describes one outbound call. Meta access tokens go in the Authorization header with
// appsecret_proof, never in the URL; app credentials go in a POST form body.
type req struct {
	method string
	url    string
	json   any        // JSON body
	form   url.Values // form body (takes precedence over json)
	token  string     // Meta access token -> "Authorization: Bearer" + appsecret_proof
}

// AppSecretProof is HMAC-SHA256(access_token, app_secret) in hex (Meta "Require App Secret").
func AppSecretProof(token, appSecret string) string {
	m := hmac.New(sha256.New, []byte(appSecret))
	m.Write([]byte(token))
	return hex.EncodeToString(m.Sum(nil))
}

func (c *HTTPClient) do(ctx context.Context, r req, out any) error {
	var rd io.Reader
	ctype := ""
	switch {
	case r.form != nil:
		rd, ctype = strings.NewReader(r.form.Encode()), "application/x-www-form-urlencoded"
	case r.json != nil:
		b, err := json.Marshal(r.json)
		if err != nil {
			return err
		}
		rd, ctype = bytes.NewReader(b), "application/json"
	}
	u := r.url
	if r.token != "" && c.AppSecret != "" {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + "appsecret_proof=" + AppSecretProof(r.token, c.AppSecret)
	}
	hreq, err := http.NewRequestWithContext(ctx, r.method, u, rd)
	if err != nil {
		return &TransportError{Op: r.method, Host: hostOf(r.url), Err: errors.New("bad request")}
	}
	if ctype != "" {
		hreq.Header.Set("Content-Type", ctype)
	}
	if r.token != "" {
		hreq.Header.Set("Authorization", "Bearer "+r.token)
	}
	resp, err := c.HTTP.Do(hreq)
	if err != nil {
		return &TransportError{Op: r.method, Host: hreq.URL.Host, Err: err}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
			Description string `json:"description"` // telegram
		}
		_ = json.Unmarshal(raw, &e)
		msg := e.Error.Message
		if msg == "" {
			msg = e.Description
		}
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return &APIError{Status: resp.StatusCode, Msg: msg}
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return &APIError{Status: resp.StatusCode, Msg: "unexpected response"}
		}
	}
	return nil
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		return u.Host
	}
	return "?"
}

func (c *HTTPClient) graph(path string, q url.Values) string {
	u := c.GraphBase + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

func (c *HTTPClient) ExchangeCode(ctx context.Context, code, redirectURI string) (string, error) {
	var out struct {
		AccessToken string `json:"access_token"`
	}
	err := c.do(ctx, req{method: http.MethodPost, url: c.graph("/oauth/access_token", nil), form: url.Values{"client_id": {c.AppID},
		"client_secret": {c.AppSecret}, "redirect_uri": {redirectURI}, "code": {code}}}, &out)
	return out.AccessToken, err
}

func (c *HTTPClient) LongLivedToken(ctx context.Context, token string) (string, *time.Time, error) {
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	err := c.do(ctx, req{method: http.MethodPost, url: c.graph("/oauth/access_token", nil), form: url.Values{"grant_type": {"fb_exchange_token"},
		"client_id": {c.AppID}, "client_secret": {c.AppSecret}, "fb_exchange_token": {token}}}, &out)
	if err != nil {
		return "", nil, err
	}
	var exp *time.Time
	if out.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)
		exp = &t
	}
	return out.AccessToken, exp, nil
}

func (c *HTTPClient) Pages(ctx context.Context, userToken string) ([]MetaPage, error) {
	var out struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			AccessToken string `json:"access_token"`
			Picture     struct {
				Data struct {
					URL string `json:"url"`
				} `json:"data"`
			} `json:"picture"`
			Instagram *struct {
				ID       string `json:"id"`
				Username string `json:"username"`
				Picture  string `json:"profile_picture_url"`
			} `json:"instagram_business_account"`
		} `json:"data"`
	}
	err := c.do(ctx, req{method: http.MethodGet, token: userToken, url: c.graph("/me/accounts", url.Values{"limit": {"100"},
		"fields": {"id,name,access_token,picture{url},instagram_business_account{id,username,profile_picture_url}"}})}, &out)
	if err != nil {
		return nil, err
	}
	pages := make([]MetaPage, 0, len(out.Data))
	for _, d := range out.Data {
		p := MetaPage{ID: d.ID, Name: d.Name, AccessToken: d.AccessToken, PictureURL: d.Picture.Data.URL}
		if d.Instagram != nil && d.Instagram.ID != "" {
			p.Instagram = &MetaInstagram{ID: d.Instagram.ID, Username: d.Instagram.Username, PictureURL: d.Instagram.Picture}
		}
		pages = append(pages, p)
	}
	return pages, nil
}

func (c *HTTPClient) SubscribePage(ctx context.Context, pageID, pageToken string) error {
	return c.do(ctx, req{method: http.MethodPost, token: pageToken, url: c.graph("/"+url.PathEscape(pageID)+"/subscribed_apps", nil),
		form: url.Values{"subscribed_fields": {"messages,messaging_postbacks,message_echoes"}}}, nil)
}

func (c *HTTPClient) VerifyToken(ctx context.Context, token string) error {
	return c.do(ctx, req{method: http.MethodGet, token: token, url: c.graph("/me", url.Values{"fields": {"id"}})}, nil)
}

// WhatsAppPhoneNumbers lists the phone number ids of a WhatsApp Business Account that the
// token can see (GET /{waba-id}/phone_numbers); used to prove number ownership (H5).
func (c *HTTPClient) WhatsAppPhoneNumbers(ctx context.Context, wabaID, token string) ([]string, error) {
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.do(ctx, req{method: http.MethodGet, token: token, url: c.graph("/"+url.PathEscape(wabaID)+"/phone_numbers",
		url.Values{"fields": {"id"}, "limit": {"100"}})}, &out); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(out.Data))
	for _, d := range out.Data {
		ids = append(ids, d.ID)
	}
	return ids, nil
}

func (c *HTTPClient) tg(botToken, method string) string {
	return c.TelegramBase + "/bot" + botToken + "/" + method
}

func (c *HTTPClient) TelegramGetMe(ctx context.Context, botToken string) (TelegramBot, error) {
	var out struct {
		Result struct {
			ID        int64  `json:"id"`
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
		} `json:"result"`
	}
	if err := c.do(ctx, req{method: http.MethodGet, url: c.tg(botToken, "getMe")}, &out); err != nil {
		return TelegramBot{}, err
	}
	return TelegramBot{ID: out.Result.ID, Username: out.Result.Username, FirstName: out.Result.FirstName}, nil
}

func (c *HTTPClient) TelegramSetWebhook(ctx context.Context, botToken, webhookURL, secret string) error {
	return c.do(ctx, req{method: http.MethodPost, url: c.tg(botToken, "setWebhook"), json: map[string]any{
		"url": webhookURL, "secret_token": secret, "allowed_updates": []string{"message", "edited_message"}}}, nil)
}

func messengerBody(recipient, text, tag string, att *Attachment) map[string]any {
	msg := map[string]any{}
	if att != nil {
		typ := att.Type
		if typ != "image" && typ != "video" && typ != "audio" {
			typ = "file"
		}
		msg["attachment"] = map[string]any{"type": typ, "payload": map[string]any{"url": att.URL, "is_reusable": true}}
	} else {
		msg["text"] = text
	}
	body := map[string]any{"recipient": map[string]string{"id": recipient}, "message": msg, "messaging_type": "RESPONSE"}
	if tag != "" {
		body["messaging_type"], body["tag"] = "MESSAGE_TAG", tag
	}
	return body
}

// sendMeta sends text and then each URL attachment; returns the first message id.
func (c *HTTPClient) sendMeta(ctx context.Context, endpoint, token, recipient, text, tag string, att []Attachment) (string, error) {
	var first string
	send := func(a *Attachment) error {
		var out struct {
			MessageID string `json:"message_id"`
		}
		if err := c.do(ctx, req{method: http.MethodPost, url: c.graph(endpoint, nil), token: token, json: messengerBody(recipient, text, tag, a)}, &out); err != nil {
			return err
		}
		if first == "" {
			first = out.MessageID
		}
		return nil
	}
	if strings.TrimSpace(text) != "" {
		if err := send(nil); err != nil {
			return "", err
		}
	}
	for i := range att {
		if att[i].URL == "" {
			continue
		}
		if err := send(&att[i]); err != nil {
			return first, err
		}
	}
	return first, nil
}

func (c *HTTPClient) SendMessenger(ctx context.Context, pageToken, psid, text, tag string, att []Attachment) (string, error) {
	return c.sendMeta(ctx, "/me/messages", pageToken, psid, text, tag, att)
}

func (c *HTTPClient) SendInstagram(ctx context.Context, igUserID, token, igsid, text, tag string, att []Attachment) (string, error) {
	return c.sendMeta(ctx, "/"+url.PathEscape(igUserID)+"/messages", token, igsid, text, tag, att)
}

func (c *HTTPClient) SendWhatsApp(ctx context.Context, phoneNumberID, token, to, text string, att []Attachment) (string, error) {
	var first string
	send := func(body map[string]any) error {
		body["messaging_product"], body["to"] = "whatsapp", to
		var out struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
		}
		if err := c.do(ctx, req{method: http.MethodPost, url: c.graph("/"+url.PathEscape(phoneNumberID)+"/messages", nil), token: token, json: body}, &out); err != nil {
			return err
		}
		if first == "" && len(out.Messages) > 0 {
			first = out.Messages[0].ID
		}
		return nil
	}
	if strings.TrimSpace(text) != "" {
		if err := send(map[string]any{"type": "text", "text": map[string]any{"body": text}}); err != nil {
			return "", err
		}
	}
	for _, a := range att {
		if a.URL == "" {
			continue
		}
		typ := a.Type
		if typ != "image" && typ != "video" && typ != "audio" {
			typ = "document"
		}
		if err := send(map[string]any{"type": typ, typ: map[string]any{"link": a.URL}}); err != nil {
			return first, err
		}
	}
	return first, nil
}

func (c *HTTPClient) telegramSend(ctx context.Context, botToken, method string, body map[string]any) (string, error) {
	var out struct {
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	if err := c.do(ctx, req{method: http.MethodPost, url: c.tg(botToken, method), json: body}, &out); err != nil {
		return "", err
	}
	return strconv.FormatInt(out.Result.MessageID, 10), nil
}

func (c *HTTPClient) SendTelegram(ctx context.Context, botToken, chatID, text string, att []Attachment) (string, error) {
	for _, a := range att {
		if a.URL == "" && a.ID == "" {
			continue
		}
		file := a.URL
		if file == "" {
			file = a.ID
		}
		method, field := "sendDocument", "document"
		switch a.Type {
		case "image":
			method, field = "sendPhoto", "photo"
		case "video":
			method, field = "sendVideo", "video"
		case "audio":
			method, field = "sendAudio", "audio"
		}
		// The first attachment carries the text as caption.
		return c.telegramSend(ctx, botToken, method, map[string]any{"chat_id": chatID, field: file, "caption": text})
	}
	return c.telegramSend(ctx, botToken, "sendMessage", map[string]any{"chat_id": chatID, "text": text})
}

func (c *HTTPClient) PublishFacebook(ctx context.Context, pageID, pageToken, message, imageURL string) (string, error) {
	var out struct {
		ID     string `json:"id"`
		PostID string `json:"post_id"`
	}
	var err error
	if imageURL != "" {
		err = c.do(ctx, req{method: http.MethodPost, url: c.graph("/"+url.PathEscape(pageID)+"/photos", nil), token: pageToken,
			json: map[string]any{"url": imageURL, "caption": message}}, &out)
	} else {
		err = c.do(ctx, req{method: http.MethodPost, url: c.graph("/"+url.PathEscape(pageID)+"/feed", nil), token: pageToken,
			json: map[string]any{"message": message}}, &out)
	}
	if out.PostID != "" {
		return out.PostID, err
	}
	return out.ID, err
}

func (c *HTTPClient) PublishInstagram(ctx context.Context, igUserID, token, caption, imageURL string) (string, error) {
	if imageURL == "" {
		return "", &APIError{Status: http.StatusBadRequest, Msg: "Instagram posts need an image"}
	}
	var container struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, req{method: http.MethodPost, url: c.graph("/"+url.PathEscape(igUserID)+"/media", nil), token: token,
		json: map[string]any{"image_url": imageURL, "caption": caption}}, &container); err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	err := c.do(ctx, req{method: http.MethodPost, url: c.graph("/"+url.PathEscape(igUserID)+"/media_publish", nil), token: token,
		json: map[string]any{"creation_id": container.ID}}, &out)
	return out.ID, err
}

func (c *HTTPClient) PublishTelegram(ctx context.Context, botToken, chatID, text, photoURL string) (string, error) {
	if photoURL != "" {
		return c.telegramSend(ctx, botToken, "sendPhoto", map[string]any{"chat_id": chatID, "photo": photoURL, "caption": text})
	}
	return c.telegramSend(ctx, botToken, "sendMessage", map[string]any{"chat_id": chatID, "text": text})
}

// ---- fake implementation

// FakeCall records one call made to the Fake client.
type FakeCall struct {
	Method string
	Args   []string
}

// Fake is an in-memory Client. Tokens containing "invalid" fail with an auth error.
type Fake struct {
	mu    sync.Mutex
	n     int
	Calls []FakeCall
	// Fail, when set, makes every send/publish fail with this error.
	Fail error
	// wabaNumbers: phone number ids per WhatsApp Business Account (AddWABANumber).
	wabaNumbers map[string][]string
}

// AddWABANumber registers phoneNumberID as belonging to wabaID.
func (f *Fake) AddWABANumber(wabaID, phoneNumberID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.wabaNumbers == nil {
		f.wabaNumbers = map[string][]string{}
	}
	f.wabaNumbers[wabaID] = append(f.wabaNumbers[wabaID], phoneNumberID)
}

// WhatsAppPhoneNumbers returns the numbers registered for wabaID plus, for manual testing
// in dev without a Meta app, "<wabaID>-1", "<wabaID>-2" and "<wabaID>-3".
func (f *Fake) WhatsAppPhoneNumbers(_ context.Context, wabaID, token string) ([]string, error) {
	f.record("WhatsAppPhoneNumbers", wabaID)
	if err := badToken(token); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append(append([]string{}, f.wabaNumbers[wabaID]...), wabaID+"-1", wabaID+"-2", wabaID+"-3"), nil
}

func NewFake() *Fake { return &Fake{} }

func (f *Fake) record(method string, args ...string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	f.Calls = append(f.Calls, FakeCall{Method: method, Args: args})
	return fmt.Sprintf("fake-%s-%d", strings.ToLower(method), f.n)
}

// CallsOf returns recorded calls of one method.
func (f *Fake) CallsOf(method string) []FakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []FakeCall
	for _, c := range f.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

func badToken(t string) error {
	if t == "" || strings.Contains(t, "invalid") {
		return &APIError{Status: http.StatusUnauthorized, Msg: "Invalid OAuth access token"}
	}
	return nil
}

func (f *Fake) ExchangeCode(_ context.Context, code, _ string) (string, error) {
	f.record("ExchangeCode", code)
	if err := badToken(code); err != nil {
		return "", err
	}
	return "fake-user-token-" + code, nil
}

func (f *Fake) LongLivedToken(_ context.Context, token string) (string, *time.Time, error) {
	f.record("LongLivedToken", token)
	exp := time.Now().Add(60 * 24 * time.Hour)
	return "long-" + token, &exp, nil
}

func (f *Fake) Pages(_ context.Context, userToken string) ([]MetaPage, error) {
	f.record("Pages", userToken)
	if err := badToken(userToken); err != nil {
		return nil, err
	}
	// Page ids derive from the login (code) so different fake logins own different pages
	// (page ids are unique across businesses, SECURITY H5).
	h := fnv.New64a()
	_, _ = h.Write([]byte(userToken))
	n := h.Sum64() % 1_000_000_000_000
	return []MetaPage{{ID: fmt.Sprintf("10%013d", n), Name: "Demo Digital Store", AccessToken: "fake-page-token-" + strconv.FormatUint(n, 10),
		Instagram: &MetaInstagram{ID: fmt.Sprintf("178414%011d", n%100_000_000_000), Username: "demo.digital.store"}}}, nil
}

func (f *Fake) SubscribePage(_ context.Context, pageID, pageToken string) error {
	f.record("SubscribePage", pageID)
	return badToken(pageToken)
}

func (f *Fake) VerifyToken(_ context.Context, token string) error {
	f.record("VerifyToken")
	return badToken(token)
}

func (f *Fake) TelegramGetMe(_ context.Context, botToken string) (TelegramBot, error) {
	f.record("TelegramGetMe")
	if err := badToken(botToken); err != nil {
		return TelegramBot{}, err
	}
	idPart, _, ok := strings.Cut(botToken, ":")
	id, err := strconv.ParseInt(idPart, 10, 64)
	if !ok || err != nil {
		return TelegramBot{}, &APIError{Status: http.StatusUnauthorized, Msg: "Unauthorized"}
	}
	return TelegramBot{ID: id, Username: fmt.Sprintf("demo%d_bot", id%1000), FirstName: "Demo Bot"}, nil
}

func (f *Fake) TelegramSetWebhook(_ context.Context, botToken, webhookURL, _ string) error {
	f.record("TelegramSetWebhook", webhookURL)
	return badToken(botToken)
}

func (f *Fake) send(method, token, to, text string) (string, error) {
	if f.Fail != nil {
		return "", f.Fail
	}
	id := f.record(method, to, text)
	return id, badToken(token)
}

func (f *Fake) SendMessenger(_ context.Context, pageToken, psid, text, _ string, _ []Attachment) (string, error) {
	return f.send("SendMessenger", pageToken, psid, text)
}

func (f *Fake) SendInstagram(_ context.Context, _, token, igsid, text, _ string, _ []Attachment) (string, error) {
	return f.send("SendInstagram", token, igsid, text)
}

func (f *Fake) SendWhatsApp(_ context.Context, _, token, to, text string, _ []Attachment) (string, error) {
	return f.send("SendWhatsApp", token, to, text)
}

func (f *Fake) SendTelegram(_ context.Context, botToken, chatID, text string, _ []Attachment) (string, error) {
	return f.send("SendTelegram", botToken, chatID, text)
}

func (f *Fake) PublishFacebook(_ context.Context, pageID, pageToken, message, _ string) (string, error) {
	return f.send("PublishFacebook", pageToken, pageID, message)
}

func (f *Fake) PublishInstagram(_ context.Context, igUserID, token, caption, imageURL string) (string, error) {
	if imageURL == "" {
		return "", &APIError{Status: http.StatusBadRequest, Msg: "Instagram posts need an image"}
	}
	return f.send("PublishInstagram", token, igUserID, caption)
}

func (f *Fake) PublishTelegram(_ context.Context, botToken, chatID, text, _ string) (string, error) {
	return f.send("PublishTelegram", botToken, chatID, text)
}
