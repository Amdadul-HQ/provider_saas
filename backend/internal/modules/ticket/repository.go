package ticket

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/pagination"
	"provider_saas/internal/platform/sqlb"
)

type Repository struct{}

const cols = `t.id, t.number, t.customer_id, t.subscription_id, t.conversation_id, t.kind, t.priority, t.status, t.title, t.description,
	t.reason, t.old_slot_id, t.new_slot_id, t.assigned_to, t.resolved_at, t.created_by, t.created_at, t.updated_at`

func scan(r pgx.Row) (Ticket, error) {
	var t Ticket
	err := r.Scan(&t.ID, &t.Number, &t.CustomerID, &t.SubscriptionID, &t.ConversationID, &t.Kind, &t.Priority, &t.Status, &t.Title,
		&t.Description, &t.Reason, &t.OldSlotID, &t.NewSlotID, &t.AssignedTo, &t.ResolvedAt, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

// Insert numbers the ticket per tenant (#1, #2, ... via db.NextNumber; QA BUG-11).
func (Repository) Insert(ctx context.Context, q db.Querier, tenantID uuid.UUID, in CreateInput, oldSlot, by *uuid.UUID) (Ticket, error) {
	n, err := db.NextNumber(ctx, q, tenantID, "ticket")
	if err != nil {
		return Ticket{}, err
	}
	t, err := scan(q.QueryRow(ctx, `INSERT INTO tickets AS t (tenant_id, number, customer_id, subscription_id, conversation_id, kind, priority,
		title, description, reason, old_slot_id, created_by)
		VALUES ($1, $12, $2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING `+cols,
		tenantID, in.CustomerID, in.SubscriptionID, in.ConversationID, in.Kind, in.Priority, in.Title, in.Description, in.Reason, oldSlot, by, n))
	return t, db.MapErr(err, "ticket")
}

func (Repository) Get(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, lock bool) (Ticket, error) {
	sql := `SELECT ` + cols + ` FROM tickets t WHERE t.tenant_id = $1 AND t.id = $2`
	if lock {
		sql += ` FOR UPDATE`
	}
	t, err := scan(q.QueryRow(ctx, sql, tenantID, id))
	return t, db.MapErr(err, "ticket")
}

// Save persists every mutable field.
func (Repository) Save(ctx context.Context, q db.Querier, tenantID uuid.UUID, t Ticket) (Ticket, error) {
	out, err := scan(q.QueryRow(ctx, `UPDATE tickets t SET priority = $3, status = $4, title = $5, description = $6, reason = $7,
		old_slot_id = $8, new_slot_id = $9, assigned_to = $10, resolved_at = $11
		WHERE t.tenant_id = $1 AND t.id = $2 RETURNING `+cols, tenantID, t.ID, t.Priority, t.Status, t.Title, t.Description, t.Reason,
		t.OldSlotID, t.NewSlotID, t.AssignedTo, t.ResolvedAt))
	return out, db.MapErr(err, "ticket")
}

var keyset = sqlb.Keyset{Sort: "t.created_at", ID: "t.id", From: "tickets t"}

func (Repository) List(ctx context.Context, q db.Querier, tenantID uuid.UUID, f Filter, page pagination.Page) ([]Ticket, error) {
	w := sqlb.New("t.tenant_id", tenantID)
	sqlb.Eq(w, "t.status", f.Status)
	sqlb.Eq(w, "t.kind", f.Kind)
	sqlb.Eq(w, "t.priority", f.Priority)
	sqlb.Eq(w, "t.customer_id", f.CustomerID)
	sqlb.Eq(w, "t.assigned_to", f.AssignedTo)
	w.Search(sqlb.NumberTerm(f.Search),
		sqlb.Like("t.title", "t.description"),
		sqlb.Exact("t.number::text"),
		sqlb.Related("customers c", "c.tenant_id = t.tenant_id AND c.id = t.customer_id", sqlb.Like("c.name", "c.phone")))
	rows, err := q.Query(ctx, `SELECT `+cols+` FROM tickets t`+w.Page(page, keyset), w.Args()...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Ticket, error) { return scan(r) })
}

func (Repository) Stats(ctx context.Context, q db.Querier, tenantID uuid.UUID, monthStart time.Time) (Stats, error) {
	var s Stats
	err := q.QueryRow(ctx, `SELECT
		count(*) FILTER (WHERE status IN ('open','waiting_customer')),
		count(*) FILTER (WHERE status = 'in_progress'),
		count(*) FILTER (WHERE status IN ('resolved','closed') AND resolved_at >= $2),
		count(*) FILTER (WHERE kind = 'replacement' AND created_at >= $2)
		FROM tickets WHERE tenant_id = $1`, tenantID, monthStart).Scan(&s.Open, &s.InProgress, &s.ResolvedThisMonth, &s.ReplacementsThisMonth)
	return s, err
}

func (Repository) InsertComment(ctx context.Context, q db.Querier, tenantID, ticketID uuid.UUID, author *uuid.UUID, body string, internal bool) (Comment, error) {
	var c Comment
	err := q.QueryRow(ctx, `INSERT INTO ticket_comments (tenant_id, ticket_id, author_id, body, is_internal) VALUES ($1,$2,$3,$4,$5)
		RETURNING id, ticket_id, author_id, body, is_internal, created_at`, tenantID, ticketID, author, body, internal).
		Scan(&c.ID, &c.TicketID, &c.AuthorID, &c.Body, &c.IsInternal, &c.CreatedAt)
	return c, db.MapErr(err, "comment")
}

func (Repository) CommentsByTickets(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Comment, error) {
	rows, err := q.Query(ctx, `SELECT id, ticket_id, author_id, body, is_internal, created_at FROM ticket_comments
		WHERE tenant_id = $1 AND ticket_id = ANY($2) ORDER BY created_at, id`, tenantID, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Comment, error) {
		var c Comment
		err := r.Scan(&c.ID, &c.TicketID, &c.AuthorID, &c.Body, &c.IsInternal, &c.CreatedAt)
		return c, err
	})
}
