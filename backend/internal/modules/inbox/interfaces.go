package inbox

import (
	"context"

	"github.com/google/uuid"
)

// Sender delivers a text message to a customer through a connected channel. It is
// what billing (invoices) and reminder need from inbox; billing.Notifier has the same
// method set, so an inbox Service satisfies it structurally. The message is queued as
// an inbox.send job (joining the caller's transaction) rather than calling Meta/Telegram
// inside the caller's DB transaction.
//
// channel: "" = customer's preferred channel, else facebook|instagram|whatsapp|telegram.
type Sender interface {
	SendToCustomer(ctx context.Context, tenantID, customerID uuid.UUID, channel, body string) error
}
