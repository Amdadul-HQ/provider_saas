// Package inbox is the unified social inbox: conversations, messages, templates.
// Ingest is the single entry point for inbound webhooks (docs/integrations.md); outbound
// messages are queued as inbox.send jobs and delivered through social.Sender.
// It implements Sender (interfaces.go) used by billing/reminder/storefront to message customers.
package inbox
