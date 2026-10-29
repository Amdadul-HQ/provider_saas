package delivery

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// JobSend is the job kind; one pending job per subscription (dedupe key).
const JobSend = "delivery.send"

// Payload of a delivery.send job.
type Payload struct {
	TenantID       uuid.UUID `json:"tenant_id"`
	SubscriptionID uuid.UUID `json:"subscription_id"`
	Reason         string    `json:"reason,omitempty"`
}

// Queue is jobs.Queue.
type Queue interface {
	Enqueue(ctx context.Context, kind string, payload any, runAt time.Time, dedupeKey string) (int64, error)
}

// Enqueue schedules delivery of a subscription (joins the caller's transaction).
func Enqueue(ctx context.Context, q Queue, tenantID, subscriptionID uuid.UUID, reason string) error {
	_, err := q.Enqueue(ctx, JobSend, Payload{TenantID: tenantID, SubscriptionID: subscriptionID, Reason: reason},
		time.Now(), JobSend+":"+subscriptionID.String())
	return err
}
