package social

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
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

func (c *HTTPClient) do(ctx context.Context, method, rawURL string, body any, bearer string, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
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
		return json.Unmarshal(raw, out)
	}
	return nil
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
	err := c.do(ctx, http.MethodGet, c.graph("/oauth/access_token", url.Values{"client_id": {c.AppID},
		"client_secret": {c.AppSecret}, "redirect_uri": {redirectURI}, "code": {code}}), nil, "", &out)
	return out.AccessToken, err
}

func (c *HTTPClient) LongLivedToken(ctx context.Context, token string) (string, *time.Time, error) {
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	err := c.do(ctx, http.MethodGet, c.graph("/oauth/access_token", url.Values{"grant_type": {"fb_exchange_token"},
		"client_id": {c.AppID}, "client_secret": {c.AppSecret}, "fb_exchange_token": {token}}), nil, "", &out)
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
	err := c.do(ctx, http.MethodGet, c.graph("/me/accounts", url.Values{"limit": {"100"}, "access_token": {userToken},
		"fields": {"id,name,access_token,picture{url},instagram_business_account{id,username,profile_picture_url}"}}), nil, "", &out)
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
	return c.do(ctx, http.MethodPost, c.graph("/"+url.PathEscape(pageID)+"/subscribed_apps", url.Values{
		"subscribed_fields": {"messages,messaging_postbacks,message_echoes"}, "access_token": {pageToken}}), nil, "", nil)
}

func (c *HTTPClient) VerifyToken(ctx context.Context, token string) error {
	return c.do(ctx, http.MethodGet, c.graph("/me", url.Values{"fields": {"id"}, "access_token": {token}}), nil, "", nil)
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
	if err := c.do(ctx, http.MethodGet, c.tg(botToken, "getMe"), nil, "", &out); err != nil {
		return TelegramBot{}, err
	}
	return TelegramBot{ID: out.Result.ID, Username: out.Result.Username, FirstName: out.Result.FirstName}, nil
}

func (c *HTTPClient) TelegramSetWebhook(ctx context.Context, botToken, webhookURL, secret string) error {
	return c.do(ctx, http.MethodPost, c.tg(botToken, "setWebhook"), map[string]any{
		"url": webhookURL, "secret_token": secret, "allowed_updates": []string{"message", "edited_message"}}, "", nil)
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
		if err := c.do(ctx, http.MethodPost, c.graph(endpoint, url.Values{"access_token": {token}}), messengerBody(recipient, text, tag, a), "", &out); err != nil {
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
		if err := c.do(ctx, http.MethodPost, c.graph("/"+url.PathEscape(phoneNumberID)+"/messages", nil), body, token, &out); err != nil {
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
	if err := c.do(ctx, http.MethodPost, c.tg(botToken, method), body, "", &out); err != nil {
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
	q := url.Values{"access_token": {pageToken}}
	var err error
	if imageURL != "" {
		err = c.do(ctx, http.MethodPost, c.graph("/"+url.PathEscape(pageID)+"/photos", q), map[string]any{"url": imageURL, "caption": message}, "", &out)
	} else {
		err = c.do(ctx, http.MethodPost, c.graph("/"+url.PathEscape(pageID)+"/feed", q), map[string]any{"message": message}, "", &out)
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
	q := url.Values{"access_token": {token}}
	var container struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, c.graph("/"+url.PathEscape(igUserID)+"/media", q),
		map[string]any{"image_url": imageURL, "caption": caption}, "", &container); err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	err := c.do(ctx, http.MethodPost, c.graph("/"+url.PathEscape(igUserID)+"/media_publish", q),
		map[string]any{"creation_id": container.ID}, "", &out)
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
	return []MetaPage{{ID: "100200300400500", Name: "Demo Digital Store", AccessToken: "fake-page-token-1",
		Instagram: &MetaInstagram{ID: "17841400000000001", Username: "demo.digital.store"}}}, nil
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
