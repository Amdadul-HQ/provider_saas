package social

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/vault"
)

// Vault is the subset of platform/vault the social module uses.
type Vault interface {
	Store(ctx context.Context, tenantID uuid.UUID, kind string, plaintext []byte) (uuid.UUID, error)
	Update(ctx context.Context, tenantID, secretID uuid.UUID, plaintext []byte) error
	Open(ctx context.Context, tenantID, secretID uuid.UUID) ([]byte, error)
	Delete(ctx context.Context, tenantID, secretID uuid.UUID) error
}

// Config holds the Meta app + URLs used to build OAuth and webhook URLs.
type Config struct {
	AppID        string
	AppSecret    string
	GraphVersion string
	PublicAPIURL string // https://api.example.com (webhook + OAuth callback host)
	PublicAppURL string // https://app.example.com (frontend)
	// Secret signs OAuth state and derives Telegram webhook secrets (JWT secret).
	Secret []byte
}

// RedirectURI is the OAuth callback registered in the Meta app.
func (c Config) RedirectURI() string { return c.PublicAPIURL + "/webhooks/meta/oauth" }

// Service is the social module API.
type Service interface {
	List(ctx context.Context, tenantID uuid.UUID) ([]Account, error)
	Get(ctx context.Context, tenantID, id uuid.UUID) (Account, error)
	GetByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Account, error)

	// StartMetaConnect signs a state and stores its nonce (single use, bound to user+tenant).
	StartMetaConnect(ctx context.Context, tenantID, userID uuid.UUID, platform string) (OAuthURL, error)
	// CompleteMetaConnect (authenticated) consumes the state (it must have been issued to
	// this user in this tenant and not used before), exchanges the code for tokens, stores
	// every page (and linked Instagram account) and subscribes the pages to webhooks.
	CompleteMetaConnect(ctx context.Context, tenantID, userID uuid.UUID, code, state string) ([]Account, error)
	// CheckState verifies a state's signature and expiry without consuming it (OAuth redirect).
	CheckState(state string) error
	ConnectTelegram(ctx context.Context, tenantID, userID uuid.UUID, botToken string) (Account, error)
	ConnectWhatsApp(ctx context.Context, tenantID, userID uuid.UUID, in ConnectWhatsAppInput) (Account, error)
	Disconnect(ctx context.Context, tenantID, id uuid.UUID) error
	Sync(ctx context.Context, tenantID, id uuid.UUID) (Account, error)

	// ResolveInbound maps a webhook (platform, external id) to the connected account (no tenant scope).
	ResolveInbound(ctx context.Context, platform, externalID string) (Account, error)
	// AccountByID loads an account without tenant scope (Telegram webhook route).
	AccountByID(ctx context.Context, id uuid.UUID) (Account, error)
	MarkWebhookVerified(ctx context.Context, tenantID, id uuid.UUID) error
	// WebhookSecret is the expected Telegram secret-token header for an account.
	WebhookSecret(accountID uuid.UUID) string

	// Token decrypts the account's access token (server-side use only).
	Token(ctx context.Context, a Account) (string, error)
	// Send delivers one message via the platform adapter; auth failures flag the account.
	Send(ctx context.Context, a Account, thread, body string, attachments []Attachment) (string, error)
	Publish(ctx context.Context, a Account, body string, media []Attachment) (string, error)
}

type service struct {
	pool      *pgxpool.Pool
	repo      Repository
	vault     Vault
	client    Client
	sender    Sender
	publisher Publisher
	cfg       Config
	now       func() time.Time
}

// NewService wires the module. sender/publisher nil = platform adapters on client.
func NewService(pool *pgxpool.Pool, v Vault, client Client, sender Sender, publisher Publisher, cfg Config) Service {
	s := &service{pool: pool, vault: v, client: client, cfg: cfg, now: time.Now}
	if sender == nil {
		sender = NewPlatformSender(client, s)
	}
	if publisher == nil {
		publisher = ClientPublisher{Client: client, Tokens: s}
	}
	s.sender, s.publisher = sender, publisher
	return s
}

func (s *service) tx(ctx context.Context, tenantID uuid.UUID, fn db.TxFn) error {
	return db.WithTenantTx(ctx, s.pool, tenantID, fn)
}

func actor(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func strp(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func (s *service) List(ctx context.Context, tenantID uuid.UUID) (out []Account, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.List(ctx, tx, tenantID)
		return err
	})
	return out, err
}

func (s *service) Get(ctx context.Context, tenantID, id uuid.UUID) (out Account, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.Get(ctx, tx, tenantID, id)
		return err
	})
	return out, err
}

func (s *service) GetByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Account, error) {
	out := map[uuid.UUID]Account{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		as, err := s.repo.GetByIDs(ctx, tx, tenantID, ids)
		for _, a := range as {
			out[a.ID] = a
		}
		return err
	})
	return out, err
}

func (s *service) StartMetaConnect(ctx context.Context, tenantID, userID uuid.UUID, platform string) (OAuthURL, error) {
	platform = strings.ToLower(platform)
	if platform != PlatformFacebook && platform != PlatformInstagram {
		return OAuthURL{}, apperr.ValidationField("platform", "use connectWhatsApp / connectTelegram for this platform")
	}
	nonce, err := newNonce()
	if err != nil {
		return OAuthURL{}, err
	}
	exp := s.now().Add(StateTTL)
	if err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return s.repo.InsertState(ctx, tx, tenantID, userID, nonce, platform, exp)
	}); err != nil {
		return OAuthURL{}, err
	}
	state := SignState(s.cfg.Secret, State{TenantID: tenantID, UserID: userID, Platform: platform, Exp: exp.Unix(), Nonce: nonce})
	appID := s.cfg.AppID
	if appID == "" {
		appID = "dev-fake-app" // Fake client mode: the URL is not usable against Meta
	}
	q := url.Values{
		"client_id":     {appID},
		"redirect_uri":  {s.cfg.RedirectURI()},
		"state":         {state},
		"response_type": {"code"},
		"scope":         {strings.Join(MetaScopes, ",")},
	}
	return OAuthURL{URL: "https://www.facebook.com/" + s.cfg.GraphVersion + "/dialog/oauth?" + q.Encode(), State: state}, nil
}

func (s *service) CheckState(state string) error {
	_, err := VerifyState(s.cfg.Secret, state, s.now())
	return err
}

// errStateUsed hides whether a state was replayed, issued to someone else or expired.
var errStateUsed = apperr.ValidationField("state", "this connection link was already used or has expired; start again")

func (s *service) CompleteMetaConnect(ctx context.Context, tenantID, userID uuid.UUID, code, state string) ([]Account, error) {
	st, err := VerifyState(s.cfg.Secret, state, s.now())
	if err != nil {
		return nil, err
	}
	if st.TenantID != tenantID || st.UserID != userID {
		return nil, apperr.Forbidden("this connection was started by another user or business")
	}
	if strings.TrimSpace(code) == "" {
		return nil, apperr.ValidationField("code", "required")
	}
	// Single use (SECURITY M2): the nonce row is deleted before any token exchange.
	if err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ok, err := s.repo.ConsumeState(ctx, tx, tenantID, userID, st.Nonce, s.now())
		if err == nil && !ok {
			return errStateUsed
		}
		return err
	}); err != nil {
		return nil, err
	}
	return s.completeMeta(ctx, st.TenantID, userID, code)
}

// completeMeta performs the HTTP exchange first (outside any DB tx), then stores tokens and rows.
func (s *service) completeMeta(ctx context.Context, tenantID, userID uuid.UUID, code string) ([]Account, error) {
	if strings.TrimSpace(code) == "" {
		return nil, apperr.ValidationField("code", "required")
	}
	short, err := s.client.ExchangeCode(ctx, code, s.cfg.RedirectURI())
	if err != nil {
		return nil, apperr.Validation("facebook login failed: " + SafeError(err))
	}
	long, _, err := s.client.LongLivedToken(ctx, short)
	if err != nil {
		return nil, apperr.Validation("facebook token exchange failed: " + SafeError(err))
	}
	pages, err := s.client.Pages(ctx, long)
	if err != nil {
		return nil, apperr.Validation("could not list pages: " + SafeError(err))
	}
	if len(pages) == 0 {
		return nil, apperr.Validation("no Facebook pages found for this login; grant access to at least one page")
	}
	// Page tokens obtained from a long-lived user token do not expire.
	var out []Account
	for _, p := range pages {
		subErr := s.client.SubscribePage(ctx, p.ID, p.AccessToken)
		a, err := s.store(ctx, tenantID, userID, Account{Platform: PlatformFacebook, ExternalID: p.ID, Name: p.Name,
			AvatarURL: strp(p.PictureURL), Scopes: MetaScopes, WebhookVerified: subErr == nil}, p.AccessToken)
		if err != nil {
			return nil, err
		}
		if subErr != nil {
			msg := "webhook subscription failed: " + SafeError(subErr)
			if a, err = s.setStatus(ctx, tenantID, a.ID, StatusError, &msg, false); err != nil {
				return nil, err
			}
		}
		out = append(out, a)
		if p.Instagram != nil {
			ig, err := s.store(ctx, tenantID, userID, Account{Platform: PlatformInstagram, ExternalID: p.Instagram.ID,
				Name: firstNonEmpty(p.Instagram.Username, p.Name), Username: strp(p.Instagram.Username), AvatarURL: strp(p.Instagram.PictureURL),
				Scopes: MetaScopes, WebhookVerified: subErr == nil}, p.AccessToken)
			if err != nil {
				return nil, err
			}
			out = append(out, ig)
		}
	}
	return out, nil
}

func firstNonEmpty(v ...string) string {
	for _, x := range v {
		if x != "" {
			return x
		}
	}
	return ""
}

// store encrypts token (reusing the existing secret row on reconnect) and upserts the account.
func (s *service) store(ctx context.Context, tenantID, userID uuid.UUID, a Account, token string) (out Account, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		existing, err := s.repo.Find(ctx, tx, tenantID, a.Platform, a.ExternalID)
		switch {
		case err == nil && existing.TokenSecretID != nil:
			if err := s.vault.Update(ctx, tenantID, *existing.TokenSecretID, []byte(token)); err != nil {
				return err
			}
			a.TokenSecretID = existing.TokenSecretID
		case err == nil || errors.Is(err, pgx.ErrNoRows):
			id, err := s.vault.Store(ctx, tenantID, vault.KindSocialToken, []byte(token))
			if err != nil {
				return err
			}
			a.TokenSecretID = &id
		default:
			return err
		}
		a.ConnectedBy = actor(userID)
		out, err = s.repo.Upsert(ctx, tx, tenantID, a)
		if db.IsUniqueViolation(err, "social_accounts_external_active_key") {
			// SECURITY H5: one live connection per page/number/bot across all businesses.
			return apperr.Conflict("this " + a.Platform + " account is already connected to another business; " +
				"it must be disconnected there first")
		}
		return db.MapErr(err, "social account")
	})
	return out, err
}

func (s *service) setStatus(ctx context.Context, tenantID, id uuid.UUID, status string, lastErr *string, synced bool) (out Account, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.SetStatus(ctx, tx, tenantID, id, status, lastErr, synced)
		return err
	})
	return out, err
}

func (s *service) ConnectTelegram(ctx context.Context, tenantID, userID uuid.UUID, botToken string) (Account, error) {
	botToken = strings.TrimSpace(botToken)
	if !strings.Contains(botToken, ":") || len(botToken) < 20 {
		return Account{}, apperr.ValidationField("botToken", "looks invalid; copy it from @BotFather")
	}
	bot, err := s.client.TelegramGetMe(ctx, botToken)
	if err != nil {
		return Account{}, apperr.ValidationField("botToken", "Telegram rejected the token: "+SafeError(err))
	}
	name := firstNonEmpty(bot.FirstName, bot.Username)
	a, err := s.store(ctx, tenantID, userID, Account{Platform: PlatformTelegram, ExternalID: strconv.FormatInt(bot.ID, 10),
		Name: name, Username: strp(bot.Username), Scopes: []string{}}, botToken)
	if err != nil {
		return a, err
	}
	hook := fmt.Sprintf("%s/webhooks/telegram/%s", s.cfg.PublicAPIURL, a.ID)
	if err := s.client.TelegramSetWebhook(ctx, botToken, hook, s.WebhookSecret(a.ID)); err != nil {
		msg := "setWebhook failed: " + SafeError(err)
		return s.setStatus(ctx, tenantID, a.ID, StatusError, &msg, false)
	}
	if err := s.MarkWebhookVerified(ctx, tenantID, a.ID); err != nil {
		return a, err
	}
	a.WebhookVerified = true
	return a, nil
}

func (s *service) ConnectWhatsApp(ctx context.Context, tenantID, userID uuid.UUID, in ConnectWhatsAppInput) (Account, error) {
	if err := in.Validate(); err != nil {
		return Account{}, err
	}
	if err := s.client.VerifyToken(ctx, in.AccessToken); err != nil {
		return Account{}, apperr.ValidationField("accessToken", "Meta rejected the token: "+SafeError(err))
	}
	// SECURITY H5: the number must belong to the WhatsApp Business Account the token can
	// manage, otherwise anyone who knows a phone_number_id could route its inbox to themselves.
	nums, err := s.client.WhatsAppPhoneNumbers(ctx, in.BusinessAccountID, in.AccessToken)
	if err != nil {
		return Account{}, apperr.ValidationField("businessAccountId", "could not list the phone numbers of this WhatsApp Business Account: "+SafeError(err))
	}
	if !slices.Contains(nums, in.PhoneNumberID) {
		return Account{}, apperr.ValidationField("phoneNumberId", "this phone number id does not belong to the WhatsApp Business Account of the token")
	}
	// WhatsApp webhooks are configured on the Meta app (WABA subscription), not per number.
	return s.store(ctx, tenantID, userID, Account{Platform: PlatformWhatsApp, ExternalID: in.PhoneNumberID, Name: in.DisplayName,
		Username: strp(in.BusinessAccountID), Scopes: []string{"whatsapp_business_messaging"}, WebhookVerified: true}, in.AccessToken)
}

func (s *service) Disconnect(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		a, err := s.repo.Get(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		// Rows are kept (conversations reference them); only the token is destroyed.
		if err := s.repo.Disconnect(ctx, tx, tenantID, id); err != nil {
			return err
		}
		if a.TokenSecretID != nil {
			return s.vault.Delete(ctx, tenantID, *a.TokenSecretID)
		}
		return nil
	})
}

func (s *service) Sync(ctx context.Context, tenantID, id uuid.UUID) (Account, error) {
	a, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return a, err
	}
	if a.Status == StatusDisconnected {
		return a, apperr.Validation("account is disconnected; connect it again")
	}
	tok, err := s.Token(ctx, a)
	if err != nil {
		return a, err
	}
	var checkErr error
	switch a.Platform {
	case PlatformTelegram:
		bot, err := s.client.TelegramGetMe(ctx, tok)
		checkErr = err
		if err == nil {
			if err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
				return s.repo.UpdateProfile(ctx, tx, tenantID, id, firstNonEmpty(bot.FirstName, bot.Username), strp(bot.Username))
			}); err != nil {
				return a, err
			}
		}
	default:
		checkErr = s.client.VerifyToken(ctx, tok)
	}
	switch {
	case checkErr == nil:
		return s.setStatus(ctx, tenantID, id, StatusConnected, nil, true)
	case IsAuthError(checkErr):
		msg := SafeError(checkErr)
		return s.setStatus(ctx, tenantID, id, StatusTokenExpired, &msg, true)
	default:
		msg := SafeError(checkErr)
		return s.setStatus(ctx, tenantID, id, StatusError, &msg, true)
	}
}

// ResolveInbound routes a webhook. social_accounts is a tenant table: the lookup policy
// exposes only the row with this platform:external_id (unique among live connections).
func (s *service) ResolveInbound(ctx context.Context, platform, externalID string) (out Account, err error) {
	err = db.WithLookup(ctx, s.pool, db.LookupSocialKey, platform+":"+externalID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.ResolveInbound(ctx, tx, platform, externalID)
		return err
	})
	return out, err
}

func (s *service) AccountByID(ctx context.Context, id uuid.UUID) (out Account, err error) {
	err = db.WithLookup(ctx, s.pool, db.LookupSocialID, id.String(), func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.ByID(ctx, tx, id)
		return err
	})
	return out, err
}

func (s *service) MarkWebhookVerified(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return s.repo.MarkWebhookVerified(ctx, tx, tenantID, id)
	})
}

func (s *service) WebhookSecret(accountID uuid.UUID) string {
	return WebhookSecret(s.cfg.Secret, accountID)
}

func (s *service) Token(ctx context.Context, a Account) (string, error) {
	if a.TokenSecretID == nil {
		return "", apperr.Validation(a.Platform + " account " + a.Name + " has no stored token; reconnect it")
	}
	b, err := s.vault.Open(ctx, a.TenantID, *a.TokenSecretID)
	return string(b), err
}

func (s *service) Send(ctx context.Context, a Account, thread, body string, att []Attachment) (string, error) {
	if a.Status == StatusDisconnected {
		return "", apperr.Validation("the " + a.Platform + " account is disconnected")
	}
	id, err := s.sender.Send(ctx, a, thread, body, att)
	if err != nil && IsAuthError(err) {
		msg := SafeError(err)
		_, _ = s.setStatus(context.WithoutCancel(ctx), a.TenantID, a.ID, StatusTokenExpired, &msg, false)
	}
	return id, err
}

func (s *service) Publish(ctx context.Context, a Account, body string, media []Attachment) (string, error) {
	if a.Status == StatusDisconnected {
		return "", apperr.Validation("the " + a.Platform + " account is disconnected")
	}
	return s.publisher.Publish(ctx, a, body, media)
}

func newNonce() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
