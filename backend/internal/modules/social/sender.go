package social

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Sender delivers one outbound message on a connected account. thread is the external
// thread id (PSID / IGSID / wa_id / chat id). Returns the platform message id.
type Sender interface {
	Send(ctx context.Context, account Account, thread, body string, attachments []Attachment) (string, error)
}

// Publisher publishes a post (body + optional media) to an account's feed/channel.
type Publisher interface {
	Publish(ctx context.Context, account Account, body string, media []Attachment) (string, error)
}

// TokenSource returns the plaintext access token of an account (from the vault).
type TokenSource interface {
	Token(ctx context.Context, account Account) (string, error)
}

type tagKey struct{}

// WithMessageTag marks an outbound Messenger/Instagram message as a tagged message
// (sent outside the 24h window, e.g. "ACCOUNT_UPDATE" for renewal reminders).
func WithMessageTag(ctx context.Context, tag string) context.Context {
	return context.WithValue(ctx, tagKey{}, tag)
}

func messageTag(ctx context.Context) string {
	t, _ := ctx.Value(tagKey{}).(string)
	return t
}

// ErrUnsupported is returned when a platform cannot perform the operation.
var ErrUnsupported = errors.New("operation not supported on this platform")

// ---- per-platform adapters

type MessengerSender struct {
	Client Client
	Tokens TokenSource
}

func (s MessengerSender) Send(ctx context.Context, a Account, thread, body string, att []Attachment) (string, error) {
	tok, err := s.Tokens.Token(ctx, a)
	if err != nil {
		return "", err
	}
	return s.Client.SendMessenger(ctx, tok, thread, body, messageTag(ctx), att)
}

type InstagramSender struct {
	Client Client
	Tokens TokenSource
}

func (s InstagramSender) Send(ctx context.Context, a Account, thread, body string, att []Attachment) (string, error) {
	tok, err := s.Tokens.Token(ctx, a)
	if err != nil {
		return "", err
	}
	return s.Client.SendInstagram(ctx, a.ExternalID, tok, thread, body, messageTag(ctx), att)
}

type WhatsAppSender struct {
	Client Client
	Tokens TokenSource
}

func (s WhatsAppSender) Send(ctx context.Context, a Account, thread, body string, att []Attachment) (string, error) {
	tok, err := s.Tokens.Token(ctx, a)
	if err != nil {
		return "", err
	}
	if tpl, ok := whatsAppTemplateFrom(ctx); ok {
		return s.Client.SendWhatsAppTemplate(ctx, a.ExternalID, tok, thread, tpl)
	}
	return s.Client.SendWhatsApp(ctx, a.ExternalID, tok, thread, body, att)
}

type TelegramSender struct {
	Client Client
	Tokens TokenSource
}

func (s TelegramSender) Send(ctx context.Context, a Account, thread, body string, att []Attachment) (string, error) {
	tok, err := s.Tokens.Token(ctx, a)
	if err != nil {
		return "", err
	}
	return s.Client.SendTelegram(ctx, tok, thread, body, att)
}

// PlatformSender dispatches to the adapter of the account's platform.
type PlatformSender map[string]Sender

// NewPlatformSender builds the standard adapters on one Client.
func NewPlatformSender(c Client, tokens TokenSource) PlatformSender {
	return PlatformSender{
		PlatformFacebook:  MessengerSender{c, tokens},
		PlatformInstagram: InstagramSender{c, tokens},
		PlatformWhatsApp:  WhatsAppSender{c, tokens},
		PlatformTelegram:  TelegramSender{c, tokens},
	}
}

func (p PlatformSender) Send(ctx context.Context, a Account, thread, body string, att []Attachment) (string, error) {
	s, ok := p[a.Platform]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupported, a.Platform)
	}
	return s.Send(ctx, a, thread, body, att)
}

// SentMessage is one message captured by FakeSender.
type SentMessage struct {
	Account Account
	Thread  string
	Body    string
	Tag     string
	// Template is set when a WhatsApp template message was requested (outside 24h).
	Template *WhatsAppTemplate
}

// FakeSender records messages instead of sending them (tests, dev without credentials).
type FakeSender struct {
	mu   sync.Mutex
	Sent []SentMessage
	Fail error
}

func (f *FakeSender) Send(ctx context.Context, a Account, thread, body string, _ []Attachment) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Fail != nil {
		return "", f.Fail
	}
	sm := SentMessage{Account: a, Thread: thread, Body: body, Tag: messageTag(ctx)}
	if tpl, ok := whatsAppTemplateFrom(ctx); ok {
		sm.Template = &tpl
	}
	f.Sent = append(f.Sent, sm)
	return fmt.Sprintf("fake-out-%d", len(f.Sent)), nil
}

// Messages returns a copy of what was sent.
func (f *FakeSender) Messages() []SentMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]SentMessage(nil), f.Sent...)
}

// ---- publishing

// ClientPublisher publishes through the Graph / Bot APIs.
type ClientPublisher struct {
	Client Client
	Tokens TokenSource
}

func firstImage(media []Attachment) string {
	for _, m := range media {
		if m.URL != "" && (m.Type == "" || m.Type == "image") {
			return m.URL
		}
	}
	return ""
}

func (p ClientPublisher) Publish(ctx context.Context, a Account, body string, media []Attachment) (string, error) {
	tok, err := p.Tokens.Token(ctx, a)
	if err != nil {
		return "", err
	}
	img := firstImage(media)
	switch a.Platform {
	case PlatformFacebook:
		return p.Client.PublishFacebook(ctx, a.ExternalID, tok, body, img)
	case PlatformInstagram:
		return p.Client.PublishInstagram(ctx, a.ExternalID, tok, body, img)
	case PlatformTelegram:
		ch := a.TelegramChannel()
		if ch == "" {
			return "", errors.New("telegram: no channel configured for this bot (add the bot to a channel and set channel:<id>)")
		}
		return p.Client.PublishTelegram(ctx, tok, ch, body, img)
	}
	return "", fmt.Errorf("%w: %s posts", ErrUnsupported, a.Platform)
}

// FakePublisher records posts.
type FakePublisher struct {
	mu        sync.Mutex
	Published []SentMessage
	// FailFor makes publishing to these platforms fail (tests of partial status).
	FailFor map[string]error
}

func (f *FakePublisher) Publish(_ context.Context, a Account, body string, _ []Attachment) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.FailFor[a.Platform]; err != nil {
		return "", err
	}
	if a.Platform == PlatformWhatsApp {
		return "", fmt.Errorf("%w: whatsapp posts", ErrUnsupported)
	}
	f.Published = append(f.Published, SentMessage{Account: a, Body: body})
	return fmt.Sprintf("fake-post-%d", len(f.Published)), nil
}
