# Integrations & webhooks (REST, outside GraphQL — ADR-003)

All webhook routes live under `backend/internal/platform/httpx` → `internal/modules/social/webhooks.go` and `billing/webhooks.go`.

| Route | Method | Who calls | Verification | Effect |
|---|---|---|---|---|
| `/webhooks/meta` | GET | Meta (subscribe) | `hub.verify_token == META_VERIFY_TOKEN` → echo `hub.challenge` | — |
| `/webhooks/meta` | POST | Meta (Messenger, Instagram, WhatsApp Cloud API) | HMAC-SHA256 of raw body with `META_APP_SECRET` vs `X-Hub-Signature-256` | upsert `customer_identities` + `conversations` + `messages`; auto-create customer; enqueue `inbox.autoreply` if rule matches |
| `/webhooks/meta/oauth` | GET | Browser redirect after Facebook Login | `state` matches signed state issued by `startMetaConnect` | exchange code → long-lived user token → list pages → store page tokens in vault → `social_accounts` rows → subscribe page to webhooks |
| `/webhooks/telegram/{accountId}` | POST | Telegram Bot API | `X-Telegram-Bot-Api-Secret-Token` equals per-account secret | same ingest as Meta |
| `/webhooks/payments/sms` | POST | SMS-forwarder app on provider's phone | `Authorization: Bearer <tenant sms token>` (stored in vault, shown once in settings) | insert `payment_inbox`, parse bKash/Nagad/Rocket SMS, enqueue `payment.match` |
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
