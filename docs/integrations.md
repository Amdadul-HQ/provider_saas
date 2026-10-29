# Integrations & webhooks (REST, outside GraphQL — ADR-003)

All webhook routes live under `backend/internal/platform/httpx` → `internal/modules/social/webhooks.go` and `billing/webhooks.go`.

| Route | Method | Who calls | Verification | Effect |
|---|---|---|---|---|
| `/webhooks/meta` | GET | Meta (subscribe) | `hub.verify_token == META_VERIFY_TOKEN` → echo `hub.challenge` | — |
| `/webhooks/meta` | POST | Meta (Messenger, Instagram, WhatsApp Cloud API) | HMAC-SHA256 of raw body with `META_APP_SECRET` vs `X-Hub-Signature-256` | upsert `customer_identities` + `conversations` + `messages`; auto-create customer; enqueue `inbox.autoreply` if rule matches |
| `/webhooks/meta/oauth` | GET | Browser redirect after Facebook Login | `state` signature + expiry (not consumed here) | **connects nothing** (SECURITY M2): redirects to `PUBLIC_APP_URL/social?code=..&state=..`; the signed-in SPA calls `completeMetaConnect`, which consumes the single-use state (bound to the user + tenant that called `startMetaConnect`, stored in `oauth_states`), then exchanges code → long-lived token → pages → vault → `social_accounts` → page webhook subscription. Errors redirect to `/social?error=denied|user_denied|invalid_state` (no upstream text) |
| `/webhooks/telegram/{accountId}` | POST | Telegram Bot API | `X-Telegram-Bot-Api-Secret-Token` equals per-account secret | same ingest as Meta |
| `/webhooks/payments/sms` | POST | SMS-forwarder app on provider's phone | `Authorization: Bearer <tenant sms token>` (vault; returned once by `rotateSmsWebhookToken`) | **`from` (SMS sender id) is required** (JSON `from`, or `?from=` / `X-SMS-From` for text bodies; 400 without it). Insert `payment_inbox` (with `sender`), parse bKash/Nagad SMS; auto-verify a pending payment (same method + TrxID + amount, not overpaying) **only** when the sender is allowed for the method: defaults `bkash: bKash, 16247` · `nagad: NAGAD, 16167` · `rocket: Rocket, 16216`, overridable per tenant with `settings.payment_sms_senders` (`{"bkash": ["..."]}`). Anything else is stored as `needs_review` and never auto-verifies (SECURITY H4) |
| `/uploads` | POST | frontend (multipart) | JWT | store file (local `./uploads` in dev, S3-compatible via env in prod), return URL |

## Message ingest contract (platform-agnostic)
```go
type InboundMessage struct {
  Platform        string // facebook|instagram|whatsapp|telegram
  AccountExternal string // page id / phone_number_id / bot id
  ThreadExternal  string // PSID / wa_id / chat id
  MessageExternal string // mid / wamid / message_id (idempotency key)
  SenderName      string
  SenderAvatar    string
  Kind            string // text|image|video|audio|file|sticker
  Body            string
  Attachments     []Attachment
  SentAt          time.Time
}
```
`inbox.Service.Ingest(ctx, msg)` is the single entry point: resolves `social_accounts` by (platform, AccountExternal) → tenant; upserts identity → customer (auto-create with source=platform); upserts conversation; inserts message (ignore duplicate external id); bumps unread + last_message; emits `notification`.

## Outbound send
`inbox.Service.Send(ctx, tenantID, conversationID, body, attachments)` → picks adapter by platform:
- Messenger Send API `POST /v21.0/me/messages` (page token from vault). Respect 24h policy (`last_inbound_at`); outside it use `MESSAGE_TAG` only for allowed tags, else fail with clear error.
- Instagram: `POST /v21.0/{ig-user-id}/messages`.
- WhatsApp Cloud: `POST /v21.0/{phone_number_id}/messages` (text inside 24h; template outside).
- Telegram: `sendMessage` with bot token from vault.
Each adapter implements `social.Sender` interface; a `FakeSender` is used in tests and in dev when credentials are empty.

## Reminder pipeline
`reminder.Planner` (worker, hourly): for each active rule × matching subscription/invoice/master account, upsert a `reminders` row at `scheduled_at` (tenant timezone, `send_hour`). `reminder.Dispatcher` (worker, every minute): `reminders` due → render template (`{{customer}} {{product}} {{expiry}} {{amount}} {{link}}` with bn date formatting) → `inbox.Send` via channel (preferred → identity lookup) → mark sent/failed.

## Payment auto-match
`payment.match` job: for a `payment_inbox` row with trx_id: find `payments` with same tenant, method, trx_id, status pending → verify (source `sms_match`) → invoice paid/partial → if invoice fully paid and linked subscription is pending → activate + enqueue `delivery.send` (product delivery_template, credentials from vault rendered server-side, never stored in message body as plaintext beyond what provider configured).

## Notes after implementation (Sprint 1)
- **SMS forwarder token** is per tenant, stored only in the vault. `rotateSmsWebhookToken` (OWNER/ADMIN) creates or replaces it, returns the full token exactly once and writes an `sms_token.rotate` audit row; the old token stops working immediately. `tenant.settings.sms_webhook_token_hint` shows a masked hint. The `tenant` query no longer creates tokens as a side effect (L3). Seed creates one for the demo tenant (printed in dev only).
- **Facebook Login** redirect URI is `PUBLIC_API_URL/webhooks/meta/oauth`; that route only forwards `code`+`state` to `PUBLIC_APP_URL/social` where the authenticated `completeMetaConnect` finishes the connection (M2).
- **WhatsApp**: `connectWhatsApp` checks the token (`GET /me`) and that `phoneNumberId` is listed by `GET /{businessAccountId}/phone_numbers` for that token (H5). The in-memory fake accepts numbers registered with `Fake.AddWABANumber` plus `<wabaId>-1..3` for manual dev testing.
- **One owner per channel**: `(platform, external_id)` is unique among non-disconnected `social_accounts` rows across all tenants; connecting a page/number/bot another business holds returns `CONFLICT` (H5). Webhook routing never has to choose between tenants.
- **Tokens never in URLs** (M3): Meta calls send `Authorization: Bearer <token>` plus `appsecret_proof`; OAuth code exchange posts `client_secret` in a form body. Transport errors are reduced to `social api <METHOD> <host>: timeout|dns lookup failed|connection failed` (`social.SafeError`) before they are returned, stored in `last_error`/`messages.error` or logged. Telegram's API requires the bot token in the path; its `*url.Error` is unwrapped the same way.
- **Credential delivery** (H3): `messages.body` and the conversation preview store the redacted text (`Password: ••••••`, `PIN: ••••••`); the real text travels only in the `inbox.send` job payload sealed with `vault.SealTransient` (AAD = tenant + message id) and is removed from the job row when the job finishes.
- Not yet: WhatsApp template messages outside the 24h window, `inbox.autoreply`, `POST /uploads`, Bengali glyphs in the price-list PNG (Latin font only), Telegram channel posting UI.
