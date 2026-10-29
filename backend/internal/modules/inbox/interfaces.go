package inbox

import (
	"context"

	"github.com/google/uuid"
)

// Sender delivers a text message to a customer through a connected channel. It is
// what billing (invoices) and reminder need from inbox; billing.Notifier has the same
// method set, so an inbox Service satisfies it structurally. Implementations should
// enqueue a job (jobs.Queue.Enqueue joins the caller's transaction) rather than call
// Meta/Telegram APIs inside the caller's DB transaction.
//
// channel: "" = customer's preferred channel, else facebook|instagram|whatsapp|telegram|sms|email.
type Sender interface {
	SendToCustomer(ctx context.Context, tenantID, customerID uuid.UUID, channel, body string) error
}

// Service is the inbox module API expected by graph resolvers (to be implemented).
// Resolvers live in graph/inbox.resolvers.go and currently return "not implemented".
type Service interface {
	Sender
}
