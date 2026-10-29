package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/modules/customer"
	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/money"
	"provider_saas/internal/platform/pagination"
)

// Customers is what billing needs from the customer module.
type Customers interface {
	Get(ctx context.Context, tenantID, id uuid.UUID) (customer.Customer, error)
	AdjustBalance(ctx context.Context, tenantID, customerID uuid.UUID, dueDelta, ltvDelta int64) error
}

// Notifier sends a message to a customer on a channel ("" = preferred). It is implemented
// by the inbox module (inbox.Sender) and is optional: nil means invoices are only marked sent.
type Notifier interface {
	SendToCustomer(ctx context.Context, tenantID, customerID uuid.UUID, channel, body string) error
}

// Auditor writes audit_logs rows (tenant module); joins the caller's transaction.
type Auditor interface {
	Audit(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, action, entityType string, entityID *uuid.UUID, meta map[string]any) error
}

// PaidHook is called inside the payment transaction when an invoice becomes fully
// paid (app wires it to enqueue delivery.send for undelivered subscriptions).
type PaidHook interface {
	InvoicePaid(ctx context.Context, tenantID, invoiceID uuid.UUID) error
}

// PaidHookFunc adapts a function to PaidHook.
type PaidHookFunc func(ctx context.Context, tenantID, invoiceID uuid.UUID) error

func (f PaidHookFunc) InvoicePaid(ctx context.Context, tenantID, invoiceID uuid.UUID) error {
	return f(ctx, tenantID, invoiceID)
}

// Service is the billing module API.
type Service interface {
	CreateInvoice(ctx context.Context, tenantID, actorID uuid.UUID, in CreateInvoiceInput) (Invoice, error)
	SendInvoice(ctx context.Context, tenantID, id uuid.UUID, channel string) (Invoice, error)
	VoidInvoice(ctx context.Context, tenantID, actorID, id uuid.UUID, reason *string) (Invoice, error)
	Invoice(ctx context.Context, tenantID, id uuid.UUID) (Invoice, error)
	Invoices(ctx context.Context, tenantID uuid.UUID, f InvoiceFilter, page pagination.Page) ([]Invoice, bool, int, error)
	InvoicesByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Invoice, error)
	ItemsByInvoices(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]InvoiceItem, error)
	PaymentsByInvoices(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]Payment, error)

	RecordPayment(ctx context.Context, tenantID, actorID uuid.UUID, invoiceID, customerID *uuid.UUID, in PaymentInput, opts RecordOptions) (Payment, error)
	VerifyPayment(ctx context.Context, tenantID, actorID, id uuid.UUID) (Payment, error)
	RejectPayment(ctx context.Context, tenantID, actorID, id uuid.UUID, reason *string) (Payment, error)
	Payment(ctx context.Context, tenantID, id uuid.UUID) (Payment, error)
	Payments(ctx context.Context, tenantID uuid.UUID, f PaymentFilter, page pagination.Page) ([]Payment, bool, int, error)

	// IngestSMS stores a forwarded SMS; only SMS from an allowed sender id may auto-verify.
	IngestSMS(ctx context.Context, tenantID uuid.UUID, sender, raw string, receivedAt time.Time) (InboxItem, error)
	PaymentInbox(ctx context.Context, tenantID uuid.UUID, status *string, page pagination.Page) ([]InboxItem, error)
	MatchInbox(ctx context.Context, tenantID, actorID, inboxID, paymentID uuid.UUID) (Payment, error)
	IgnoreInbox(ctx context.Context, tenantID, inboxID uuid.UUID) error

	Ledger(ctx context.Context, tenantID, customerID uuid.UUID) ([]LedgerEntry, error)
	AddLedgerAdjustment(ctx context.Context, tenantID, actorID, customerID uuid.UUID, entryType string, amount int64, note *string) (LedgerEntry, error)

	MarkOverdue(ctx context.Context, tenantID uuid.UUID, now time.Time) (int64, error)
	// AttachPayment links an invoice-less payment (e.g. a storefront order payment) to an
	// invoice; a verified payment is applied to the invoice (its ledger credit already exists).
	AttachPayment(ctx context.Context, tenantID, paymentID, invoiceID uuid.UUID) (Payment, error)
	PaymentsByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Payment, error)
}

type service struct {
	pool      *pgxpool.Pool
	repo      Repository
	customers Customers
	notifier  Notifier
	auditor   Auditor
	hooks     []PaidHook
	now       func() time.Time
}

// NewService builds billing. auditor may be nil (no audit rows; tests only).
func NewService(pool *pgxpool.Pool, customers Customers, notifier Notifier, auditor Auditor, hooks ...PaidHook) Service {
	return &service{pool: pool, customers: customers, notifier: notifier, auditor: auditor, hooks: hooks, now: time.Now}
}

// audit records a money action (SECURITY M9) inside the caller's transaction.
func (s *service) audit(ctx context.Context, tenantID, actorID uuid.UUID, action, entity string, id uuid.UUID, meta map[string]any) error {
	if s.auditor == nil {
		return nil
	}
	return s.auditor.Audit(ctx, tenantID, actor(actorID), action, entity, &id, meta)
}

// errOverpaid is returned when a payment would push an invoice past its total (QA BUG-04).
func errOverpaid(amount, due int64, currency string) error {
	return apperr.ValidationField("amount", fmt.Sprintf("amount %s is more than the remaining due %s; record the rest as a separate advance or enable overpayments",
		money.Format(amount, currency, true), money.Format(max(due, 0), currency, true)))
}

// checkNotOverpaid enforces amount <= remaining due unless the tenant allows overpayment.
func (s *service) checkNotOverpaid(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, inv Invoice, amount int64) error {
	due := inv.Total - inv.PaidAmount
	if amount <= due {
		return nil
	}
	settings, err := s.repo.TenantSettings(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	if AllowOverpayment(settings) {
		return nil
	}
	return errOverpaid(amount, due, inv.Currency)
}

func (s *service) invoicePaid(ctx context.Context, tenantID, invoiceID uuid.UUID) error {
	for _, h := range s.hooks {
		if err := h.InvoicePaid(ctx, tenantID, invoiceID); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) tx(ctx context.Context, tenantID uuid.UUID, fn db.TxFn) error {
	return db.WithTenantTx(ctx, s.pool, tenantID, fn)
}

func actor(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func strp(s string) *string { return &s }

// ---- invoices

func (s *service) CreateInvoice(ctx context.Context, tenantID, actorID uuid.UUID, in CreateInvoiceInput) (inv Invoice, err error) {
	subtotal, total, err := Totals(in.Items, in.Discount)
	if err != nil {
		return inv, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.customers.Get(ctx, tenantID, in.CustomerID); err != nil {
			return err
		}
		cur, err := s.repo.TenantCurrency(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		seq, err := s.repo.NextInvoiceSeq(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		status := InvoiceDraft
		if in.Send {
			status = InvoiceSent
		}
		if total == 0 {
			status = InvoicePaid
		}
		if inv, err = s.repo.InsertInvoice(ctx, tx, tenantID, InvoiceNumber(s.now(), seq), in, subtotal, total, status, cur, actor(actorID)); err != nil {
			return err
		}
		for _, it := range in.Items {
			if err := s.repo.InsertItem(ctx, tx, tenantID, inv.ID, it); err != nil {
				return err
			}
		}
		if total > 0 {
			if err := s.ledger(ctx, tx, tenantID, inv.CustomerID, Debit, total, cur, "invoice", inv.ID, strp(inv.Number), actor(actorID)); err != nil {
				return err
			}
		}
		if status == InvoicePaid {
			if err := s.invoicePaid(ctx, tenantID, inv.ID); err != nil {
				return err
			}
		}
		if in.Send {
			// The invoice is issued even if the customer has no reachable channel yet
			// (validation error from the notifier); sendInvoice reports that explicitly.
			if err := s.notify(ctx, tenantID, inv, ""); err != nil && !errors.Is(err, apperr.ErrValidation) {
				return err
			}
		}
		return nil
	})
	return inv, err
}

func (s *service) notify(ctx context.Context, tenantID uuid.UUID, inv Invoice, channel string) error {
	if s.notifier == nil {
		return nil
	}
	body := fmt.Sprintf("ইনভয়েস %s — মোট %s, বাকি %s", inv.Number, money.Format(inv.Total, inv.Currency, true), money.Format(inv.DueAmount(), inv.Currency, true))
	return s.notifier.SendToCustomer(ctx, tenantID, inv.CustomerID, channel, body)
}

// ledger writes a ledger row and moves the cached customers.due_balance in the same tx.
func (s *service) ledger(ctx context.Context, tx pgx.Tx, tenantID, customerID uuid.UUID, typ string, amount int64, currency, refType string, refID uuid.UUID, note *string, by *uuid.UUID) error {
	if _, err := s.repo.InsertLedger(ctx, tx, tenantID, LedgerEntry{CustomerID: customerID, EntryType: typ, Amount: amount, Currency: currency,
		RefType: &refType, RefID: &refID, Note: note, CreatedBy: by}); err != nil {
		return err
	}
	delta := amount
	if typ == Credit {
		delta = -amount
	}
	return s.customers.AdjustBalance(ctx, tenantID, customerID, delta, 0)
}

func (s *service) SendInvoice(ctx context.Context, tenantID, id uuid.UUID, channel string) (inv Invoice, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if inv, err = s.repo.GetInvoice(ctx, tx, tenantID, id, true); err != nil {
			return err
		}
		if inv.Status == InvoiceVoid {
			return apperr.Validation("invoice is void")
		}
		if inv.Status == InvoiceDraft {
			if err := s.repo.SetInvoiceStatus(ctx, tx, tenantID, id, InvoiceSent, nil); err != nil {
				return err
			}
			inv.Status = InvoiceSent
		}
		return s.notify(ctx, tenantID, inv, strings.ToLower(channel))
	})
	return inv, err
}

func (s *service) VoidInvoice(ctx context.Context, tenantID, actorID, id uuid.UUID, reason *string) (inv Invoice, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if inv, err = s.repo.GetInvoice(ctx, tx, tenantID, id, true); err != nil {
			return err
		}
		if inv.Status == InvoiceVoid {
			return apperr.Validation("invoice is already void")
		}
		if inv.PaidAmount > 0 {
			return apperr.Validation("invoice has payments; refund them before voiding")
		}
		note := "void"
		if reason != nil && *reason != "" {
			note = "void: " + *reason
		}
		if err := s.repo.SetInvoiceStatus(ctx, tx, tenantID, id, InvoiceVoid, &note); err != nil {
			return err
		}
		if inv.Total > 0 {
			if err := s.ledger(ctx, tx, tenantID, inv.CustomerID, Credit, inv.Total, inv.Currency, "invoice", inv.ID, &note, actor(actorID)); err != nil {
				return err
			}
		}
		if err := s.audit(ctx, tenantID, actorID, "invoice.void", "invoice", id, map[string]any{"number": inv.Number,
			"total": inv.Total, "reason": deref(reason)}); err != nil {
			return err
		}
		inv, err = s.repo.GetInvoice(ctx, tx, tenantID, id, false)
		return err
	})
	return inv, err
}

func (s *service) Invoice(ctx context.Context, tenantID, id uuid.UUID) (inv Invoice, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		inv, err = s.repo.GetInvoice(ctx, tx, tenantID, id, false)
		return err
	})
	return inv, err
}

func (s *service) Invoices(ctx context.Context, tenantID uuid.UUID, f InvoiceFilter, page pagination.Page) (items []Invoice, more bool, total int, err error) {
	if f.Status != nil {
		v := strings.ToLower(*f.Status)
		f.Status = &v
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.repo.ListInvoices(ctx, tx, tenantID, f, page)
		if err != nil {
			return err
		}
		items, more = pagination.Trim(rows, page.First)
		total, err = s.repo.CountInvoices(ctx, tx, tenantID, f)
		return err
	})
	return items, more, total, err
}

func (s *service) InvoicesByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Invoice, error) {
	out := map[uuid.UUID]Invoice{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		is, err := s.repo.InvoicesByIDs(ctx, tx, tenantID, ids)
		for _, i := range is {
			out[i.ID] = i
		}
		return err
	})
	return out, err
}

func (s *service) ItemsByInvoices(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]InvoiceItem, error) {
	out := map[uuid.UUID][]InvoiceItem{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		is, err := s.repo.ItemsByInvoices(ctx, tx, tenantID, ids)
		for _, i := range is {
			out[i.InvoiceID] = append(out[i.InvoiceID], i)
		}
		return err
	})
	return out, err
}

func (s *service) PaymentsByInvoices(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]Payment, error) {
	out := map[uuid.UUID][]Payment{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ps, err := s.repo.PaymentsByInvoices(ctx, tx, tenantID, ids)
		for _, p := range ps {
			out[*p.InvoiceID] = append(out[*p.InvoiceID], p)
		}
		return err
	})
	return out, err
}

func (s *service) MarkOverdue(ctx context.Context, tenantID uuid.UUID, now time.Time) (n int64, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		n, err = s.repo.MarkOverdue(ctx, tx, tenantID, now)
		return err
	})
	return n, err
}

// ---- payments

// RecordPayment stores a payment. Verification rules:
//   - opts.ForceVerified (seller asserts receipt, e.g. createSubscription.markPaid) -> verified/manual;
//   - a TrxID that matches an unmatched SMS in payment_inbox (same amount) -> verified/sms_match;
//   - cash/bank/card/other without TrxID -> verified/manual;
//   - otherwise (mobile money awaiting SMS) -> pending until verifyPayment / SMS arrives.
//
// Verified payments update invoice paid_amount/status, write a ledger credit and reduce due.
func (s *service) RecordPayment(ctx context.Context, tenantID, actorID uuid.UUID, invoiceID, customerID *uuid.UUID, in PaymentInput, opts RecordOptions) (p Payment, err error) {
	if err := in.Validate(); err != nil {
		return p, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.TenantCurrency(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		if invoiceID != nil {
			inv, err := s.repo.GetInvoice(ctx, tx, tenantID, *invoiceID, true)
			if err != nil {
				return err
			}
			switch {
			case inv.Status == InvoiceVoid:
				return apperr.Validation("invoice is void")
			case inv.Status == InvoicePaid && inv.Total > 0:
				return apperr.Validation("invoice is already paid")
			case customerID != nil && *customerID != inv.CustomerID:
				return apperr.Validation("invoice belongs to another customer")
			}
			if err := s.checkNotOverpaid(ctx, tx, tenantID, inv, in.Amount); err != nil {
				return err
			}
			customerID, cur = &inv.CustomerID, inv.Currency
		}
		if customerID == nil {
			return apperr.Validation("invoiceId or customerId is required")
		}
		if _, err := s.customers.Get(ctx, tenantID, *customerID); err != nil {
			return err
		}
		now := s.now()
		p = Payment{CustomerID: customerID, InvoiceID: invoiceID, Method: in.Method, TrxID: in.TrxID, SenderNumber: in.SenderNumber,
			Amount: in.Amount, Currency: cur, Status: PaymentPending, Notes: in.Notes, ReceivedAt: now}
		if in.ReceivedAt != nil {
			p.ReceivedAt = *in.ReceivedAt
		}
		source := SourceManual
		if opts.Source != "" {
			source = opts.Source
		}
		var match *InboxItem
		if in.TrxID != nil && !opts.ForceVerified {
			m, err := s.repo.UnmatchedInbox(ctx, tx, tenantID, *in.TrxID, in.Amount)
			if err == nil {
				match, source = &m, SourceSMSMatch
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		if opts.ForceVerified || match != nil || (in.TrxID == nil && !IsMobileMoney(in.Method) && opts.Source == "") {
			p.Status, p.VerifiedAt, p.VerifiedBy = PaymentVerified, &now, actor(actorID)
		}
		p.VerificationSource = &source
		if p, err = s.repo.InsertPayment(ctx, tx, tenantID, p); err != nil {
			if db.IsUniqueViolation(err, "payments_trx_unique") {
				return apperr.Conflict("this TrxID has already been used for another payment")
			}
			return db.MapErr(err, "payment")
		}
		if match != nil {
			if err := s.repo.SetInboxStatus(ctx, tx, tenantID, match.ID, "matched", &p.ID); err != nil {
				return err
			}
		}
		if p.Status == PaymentVerified {
			if err := s.audit(ctx, tenantID, actorID, "payment.verify", "payment", p.ID, map[string]any{"amount": p.Amount,
				"method": p.Method, "trx_id": p.TrxID, "source": source, "invoice_id": p.InvoiceID, "on_record": true}); err != nil {
				return err
			}
			return s.applyVerified(ctx, tx, tenantID, p, actor(actorID))
		}
		return nil
	})
	return p, err
}

// applyVerified books a verified payment against its invoice and the customer ledger.
func (s *service) applyVerified(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, p Payment, by *uuid.UUID) error {
	if p.InvoiceID != nil {
		inv, err := s.repo.GetInvoice(ctx, tx, tenantID, *p.InvoiceID, true)
		if err != nil {
			return err
		}
		paid := inv.PaidAmount + p.Amount
		status := StatusAfterPayment(inv.Status, paid, inv.Total)
		var paidAt *time.Time
		if status == InvoicePaid {
			t := s.now()
			paidAt = &t
		}
		if err := s.repo.UpdateInvoicePayment(ctx, tx, tenantID, inv.ID, paid, status, paidAt); err != nil {
			return err
		}
		if status == InvoicePaid && inv.Status != InvoicePaid {
			if err := s.invoicePaid(ctx, tenantID, inv.ID); err != nil {
				return err
			}
		}
	}
	if p.CustomerID == nil {
		return nil
	}
	note := p.Method
	if p.TrxID != nil {
		note += " " + *p.TrxID
	}
	if _, err := s.repo.InsertLedger(ctx, tx, tenantID, LedgerEntry{CustomerID: *p.CustomerID, EntryType: Credit, Amount: p.Amount,
		Currency: p.Currency, RefType: strp("payment"), RefID: &p.ID, Note: &note, CreatedBy: by}); err != nil {
		return err
	}
	return s.customers.AdjustBalance(ctx, tenantID, *p.CustomerID, -p.Amount, p.Amount)
}

func (s *service) VerifyPayment(ctx context.Context, tenantID, actorID, id uuid.UUID) (p Payment, err error) {
	return s.verify(ctx, tenantID, actorID, id, SourceManual, nil)
}

func (s *service) verify(ctx context.Context, tenantID, actorID, id uuid.UUID, source string, inboxID *uuid.UUID) (p Payment, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.GetPayment(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		if cur.Status != PaymentPending {
			return apperr.Validation("payment is already " + cur.Status)
		}
		if cur.InvoiceID != nil {
			inv, err := s.repo.GetInvoice(ctx, tx, tenantID, *cur.InvoiceID, true)
			if err != nil {
				return err
			}
			if inv.Status == InvoiceVoid {
				return apperr.Validation("invoice is void")
			}
			if err := s.checkNotOverpaid(ctx, tx, tenantID, inv, cur.Amount); err != nil {
				return err
			}
		}
		now := s.now()
		if p, err = s.repo.UpdatePaymentStatus(ctx, tx, tenantID, id, PaymentVerified, actor(actorID), &now, &source, nil); err != nil {
			return err
		}
		if inboxID == nil && p.TrxID != nil {
			if m, err := s.repo.UnmatchedInbox(ctx, tx, tenantID, *p.TrxID, p.Amount); err == nil {
				inboxID = &m.ID
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		if inboxID != nil {
			if err := s.repo.SetInboxStatus(ctx, tx, tenantID, *inboxID, "matched", &p.ID); err != nil {
				return err
			}
		}
		if err := s.audit(ctx, tenantID, actorID, "payment.verify", "payment", p.ID, map[string]any{"amount": p.Amount,
			"method": p.Method, "trx_id": p.TrxID, "source": source, "invoice_id": p.InvoiceID, "inbox_id": inboxID}); err != nil {
			return err
		}
		return s.applyVerified(ctx, tx, tenantID, p, actor(actorID))
	})
	return p, err
}

// RejectPayment rejects a pending payment, or reverses a verified one (wrongly verified,
// e.g. a spoofed SMS): the invoice paid amount/status, the ledger (a debit entry) and the
// customer's due/lifetime value are restored in the same transaction.
func (s *service) RejectPayment(ctx context.Context, tenantID, actorID, id uuid.UUID, reason *string) (p Payment, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.GetPayment(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		if cur.Status != PaymentPending && cur.Status != PaymentVerified {
			return apperr.Validation("payment is already " + cur.Status)
		}
		note := "rejected"
		if reason != nil && *reason != "" {
			note += ": " + *reason
		}
		if p, err = s.repo.UpdatePaymentStatus(ctx, tx, tenantID, id, PaymentRejected, actor(actorID), nil, nil, &note); err != nil {
			return err
		}
		if cur.Status == PaymentVerified {
			if err := s.reverseVerified(ctx, tx, tenantID, cur, note, actor(actorID)); err != nil {
				return err
			}
		}
		return s.audit(ctx, tenantID, actorID, "payment.reject", "payment", id, map[string]any{"amount": cur.Amount,
			"method": cur.Method, "previous_status": cur.Status, "reason": deref(reason), "invoice_id": cur.InvoiceID})
	})
	return p, err
}

// reverseVerified undoes applyVerified for a payment that is being rejected.
func (s *service) reverseVerified(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, p Payment, note string, by *uuid.UUID) error {
	if p.InvoiceID != nil {
		inv, err := s.repo.GetInvoice(ctx, tx, tenantID, *p.InvoiceID, true)
		if err != nil {
			return err
		}
		paid := max(0, inv.PaidAmount-p.Amount)
		status := StatusAfterPayment(inv.Status, paid, inv.Total)
		var paidAt *time.Time
		if status == InvoicePaid {
			paidAt = inv.PaidAt
		}
		if err := s.repo.UpdateInvoicePayment(ctx, tx, tenantID, inv.ID, paid, status, paidAt); err != nil {
			return err
		}
	}
	if p.CustomerID == nil {
		return nil
	}
	if _, err := s.repo.InsertLedger(ctx, tx, tenantID, LedgerEntry{CustomerID: *p.CustomerID, EntryType: Debit, Amount: p.Amount,
		Currency: p.Currency, RefType: strp("payment"), RefID: &p.ID, Note: &note, CreatedBy: by}); err != nil {
		return err
	}
	return s.customers.AdjustBalance(ctx, tenantID, *p.CustomerID, p.Amount, -p.Amount)
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *service) Payment(ctx context.Context, tenantID, id uuid.UUID) (p Payment, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		p, err = s.repo.GetPayment(ctx, tx, tenantID, id, false)
		return err
	})
	return p, err
}

func (s *service) Payments(ctx context.Context, tenantID uuid.UUID, f PaymentFilter, page pagination.Page) (items []Payment, more bool, total int, err error) {
	for _, p := range []**string{&f.Status, &f.Method} {
		if *p != nil {
			v := strings.ToLower(**p)
			*p = &v
		}
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.repo.ListPayments(ctx, tx, tenantID, f, page)
		if err != nil {
			return err
		}
		items, more = pagination.Trim(rows, page.First)
		total, err = s.repo.CountPayments(ctx, tx, tenantID, f)
		return err
	})
	return items, more, total, err
}

// ---- payment inbox

// IngestSMS stores a forwarded SMS and auto-verifies a pending payment with the same
// method + TrxID + amount, but only when the SMS came from an allowed sender id for that
// method (SECURITY H4). A parsed payment SMS from any other sender (e.g. a customer's
// personal number copying bKash wording) is stored as needs_review and never verifies.
func (s *service) IngestSMS(ctx context.Context, tenantID uuid.UUID, sender, raw string, receivedAt time.Time) (item InboxItem, err error) {
	if strings.TrimSpace(raw) == "" {
		return item, apperr.Validation("empty sms")
	}
	sender = strings.TrimSpace(sender)
	if sender == "" {
		return item, apperr.ValidationField("from", "sender is required")
	}
	if len(sender) > 64 {
		sender = sender[:64]
	}
	parsed, perr := ParsePaymentSMS(raw)
	item = InboxItem{Source: "sms", Sender: &sender, RawText: raw, Status: "unmatched", ReceivedAt: receivedAt}
	switch {
	case errors.Is(perr, ErrNotPaymentSMS):
		item.Status = "ignored"
	case perr != nil:
		item.Status = "parse_failed"
	}
	if parsed.Method != "" && perr == nil {
		item.Method, item.TrxID, item.Amount = &parsed.Method, &parsed.TrxID, &parsed.Amount
		if parsed.Sender != "" {
			item.SenderNumber = &parsed.Sender
		}
		if parsed.ReceivedAt != nil {
			item.ReceivedAt = *parsed.ReceivedAt
		}
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if item.Status == "unmatched" {
			settings, err := s.repo.TenantSettings(ctx, tx, tenantID)
			if err != nil {
				return err
			}
			if !SenderAllowed(*item.Method, sender, settings) {
				item.Status = "needs_review"
			}
		}
		if item, err = s.repo.InsertInbox(ctx, tx, tenantID, item); err != nil {
			return err
		}
		if item.Status != "unmatched" {
			return nil
		}
		p, err := s.repo.PendingPaymentByTrx(ctx, tx, tenantID, *item.Method, *item.TrxID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && p.Amount != *item.Amount) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := s.verify(ctx, tenantID, uuid.Nil, p.ID, SourceSMSMatch, &item.ID); err != nil {
			if errors.Is(err, apperr.ErrValidation) { // e.g. would overpay: leave it for a human
				return s.repo.SetInboxStatus(ctx, tx, tenantID, item.ID, "needs_review", nil)
			}
			return err
		}
		item.Status, item.MatchedPaymentID = "matched", &p.ID
		return nil
	})
	return item, err
}

func (s *service) PaymentInbox(ctx context.Context, tenantID uuid.UUID, status *string, page pagination.Page) (out []InboxItem, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.ListInbox(ctx, tx, tenantID, status, page)
		return err
	})
	return out, err
}

func (s *service) MatchInbox(ctx context.Context, tenantID, actorID, inboxID, paymentID uuid.UUID) (p Payment, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		item, err := s.repo.GetInbox(ctx, tx, tenantID, inboxID)
		if err != nil {
			return err
		}
		if item.Status == "matched" {
			return apperr.Validation("inbox item is already matched")
		}
		cur, err := s.repo.GetPayment(ctx, tx, tenantID, paymentID, true)
		if err != nil {
			return err
		}
		if item.Amount != nil && *item.Amount != cur.Amount {
			return apperr.Validation(fmt.Sprintf("amount mismatch: SMS %s vs payment %s",
				money.Format(*item.Amount, cur.Currency, true), money.Format(cur.Amount, cur.Currency, true)))
		}
		if item.TrxID != nil && cur.TrxID != nil && !strings.EqualFold(*item.TrxID, *cur.TrxID) {
			return apperr.Validation("TrxID mismatch")
		}
		switch cur.Status {
		case PaymentPending:
			p, err = s.verify(ctx, tenantID, actorID, paymentID, SourceSMSMatch, &inboxID)
			return err
		case PaymentVerified:
			p = cur
			return s.repo.SetInboxStatus(ctx, tx, tenantID, inboxID, "matched", &paymentID)
		default:
			return apperr.Validation("payment is " + cur.Status)
		}
	})
	return p, err
}

func (s *service) IgnoreInbox(ctx context.Context, tenantID, inboxID uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		item, err := s.repo.GetInbox(ctx, tx, tenantID, inboxID)
		if err != nil {
			return err
		}
		if item.Status == "matched" {
			return apperr.Validation("inbox item is already matched")
		}
		return s.repo.SetInboxStatus(ctx, tx, tenantID, inboxID, "ignored", nil)
	})
}

// ---- ledger

func (s *service) Ledger(ctx context.Context, tenantID, customerID uuid.UUID) (out []LedgerEntry, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.Ledger(ctx, tx, tenantID, customerID)
		return err
	})
	return out, err
}

func (s *service) AddLedgerAdjustment(ctx context.Context, tenantID, actorID, customerID uuid.UUID, entryType string, amount int64, note *string) (e LedgerEntry, err error) {
	entryType = strings.ToLower(entryType)
	if entryType != Debit && entryType != Credit {
		return e, apperr.ValidationField("entryType", "must be DEBIT or CREDIT")
	}
	if amount <= 0 {
		return e, apperr.ValidationField("amount", "must be > 0")
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.customers.Get(ctx, tenantID, customerID); err != nil {
			return err
		}
		cur, err := s.repo.TenantCurrency(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		if e, err = s.repo.InsertLedger(ctx, tx, tenantID, LedgerEntry{CustomerID: customerID, EntryType: entryType, Amount: amount,
			Currency: cur, RefType: strp("adjustment"), Note: note, CreatedBy: actor(actorID)}); err != nil {
			return err
		}
		delta := amount
		if entryType == Credit {
			delta = -amount
		}
		if err := s.customers.AdjustBalance(ctx, tenantID, customerID, delta, 0); err != nil {
			return err
		}
		return s.audit(ctx, tenantID, actorID, "ledger.adjust", "customer", customerID, map[string]any{"entry_type": entryType,
			"amount": amount, "note": deref(note), "ledger_entry_id": e.ID})
	})
	return e, err
}

func (s *service) AttachPayment(ctx context.Context, tenantID, paymentID, invoiceID uuid.UUID) (p Payment, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := s.repo.GetPayment(ctx, tx, tenantID, paymentID, true)
		if err != nil {
			return err
		}
		if cur.InvoiceID != nil {
			if *cur.InvoiceID == invoiceID {
				p = cur
				return nil
			}
			return apperr.Validation("payment is already linked to another invoice")
		}
		inv, err := s.repo.GetInvoice(ctx, tx, tenantID, invoiceID, true)
		if err != nil {
			return err
		}
		switch {
		case inv.Status == InvoiceVoid:
			return apperr.Validation("invoice is void")
		case cur.CustomerID != nil && *cur.CustomerID != inv.CustomerID:
			return apperr.Validation("invoice belongs to another customer")
		}
		if err := s.repo.SetPaymentInvoice(ctx, tx, tenantID, paymentID, invoiceID); err != nil {
			return err
		}
		cur.InvoiceID = &invoiceID
		if cur.Status == PaymentVerified {
			paid := inv.PaidAmount + cur.Amount
			status := StatusAfterPayment(inv.Status, paid, inv.Total)
			var paidAt *time.Time
			if status == InvoicePaid {
				t := s.now()
				paidAt = &t
			}
			if err := s.repo.UpdateInvoicePayment(ctx, tx, tenantID, inv.ID, paid, status, paidAt); err != nil {
				return err
			}
			if status == InvoicePaid && inv.Status != InvoicePaid {
				if err := s.invoicePaid(ctx, tenantID, inv.ID); err != nil {
					return err
				}
			}
		}
		p = cur
		return nil
	})
	return p, err
}

func (s *service) PaymentsByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Payment, error) {
	out := map[uuid.UUID]Payment{}
	err := s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		ps, err := s.repo.PaymentsByIDs(ctx, tx, tenantID, ids)
		for _, p := range ps {
			out[p.ID] = p
		}
		return err
	})
	return out, err
}
