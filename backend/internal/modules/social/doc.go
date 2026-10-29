// Package social connects Facebook pages, Instagram, WhatsApp Business and Telegram bots.
// Tokens are stored only in vault_secrets (platform/vault); webhooks are REST (ADR-003)
// and live in webhooks.go. All Meta/Telegram HTTP goes through Client (Fake in dev/tests).
package social
