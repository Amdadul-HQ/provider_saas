package billing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/pagination"
)

// Repository: every query filters by tenant_id; the service runs them in db.WithTenantTx.
type Repository struct{}

// ---- invoices

const invoiceCols = `i.id, i.number, i.customer_id, i.status, i.subtotal, i.discount, i.total, i.paid_amount, i.currency,
	i.issued_at, i.due_at, i.paid_at, i.notes, i.created_by, i.created_at`

func scanInvoice(r pgx.Row) (Invoice, error) {
	var i Invoice
	err := r.Scan(&i.ID, &i.Number, &i.CustomerID, &i.Status, &i.Subtotal, &i.Discount, &i.Total, &i.PaidAmount, &i.Currency,
		&i.IssuedAt, &i.DueAt, &i.PaidAt, &i.Notes, &i.CreatedBy, &i.CreatedAt)
	return i, err
}

func collectInvoices(rows pgx.Rows, err error) ([]Invoice, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Invoice, error) { return scanInvoice(r) })
}

func (Repository) NextInvoiceSeq(ctx context.Context, q db.Querier) (int64, error) {
	var n int64
	err := q.QueryRow(ctx, `SELECT nextval('invoice_number_seq')`).Scan(&n)
	return n, err
}

func (Repository) InsertInvoice(ctx context.Context, q db.Querier, tenantID uuid.UUID, number string, in CreateInvoiceInput, subtotal, total int64, status, currency string, createdBy *uuid.UUID) (Invoice, error) {
	inv, err := scanInvoice(q.QueryRow(ctx, `INSERT INTO invoices AS i (tenant_id, customer_id, number, status, subtotal, discount, total, currency, due_at, notes, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING `+invoiceCols,
		tenantID, in.CustomerID, number, status, subtotal, in.Discount, total, currency, in.DueAt, in.Notes, createdBy))
	return inv, db.MapErr(err, "invoice")
}

func (Repository) InsertItem(ctx context.Context, q db.Querier, tenantID, invoiceID uuid.UUID, it ItemInput) error {
	_, err := q.Exec(ctx, `INSERT INTO invoice_items (tenant_id, invoice_id, subscription_id, description, quantity, unit_price, total)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, tenantID, invoiceID, it.SubscriptionID, it.Description, it.Quantity, it.UnitPrice, int64(it.Quantity)*it.UnitPrice)
	return db.MapErr(err, "invoice item")
}

func (Repository) GetInvoice(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, lock bool) (Invoice, error) {
	sql := `SELECT ` + invoiceCols + ` FROM invoices i WHERE i.tenant_id = $1 AND i.id = $2`
	if lock {
		sql += ` FOR UPDATE`
	}
	inv, err := scanInvoice(q.QueryRow(ctx, sql, tenantID, id))
	return inv, db.MapErr(err, "invoice")
}

func (Repository) InvoicesByIDs(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Invoice, error) {
	return collectInvoices(q.Query(ctx, `SELECT `+invoiceCols+` FROM invoices i WHERE i.tenant_id = $1 AND i.id = ANY($2)`, tenantID, ids))
}

func invoiceWhere(f InvoiceFilter, args *[]any) string {
	add := func(v any) string { *args = append(*args, v); return fmt.Sprintf("$%d", len(*args)) }
	conds := []string{"i.tenant_id = $1"}
	if f.Search != nil && strings.TrimSpace(*f.Search) != "" {
		p := add("%" + strings.TrimSpace(*f.Search) + "%")
		conds = append(conds, fmt.Sprintf(`(i.number ILIKE %[1]s OR EXISTS (SELECT 1 FROM customers c WHERE c.tenant_id = i.tenant_id
			AND c.id = i.customer_id AND (c.name ILIKE %[1]s OR c.phone ILIKE %[1]s)))`, p))
	}
	if f.Status != nil {
		conds = append(conds, "i.status = "+add(*f.Status))
	}
	if f.CustomerID != nil {
		conds = append(conds, "i.customer_id = "+add(*f.CustomerID))
	}
	if f.OverdueOnly {
		conds = append(conds, "i.status NOT IN ('paid','void') AND i.due_at < now() AND i.total > i.paid_amount")
	}
	return strings.Join(conds, " AND ")
}

func (Repository) ListInvoices(ctx context.Context, q db.Querier, tenantID uuid.UUID, f InvoiceFilter, page pagination.Page) ([]Invoice, error) {
	args := []any{tenantID}
	w := invoiceWhere(f, &args)
	if page.After != nil {
		args = append(args, page.After.CreatedAt, page.After.ID)
		w += fmt.Sprintf(" AND (i.created_at, i.id) < ($%d, $%d)", len(args)-1, len(args))
	}
	args = append(args, page.First+1)
	return collectInvoices(q.Query(ctx, `SELECT `+invoiceCols+` FROM invoices i WHERE `+w+
		fmt.Sprintf(` ORDER BY i.created_at DESC, i.id DESC LIMIT $%d`, len(args)), args...))
}

func (Repository) CountInvoices(ctx context.Context, q db.Querier, tenantID uuid.UUID, f InvoiceFilter) (int, error) {
	args := []any{tenantID}
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM invoices i WHERE `+invoiceWhere(f, &args), args...).Scan(&n)
	return n, err
}

func (Repository) UpdateInvoicePayment(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, paid int64, status string, paidAt *time.Time) error {
	_, err := q.Exec(ctx, `UPDATE invoices SET paid_amount = $3, status = $4, paid_at = $5 WHERE tenant_id = $1 AND id = $2`, tenantID, id, paid, status, paidAt)
	return err
}

func (Repository) SetInvoiceStatus(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, status string, note *string) error {
	_, err := q.Exec(ctx, `UPDATE invoices SET status = $3, notes = CASE WHEN $4::text IS NULL THEN notes ELSE COALESCE(notes || E'\n', '') || $4 END
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, status, note)
	return err
}

// MarkOverdue flips unpaid invoices past their due date to overdue.
func (Repository) MarkOverdue(ctx context.Context, q db.Querier, tenantID uuid.UUID, now time.Time) (int64, error) {
	tag, err := q.Exec(ctx, `UPDATE invoices SET status = 'overdue' WHERE tenant_id = $1 AND status IN ('draft','sent','partial')
		AND due_at < $2 AND total > paid_amount`, tenantID, now)
	return tag.RowsAffected(), err
}

func (Repository) ItemsByInvoices(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]InvoiceItem, error) {
	rows, err := q.Query(ctx, `SELECT id, invoice_id, subscription_id, description, quantity, unit_price, total FROM invoice_items
		WHERE tenant_id = $1 AND invoice_id = ANY($2) ORDER BY created_at, id`, tenantID, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (InvoiceItem, error) {
		var it InvoiceItem
		err := r.Scan(&it.ID, &it.InvoiceID, &it.SubscriptionID, &it.Description, &it.Quantity, &it.UnitPrice, &it.Total)
		return it, err
	})
}

// ---- payments

const paymentCols = `p.id, p.customer_id, p.invoice_id, p.method, p.trx_id, p.sender_number, p.amount, p.currency, p.status,
	p.verified_by, p.verified_at, p.verification_source, p.notes, p.received_at, p.created_at`

func scanPayment(r pgx.Row) (Payment, error) {
	var p Payment
	err := r.Scan(&p.ID, &p.CustomerID, &p.InvoiceID, &p.Method, &p.TrxID, &p.SenderNumber, &p.Amount, &p.Currency, &p.Status,
		&p.VerifiedBy, &p.VerifiedAt, &p.VerificationSource, &p.Notes, &p.ReceivedAt, &p.CreatedAt)
	return p, err
}

func collectPayments(rows pgx.Rows, err error) ([]Payment, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Payment, error) { return scanPayment(r) })
}

func (Repository) InsertPayment(ctx context.Context, q db.Querier, tenantID uuid.UUID, p Payment) (Payment, error) {
	return scanPayment(q.QueryRow(ctx, `INSERT INTO payments AS p (tenant_id, customer_id, invoice_id, method, trx_id, sender_number, amount, currency,
		status, verified_by, verified_at, verification_source, notes, received_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING `+paymentCols,
		tenantID, p.CustomerID, p.InvoiceID, p.Method, p.TrxID, p.SenderNumber, p.Amount, p.Currency,
		p.Status, p.VerifiedBy, p.VerifiedAt, p.VerificationSource, p.Notes, p.ReceivedAt))
}

func (Repository) GetPayment(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, lock bool) (Payment, error) {
	sql := `SELECT ` + paymentCols + ` FROM payments p WHERE p.tenant_id = $1 AND p.id = $2`
	if lock {
		sql += ` FOR UPDATE`
	}
	p, err := scanPayment(q.QueryRow(ctx, sql, tenantID, id))
	return p, db.MapErr(err, "payment")
}

func (Repository) UpdatePaymentStatus(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, status string, verifiedBy *uuid.UUID, verifiedAt *time.Time, source *string, note *string) (Payment, error) {
	p, err := scanPayment(q.QueryRow(ctx, `UPDATE payments p SET status = $3, verified_by = $4, verified_at = $5,
		verification_source = COALESCE($6, p.verification_source),
		notes = CASE WHEN $7::text IS NULL THEN p.notes ELSE COALESCE(p.notes || E'\n', '') || $7 END
		WHERE p.tenant_id = $1 AND p.id = $2 RETURNING `+paymentCols, tenantID, id, status, verifiedBy, verifiedAt, source, note))
	return p, db.MapErr(err, "payment")
}

func (Repository) PaymentsByInvoices(ctx context.Context, q db.Querier, tenantID uuid.UUID, ids []uuid.UUID) ([]Payment, error) {
	return collectPayments(q.Query(ctx, `SELECT `+paymentCols+` FROM payments p WHERE p.tenant_id = $1 AND p.invoice_id = ANY($2) ORDER BY p.received_at`, tenantID, ids))
}

// PendingPaymentByTrx finds a pending payment for SMS auto-matching.
func (Repository) PendingPaymentByTrx(ctx context.Context, q db.Querier, tenantID uuid.UUID, method, trx string) (Payment, error) {
	return scanPayment(q.QueryRow(ctx, `SELECT `+paymentCols+` FROM payments p WHERE p.tenant_id = $1 AND p.method = $2 AND upper(p.trx_id) = $3
		AND p.status = 'pending' LIMIT 1 FOR UPDATE`, tenantID, method, trx))
}

func paymentWhere(f PaymentFilter, args *[]any) string {
	add := func(v any) string { *args = append(*args, v); return fmt.Sprintf("$%d", len(*args)) }
	conds := []string{"p.tenant_id = $1"}
	if f.Status != nil {
		conds = append(conds, "p.status = "+add(*f.Status))
	}
	if f.Method != nil {
		conds = append(conds, "p.method = "+add(*f.Method))
	}
	if f.CustomerID != nil {
		conds = append(conds, "p.customer_id = "+add(*f.CustomerID))
	}
	if f.Search != nil && strings.TrimSpace(*f.Search) != "" {
		p := add("%" + strings.TrimSpace(*f.Search) + "%")
		conds = append(conds, fmt.Sprintf("(p.trx_id ILIKE %[1]s OR p.sender_number ILIKE %[1]s)", p))
	}
	return strings.Join(conds, " AND ")
}

func (Repository) ListPayments(ctx context.Context, q db.Querier, tenantID uuid.UUID, f PaymentFilter, page pagination.Page) ([]Payment, error) {
	args := []any{tenantID}
	w := paymentWhere(f, &args)
	if page.After != nil {
		args = append(args, page.After.CreatedAt, page.After.ID)
		w += fmt.Sprintf(" AND (p.created_at, p.id) < ($%d, $%d)", len(args)-1, len(args))
	}
	args = append(args, page.First+1)
	return collectPayments(q.Query(ctx, `SELECT `+paymentCols+` FROM payments p WHERE `+w+
		fmt.Sprintf(` ORDER BY p.created_at DESC, p.id DESC LIMIT $%d`, len(args)), args...))
}

func (Repository) CountPayments(ctx context.Context, q db.Querier, tenantID uuid.UUID, f PaymentFilter) (int, error) {
	args := []any{tenantID}
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM payments p WHERE `+paymentWhere(f, &args), args...).Scan(&n)
	return n, err
}

// ---- payment inbox

const inboxCols = `id, source, method, sender_number, trx_id, amount, raw_text, status, matched_payment_id, received_at`

func scanInbox(r pgx.Row) (InboxItem, error) {
	var i InboxItem
	err := r.Scan(&i.ID, &i.Source, &i.Method, &i.SenderNumber, &i.TrxID, &i.Amount, &i.RawText, &i.Status, &i.MatchedPaymentID, &i.ReceivedAt)
	return i, err
}

func (Repository) InsertInbox(ctx context.Context, q db.Querier, tenantID uuid.UUID, i InboxItem) (InboxItem, error) {
	return scanInbox(q.QueryRow(ctx, `INSERT INTO payment_inbox (tenant_id, source, method, sender_number, trx_id, amount, raw_text, status, received_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+inboxCols,
		tenantID, i.Source, i.Method, i.SenderNumber, i.TrxID, i.Amount, i.RawText, i.Status, i.ReceivedAt))
}

func (Repository) GetInbox(ctx context.Context, q db.Querier, tenantID, id uuid.UUID) (InboxItem, error) {
	i, err := scanInbox(q.QueryRow(ctx, `SELECT `+inboxCols+` FROM payment_inbox WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, id))
	return i, db.MapErr(err, "payment inbox item")
}

// UnmatchedInbox finds an unmatched SMS with the same TrxID and amount.
func (Repository) UnmatchedInbox(ctx context.Context, q db.Querier, tenantID uuid.UUID, trx string, amount int64) (InboxItem, error) {
	return scanInbox(q.QueryRow(ctx, `SELECT `+inboxCols+` FROM payment_inbox WHERE tenant_id = $1 AND upper(trx_id) = $2 AND amount = $3
		AND status = 'unmatched' ORDER BY received_at LIMIT 1 FOR UPDATE`, tenantID, trx, amount))
}

func (Repository) SetInboxStatus(ctx context.Context, q db.Querier, tenantID, id uuid.UUID, status string, paymentID *uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE payment_inbox SET status = $3, matched_payment_id = $4 WHERE tenant_id = $1 AND id = $2`, tenantID, id, status, paymentID)
	return err
}

func (Repository) ListInbox(ctx context.Context, q db.Querier, tenantID uuid.UUID, status *string, page pagination.Page) ([]InboxItem, error) {
	args := []any{tenantID, status}
	w := "tenant_id = $1 AND ($2::text IS NULL OR status = $2)"
	if page.After != nil {
		args = append(args, page.After.CreatedAt, page.After.ID)
		w += " AND (created_at, id) < ($3, $4)"
	}
	args = append(args, page.First)
	rows, err := q.Query(ctx, `SELECT `+inboxCols+` FROM payment_inbox WHERE `+w+fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d`, len(args)), args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (InboxItem, error) { return scanInbox(r) })
}

// ---- ledger

const ledgerCols = `id, customer_id, entry_type, amount, currency, ref_type, ref_id, note, created_by, created_at`

func scanLedger(r pgx.Row) (LedgerEntry, error) {
	var l LedgerEntry
	err := r.Scan(&l.ID, &l.CustomerID, &l.EntryType, &l.Amount, &l.Currency, &l.RefType, &l.RefID, &l.Note, &l.CreatedBy, &l.CreatedAt)
	return l, err
}

func (Repository) InsertLedger(ctx context.Context, q db.Querier, tenantID uuid.UUID, l LedgerEntry) (LedgerEntry, error) {
	e, err := scanLedger(q.QueryRow(ctx, `INSERT INTO ledger_entries (tenant_id, customer_id, entry_type, amount, currency, ref_type, ref_id, note, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+ledgerCols,
		tenantID, l.CustomerID, l.EntryType, l.Amount, l.Currency, l.RefType, l.RefID, l.Note, l.CreatedBy))
	return e, db.MapErr(err, "ledger entry")
}

func (Repository) Ledger(ctx context.Context, q db.Querier, tenantID, customerID uuid.UUID) ([]LedgerEntry, error) {
	rows, err := q.Query(ctx, `SELECT `+ledgerCols+` FROM ledger_entries WHERE tenant_id = $1 AND customer_id = $2 ORDER BY created_at DESC, id`, tenantID, customerID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (LedgerEntry, error) { return scanLedger(r) })
}

func (Repository) TenantCurrency(ctx context.Context, q db.Querier, tenantID uuid.UUID) (string, error) {
	var c string
	err := q.QueryRow(ctx, `SELECT currency FROM tenants WHERE id = $1`, tenantID).Scan(&c)
	return c, err
}
