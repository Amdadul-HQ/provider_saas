package inbox

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/platform/db"
)

const ruleCols = `id, name, keywords, match_type, body, template_id, platforms, only_outside_hours, cooldown_minutes,
	priority, is_active, hits, created_at`

func scanRule(r pgx.Row) (AutoReplyRule, error) {
	var x AutoReplyRule
	err := r.Scan(&x.ID, &x.Name, &x.Keywords, &x.MatchType, &x.Body, &x.TemplateID, &x.Platforms, &x.OnlyOutsideHours,
		&x.CooldownMinutes, &x.Priority, &x.IsActive, &x.Hits, &x.CreatedAt)
	return x, err
}

func (Repository) AutoReplyRules(ctx context.Context, q db.Querier, tenantID uuid.UUID, activeOnly bool) ([]AutoReplyRule, error) {
	rows, err := q.Query(ctx, `SELECT `+ruleCols+` FROM auto_reply_rules WHERE tenant_id = $1 AND (NOT $2 OR is_active)
		ORDER BY priority DESC, created_at, id`, tenantID, activeOnly)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (AutoReplyRule, error) { return scanRule(r) })
}

func (Repository) HasActiveAutoReplyRules(ctx context.Context, q db.Querier, tenantID uuid.UUID) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM auto_reply_rules WHERE tenant_id = $1 AND is_active)`, tenantID).Scan(&ok)
	return ok, err
}

func (Repository) InsertAutoReplyRule(ctx context.Context, q db.Querier, tenantID uuid.UUID, in AutoReplyInput) (AutoReplyRule, error) {
	r, err := scanRule(q.QueryRow(ctx, `INSERT INTO auto_reply_rules (tenant_id, name, keywords, match_type, body, template_id, platforms,
		only_outside_hours, cooldown_minutes, priority, is_active) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING `+ruleCols,
		tenantID, in.Name, in.Keywords, in.MatchType, in.Body, in.TemplateID, in.Platforms, in.OnlyOutsideHours, in.CooldownMinutes,
		in.Priority, in.IsActive))
	return r, db.MapErr(err, "auto-reply rule")
}

func (Repository) UpdateAutoReplyRule(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, in AutoReplyInput) (AutoReplyRule, error) {
	r, err := scanRule(q.QueryRow(ctx, `UPDATE auto_reply_rules SET name = $3, keywords = $4, match_type = $5, body = $6, template_id = $7,
		platforms = $8, only_outside_hours = $9, cooldown_minutes = $10, priority = $11, is_active = $12
		WHERE tenant_id = $1 AND id = $2 RETURNING `+ruleCols,
		tenantID, id, in.Name, in.Keywords, in.MatchType, in.Body, in.TemplateID, in.Platforms, in.OnlyOutsideHours, in.CooldownMinutes,
		in.Priority, in.IsActive))
	return r, db.MapErr(err, "auto-reply rule")
}

func (Repository) DeleteAutoReplyRule(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (bool, error) {
	tag, err := q.Exec(ctx, `DELETE FROM auto_reply_rules WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return tag.RowsAffected() > 0, err
}

// MarkAutoReplied records the reply: conversation cooldown clock, rule hit counter, and
// which rule produced the message.
func (Repository) MarkAutoReplied(ctx context.Context, q db.Querier, tenantID, convID, messageID, ruleID uuid.UUID) error {
	if _, err := q.Exec(ctx, `UPDATE conversations SET last_auto_reply_at = now() WHERE tenant_id = $1 AND id = $2`, tenantID, convID); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `UPDATE auto_reply_rules SET hits = hits + 1 WHERE tenant_id = $1 AND id = $2`, tenantID, ruleID); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `UPDATE messages SET auto_reply_rule_id = $3 WHERE tenant_id = $1 AND id = $2`, tenantID, messageID, ruleID)
	return err
}
