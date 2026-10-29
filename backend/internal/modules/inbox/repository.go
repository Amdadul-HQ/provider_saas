package inbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/modules/social"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/pagination"
	"provider_saas/internal/platform/sqlb"
)

type Repository struct{}

const convCols = `c.id, c.tenant_id, c.social_account_id, c.customer_id, c.identity_id, c.platform, c.external_thread_id, c.status,
	c.assigned_to, c.unread_count, c.last_message_at, c.last_message_preview, c.last_inbound_at, c.labels, c.created_at, c.last_auto_reply_at`

func scanConv(r pgx.Row) (Conversation, error) {
	var c Conversation
	err := r.Scan(&c.ID, &c.TenantID, &c.SocialAccountID, &c.CustomerID, &c.IdentityID, &c.Platform, &c.ExternalThreadID, &c.Status,
		&c.AssignedTo, &c.UnreadCount, &c.LastMessageAt, &c.LastMessagePreview, &c.LastInboundAt, &c.Labels, &c.CreatedAt, &c.LastAutoReplyAt)
	return c, err
}

func collectConvs(rows pgx.Rows, err error) ([]Conversation, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Conversation, error) { return scanConv(r) })
}

const msgCols = `m.id, m.conversation_id, m.direction, m.external_id, m.kind, m.body, m.attachments, m.sent_by_user_id, m.status, m.error, m.sent_at, m.created_at`

func scanMsg(r pgx.Row) (Message, error) {
	var m Message
	var att []byte
	err := r.Scan(&m.ID, &m.ConversationID, &m.Direction, &m.ExternalID, &m.Kind, &m.Body, &att, &m.SentBy, &m.Status, &m.Error, &m.SentAt, &m.CreatedAt)
	if err == nil && len(att) > 0 {
		_ = json.Unmarshal(att, &m.Attachments)
	}
	return m, err
}

func collectMsgs(rows pgx.Rows, err error) ([]Message, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Message, error) { return scanMsg(r) })
}

// UpsertConversation returns the conversation for (account, thread), creating it.
// Existing links to a customer/identity are kept.
func (Repository) UpsertConversation(ctx context.Context, q db.Querier, tenantID, accountID uuid.UUID, platform, thread string, customerID, identityID *uuid.UUID) (Conversation, error) {
	c, err := scanConv(q.QueryRow(ctx, `INSERT INTO conversations AS c (tenant_id, social_account_id, platform, external_thread_id, customer_id, identity_id)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (social_account_id, external_thread_id) DO UPDATE SET
		  customer_id = COALESCE(c.customer_id, EXCLUDED.customer_id), identity_id = COALESCE(c.identity_id, EXCLUDED.identity_id)
		RETURNING `+convCols, tenantID, accountID, platform, thread, customerID, identityID))
	return c, db.MapErr(err, "conversation")
}

func (Repository) GetConversation(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, lock bool) (Conversation, error) {
	sql := `SELECT ` + convCols + ` FROM conversations c WHERE c.tenant_id = $1 AND c.id = $2`
	if lock {
		sql += ` FOR UPDATE`
	}
	c, err := scanConv(q.QueryRow(ctx, sql, tenantID, id))
	return c, db.MapErr(err, "conversation")
}

// LatestForCustomer returns the customer's most recently active conversation, optionally on one platform.
func (Repository) LatestForCustomer(ctx context.Context, q db.Querier, tenantID, customerID uuid.UUID, platform string) (Conversation, error) {
	args := []any{tenantID, customerID}
	w := ""
	if platform != "" {
		args = append(args, platform)
		w = " AND c.platform = $3"
	}
	c, err := scanConv(q.QueryRow(ctx, `SELECT `+convCols+` FROM conversations c
		JOIN social_accounts a ON a.id = c.social_account_id AND a.status <> 'disconnected'
		WHERE c.tenant_id = $1 AND c.customer_id = $2 AND c.status <> 'spam'`+w+`
		ORDER BY c.last_inbound_at DESC NULLS LAST, c.last_message_at DESC NULLS LAST LIMIT 1`, args...))
	return c, err
}

var convKeyset = sqlb.Keyset{Sort: "COALESCE(c.last_message_at, c.created_at)", ID: "c.id", From: "conversations c"}

// filter is shared by List and Count so the two can never disagree.
func filter(tenantID uuid.UUID, f Filter) *sqlb.Query {
	q := sqlb.New("c.tenant_id", tenantID)
	if f.Status != nil {
		q.Where("c.status = ?", *f.Status)
	} else {
		q.Where("c.status <> 'spam'")
	}
	sqlb.Eq(q, "c.platform", f.Platform)
	sqlb.Eq(q, "c.social_account_id", f.SocialAccountID)
	sqlb.Eq(q, "c.customer_id", f.CustomerID)
	sqlb.Eq(q, "c.assigned_to", f.AssignedTo)
	q.WhereIf(f.Unassigned != nil && *f.Unassigned, "c.assigned_to IS NULL")
	if f.Label != nil && strings.TrimSpace(*f.Label) != "" {
		q.Where("? = ANY(c.labels)", strings.ToLower(strings.TrimSpace(*f.Label)))
	}
	q.Search(f.Search,
		sqlb.Like("c.last_message_preview", "c.external_thread_id"),
		sqlb.Related("customers cu", "cu.tenant_id = c.tenant_id AND cu.id = c.customer_id", sqlb.Like("cu.name", "cu.phone")),
		sqlb.Related("customer_identities ci", "ci.tenant_id = c.tenant_id AND ci.id = c.identity_id", sqlb.Like("ci.handle")))
	return q
}

func (Repository) List(ctx context.Context, q db.Querier, tenantID uuid.UUID, f Filter, page pagination.Page) ([]Conversation, error) {
	w := filter(tenantID, f)
	return collectConvs(q.Query(ctx, `SELECT `+convCols+` FROM conversations c`+w.Page(page, convKeyset), w.Args()...))
}

func (Repository) Count(ctx context.Context, q db.Querier, tenantID uuid.UUID, f Filter) (int, error) {
	w := filter(tenantID, f)
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM conversations c`+w.WhereSQL(), w.Args()...).Scan(&n)
	return n, err
}

func (Repository) Counts(ctx context.Context, q db.Querier, tenantID, userID uuid.UUID) (Counts, error) {
	var c Counts
	err := q.QueryRow(ctx, `SELECT
		count(*) FILTER (WHERE status = 'open'),
		count(*) FILTER (WHERE status IN ('open','pending') AND assigned_to IS NULL),
		count(*) FILTER (WHERE status IN ('open','pending') AND assigned_to = $2),
		count(*) FILTER (WHERE status <> 'spam' AND unread_count > 0)
		FROM conversations WHERE tenant_id = $1`, tenantID, userID).Scan(&c.Open, &c.Unassigned, &c.Mine, &c.Unread)
	return c, err
}

func (Repository) OpenCountsByAccount(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := q.Query(ctx, `SELECT social_account_id, count(*) FROM conversations WHERE tenant_id = $1
		AND social_account_id = ANY($2) AND status IN ('open','pending') GROUP BY social_account_id`, tenantID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// InsertMessage stores a message. ok=false when an inbound message with the same
// external id already exists in the conversation (webhook redelivery).
func (Repository) InsertMessage(ctx context.Context, q db.Querier, tenantID uuid.UUID, m Message) (Message, bool, error) {
	att := m.Attachments
	if att == nil {
		att = []social.Attachment{}
	}
	attJSON, _ := json.Marshal(att)
	out, err := scanMsg(q.QueryRow(ctx, `INSERT INTO messages AS m (tenant_id, conversation_id, direction, external_id, kind, body, attachments,
		sent_by_user_id, status, sent_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (conversation_id, external_id) WHERE external_id IS NOT NULL DO NOTHING
		RETURNING `+msgCols, tenantID, m.ConversationID, m.Direction, m.ExternalID, m.Kind, m.Body, attJSON, m.SentBy, m.Status, m.SentAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return out, false, nil
	}
	return out, err == nil, db.MapErr(err, "message")
}

func (Repository) GetMessage(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, lock bool) (Message, error) {
	sql := `SELECT ` + msgCols + ` FROM messages m WHERE m.tenant_id = $1 AND m.id = $2`
	if lock {
		sql += ` FOR UPDATE`
	}
	m, err := scanMsg(q.QueryRow(ctx, sql, tenantID, id))
	return m, db.MapErr(err, "message")
}

func (Repository) SetMessageResult(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, status string, externalID, errMsg *string) (Message, error) {
	m, err := scanMsg(q.QueryRow(ctx, `UPDATE messages m SET status = $3, external_id = COALESCE($4, m.external_id), error = $5
		WHERE m.tenant_id = $1 AND m.id = $2 RETURNING `+msgCols, tenantID, id, status, externalID, errMsg))
	return m, db.MapErr(err, "message")
}

// Messages returns the newest page of a conversation (before the message `before`, if given),
// in ascending time order.
var msgKeyset = sqlb.Keyset{Sort: "m.sent_at", ID: "m.id", From: "messages m"}

// Messages returns up to page.First+1 messages of a conversation, newest first; the service
// trims the page and flips it to chat order.
func (Repository) Messages(ctx context.Context, q db.Querier, tenantID, convID uuid.UUID, page pagination.Page) ([]Message, error) {
	w := sqlb.New("m.tenant_id", tenantID).Where("m.conversation_id = ?", convID)
	return collectMsgs(q.Query(ctx, `SELECT `+msgCols+` FROM messages m`+w.Page(page, msgKeyset), w.Args()...))
}

// BumpInbound updates counters after an inbound message; a closed conversation reopens.
func (Repository) BumpInbound(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, at time.Time, preview string) error {
	_, err := q.Exec(ctx, `UPDATE conversations SET unread_count = unread_count + 1,
		last_message_preview = CASE WHEN last_message_at IS NULL OR $3 >= last_message_at THEN $4 ELSE last_message_preview END,
		last_message_at = GREATEST(last_message_at, $3), last_inbound_at = GREATEST(last_inbound_at, $3),
		status = CASE WHEN status = 'closed' THEN 'open' ELSE status END
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, at, preview)
	return err
}

// BumpOutbound records an outbound message (replying marks the thread read).
func (Repository) BumpOutbound(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, at time.Time, preview string, markRead bool) error {
	_, err := q.Exec(ctx, `UPDATE conversations SET last_message_at = GREATEST(last_message_at, $3),
		last_message_preview = $4, unread_count = CASE WHEN $5 THEN 0 ELSE unread_count END
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, at, preview, markRead)
	return err
}

// Update applies one mutable field change and returns the row.
func (Repository) Update(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, set string, args ...any) (Conversation, error) {
	all := append([]any{tenantID, id}, args...)
	c, err := scanConv(q.QueryRow(ctx, `UPDATE conversations c SET `+set+` WHERE c.tenant_id = $1 AND c.id = $2 RETURNING `+convCols, all...))
	return c, db.MapErr(err, "conversation")
}

// ---- templates

const tplCols = `id, key, name, body, category, is_active`

func scanTpl(r pgx.Row) (Template, error) {
	var t Template
	err := r.Scan(&t.ID, &t.Key, &t.Name, &t.Body, &t.Category, &t.IsActive)
	return t, err
}

func (Repository) Templates(ctx context.Context, q db.Querier, tenantID uuid.UUID, category *string) ([]Template, error) {
	rows, err := q.Query(ctx, `SELECT `+tplCols+` FROM message_templates WHERE tenant_id = $1 AND ($2::text IS NULL OR category = $2)
		ORDER BY category, name`, tenantID, category)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Template, error) { return scanTpl(r) })
}

func (Repository) TemplatesByIDs(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Template, error) {
	rows, err := q.Query(ctx, `SELECT `+tplCols+` FROM message_templates WHERE tenant_id = $1 AND id = ANY($2)`, tenantID, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Template, error) { return scanTpl(r) })
}

func (Repository) Template(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (Template, error) {
	t, err := scanTpl(q.QueryRow(ctx, `SELECT `+tplCols+` FROM message_templates WHERE tenant_id = $1 AND id = $2`, tenantID, id))
	return t, db.MapErr(err, "template")
}

func (Repository) TemplateByKey(ctx context.Context, q db.Querier, tenantID uuid.UUID, key string) (Template, error) {
	t, err := scanTpl(q.QueryRow(ctx, `SELECT `+tplCols+` FROM message_templates WHERE tenant_id = $1 AND key = $2`, tenantID, key))
	return t, db.MapErr(err, "template")
}

func (Repository) InsertTemplate(ctx context.Context, q db.Querier, tenantID uuid.UUID, in TemplateInput) (Template, error) {
	t, err := scanTpl(q.QueryRow(ctx, `INSERT INTO message_templates (tenant_id, key, name, body, category, is_active)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+tplCols, tenantID, in.Key, in.Name, in.Body, in.Category, in.IsActive))
	return t, db.MapErr(err, "template")
}

func (Repository) UpdateTemplate(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, in TemplateInput) (Template, error) {
	t, err := scanTpl(q.QueryRow(ctx, `UPDATE message_templates SET key = $3, name = $4, body = $5, category = $6, is_active = $7
		WHERE tenant_id = $1 AND id = $2 RETURNING `+tplCols, tenantID, id, in.Key, in.Name, in.Body, in.Category, in.IsActive))
	return t, db.MapErr(err, "template")
}

func (Repository) DeleteTemplate(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (bool, error) {
	tag, err := q.Exec(ctx, `DELETE FROM message_templates WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return tag.RowsAffected() > 0, err
}
