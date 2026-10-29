package storefront

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/modules/billing"
	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/httpx"
	"provider_saas/internal/platform/money"
	"provider_saas/internal/platform/pagination"
	"provider_saas/internal/platform/phone"
)

// Service is the storefront module API.
type Service interface {
	Get(ctx context.Context, tenantID uuid.UUID) (Storefront, error)
	Upsert(ctx context.Context, tenantID uuid.UUID, in Input) (Storefront, error)
	PublicURL(slug string) string

	// Public (unauthenticated) API.
	Public(ctx context.Context, slug string) (PublicStorefront, error)
	PlaceOrder(ctx context.Context, clientKey string, in PlaceOrderInput) (PublicOrder, error)
	PublicOrder(ctx context.Context, slug string, number int, phone string) (PublicOrder, error)
	Portal(ctx context.Context, token string) (PortalView, error)
	PortalRequestRenewal(ctx context.Context, token string, subscriptionID uuid.UUID, method, trxID *string) (PublicOrder, error)

	Orders(ctx context.Context, tenantID uuid.UUID, f OrderFilter, after *uuid.UUID, limit int) ([]Order, error)
	Order(ctx context.Context, tenantID, id uuid.UUID) (Order, error)
	ConfirmOrder(ctx context.Context, tenantID, actorID, id uuid.UUID, slotID *uuid.UUID, note *string) (Order, error)
	CancelOrder(ctx context.Context, tenantID, actorID, id uuid.UUID, reason *string) (Order, error)
	MarkDeliveredBySubscription(ctx context.Context, tenantID, subscriptionID uuid.UUID) error

	// CreatePortalLink returns the magic-link URL; sendVia ("" = don't send) messages it to the customer.
	CreatePortalLink(ctx context.Context, tenantID, customerID uuid.UUID, sendVia string) (string, error)
}

type Deps struct {
	Customers     Customers
	Catalog       Catalog
	Inventory     Inventory
	Billing       Billing
	Subscriptions Subscriptions
	Messenger     Messenger
	Notifier      Notifier
	PublicAppURL  string
}

type service struct {
	pool    *pgxpool.Pool
	repo    Repository
	d       Deps
	limiter *httpx.RateLimiter
	now     func() time.Time
}

// OrderRate limits public order placement per client: 5 burst, then 1 per 20s.
const (
	orderRatePerSec = 1.0 / 20
	orderBurst      = 5
)

func NewService(pool *pgxpool.Pool, d Deps) Service {
	return &service{pool: pool, d: d, limiter: httpx.NewRateLimiter(orderRatePerSec, orderBurst), now: time.Now}
}

func (s *service) tx(ctx context.Context, tenantID uuid.UUID, fn db.TxFn) error {
	return db.WithTenantTx(ctx, s.pool, tenantID, fn)
}

func strp(v string) *string {
	if v = strings.TrimSpace(v); v == "" {
		return nil
	}
	return &v
}

func actor(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func (s *service) PublicURL(slug string) string {
	return strings.TrimRight(s.d.PublicAppURL, "/") + "/s/" + slug
}

func (s *service) Get(ctx context.Context, tenantID uuid.UUID) (sf Storefront, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		sf, err = s.repo.ByTenant(ctx, tx, tenantID)
		return db.MapErr(err, "storefront")
	})
	return sf, err
}

func (s *service) Upsert(ctx context.Context, tenantID uuid.UUID, in Input) (sf Storefront, err error) {
	pn, err := in.Validate()
	if err != nil {
		return sf, err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.repo.ByTenant(ctx, tx, tenantID); errors.Is(err, pgx.ErrNoRows) {
			if in.Slug == nil || in.DisplayName == nil {
				return apperr.Validation("slug and displayName are required to create the storefront")
			}
			if _, err := s.repo.Insert(ctx, tx, tenantID, *in.Slug, *in.DisplayName); err != nil {
				return s.slugErr(err)
			}
		} else if err != nil {
			return err
		}
		sf, err = s.repo.Update(ctx, tx, tenantID, in, pn)
		return s.slugErr(err)
	})
	return sf, err
}

func (s *service) slugErr(err error) error {
	if db.IsUniqueViolation(err, "") {
		return &apperr.Error{Kind: apperr.ErrConflict, Msg: "this storefront address is already taken", Field: "slug"}
	}
	return db.MapErr(err, "storefront")
}

// published resolves a public slug to its storefront (published only).
func (s *service) published(ctx context.Context, slug string) (Storefront, error) {
	var sf Storefront
	err := db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		sf, err = s.repo.BySlug(ctx, tx, strings.ToLower(strings.TrimSpace(slug)))
		return err
	})
	if err == nil && !sf.IsPublished {
		err = apperr.NotFound("storefront")
	}
	return sf, err
}

func (s *service) Public(ctx context.Context, slug string) (PublicStorefront, error) {
	sf, err := s.published(ctx, slug)
	if err != nil {
		return PublicStorefront{}, err
	}
	return s.publicView(ctx, sf)
}

func (s *service) publicView(ctx context.Context, sf Storefront) (out PublicStorefront, err error) {
	out.Storefront = sf
	active := true
	err = s.tx(ctx, sf.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		prods, err := s.d.Catalog.List(ctx, sf.TenantID, catalog.Filter{IsActive: &active}, 200)
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, len(prods))
		for i, p := range prods {
			ids[i] = p.ID
		}
		plans, err := s.d.Catalog.PlansByProducts(ctx, sf.TenantID, ids)
		if err != nil {
			return err
		}
		free, err := s.d.Inventory.FreeCountsByProduct(ctx, sf.TenantID, ids)
		if err != nil {
			return err
		}
		for _, p := range prods {
			pp := PublicProduct{ID: p.ID, Name: p.Name, Category: p.Category, Description: p.Description, ImageURL: p.ImageURL,
				InStock: p.DeliveryType != catalog.DeliverySharedSlot || free[p.ID] > 0}
			for _, pl := range plans[p.ID] {
				if pl.IsActive {
					pp.Plans = append(pp.Plans, PublicPlan{ID: pl.ID, Name: pl.Name, DurationDays: pl.DurationDays, Price: pl.Price, Currency: pl.Currency})
				}
			}
			if len(pp.Plans) > 0 {
				out.Products = append(out.Products, pp)
			}
		}
		return nil
	})
	return out, err
}

func (s *service) toPublicOrder(sf Storefront, o Order, productName string) PublicOrder {
	return PublicOrder{Number: o.Number, Status: o.Status, Amount: o.Amount, Currency: o.Currency, ProductName: productName,
		PaymentInstructions: sf.PaymentInstructions, PaymentNumbers: sf.PaymentNumbersValue()}
}

func (s *service) PlaceOrder(ctx context.Context, clientKey string, in PlaceOrderInput) (PublicOrder, error) {
	if !s.limiter.Allow(clientKey + "|" + strings.ToLower(in.Slug)) {
		return PublicOrder{}, apperr.Forbidden("too many orders from this device; please wait a minute and try again")
	}
	name := strings.TrimSpace(in.CustomerName)
	if name == "" || len([]rune(name)) > 100 {
		return PublicOrder{}, apperr.ValidationField("customerName", "required")
	}
	ph, ok := phone.Normalize(in.CustomerPhone)
	if !ok {
		return PublicOrder{}, apperr.ValidationField("customerPhone", "enter a valid mobile number (01XXXXXXXXX)")
	}
	if in.Quantity == 0 {
		in.Quantity = 1
	}
	if in.Quantity < 1 || in.Quantity > 10 {
		return PublicOrder{}, apperr.ValidationField("quantity", "must be 1..10")
	}
	if in.CustomerNote != nil && len([]rune(*in.CustomerNote)) > 500 {
		return PublicOrder{}, apperr.ValidationField("customerNote", "max 500 characters")
	}
	sf, err := s.published(ctx, in.Slug)
	if err != nil {
		return PublicOrder{}, err
	}
	var out PublicOrder
	var created Order
	err = s.tx(ctx, sf.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		product, plan, err := s.productPlan(ctx, sf.TenantID, in.ProductID, &in.PlanID)
		if err != nil {
			return err
		}
		cust, err := s.customerFor(ctx, sf.TenantID, name, ph, customer.CreateInput{Source: "storefront"})
		if err != nil {
			return err
		}
		o := Order{CustomerID: &cust.ID, ProductID: product.ID, PlanID: &plan.ID, Quantity: in.Quantity,
			Amount: plan.Price * int64(in.Quantity), Currency: plan.Currency, Source: SourceStorefront, Status: OrderAwaitingPayment,
			CustomerName: &name, CustomerPhone: &ph, CustomerNote: in.CustomerNote}
		if in.TrxID != nil && strings.TrimSpace(*in.TrxID) != "" {
			p, err := s.recordOrderPayment(ctx, sf.TenantID, cust.ID, o.Amount, in.PaymentMethod, in.TrxID, in.SenderNumber)
			if err != nil {
				return err
			}
			o.PaymentID = &p.ID
			if p.Status == billing.PaymentVerified {
				o.Status = OrderPaid
			}
		}
		if created, err = s.repo.InsertOrder(ctx, tx, sf.TenantID, o); err != nil {
			return err
		}
		out = s.toPublicOrder(sf, created, product.Name)
		body := fmt.Sprintf("%s — %s (%s)", product.Name, plan.Name, money.Format(created.Amount, created.Currency, true))
		link := "/orders"
		return s.d.Notifier.Notify(ctx, sf.TenantID, nil, "order.new", fmt.Sprintf("নতুন অর্ডার #%d — %s", created.Number, name), &body, &link)
	})
	return out, err
}

func (s *service) recordOrderPayment(ctx context.Context, tenantID, customerID uuid.UUID, amount int64, method, trx, sender *string) (billing.Payment, error) {
	m := "bkash"
	if method != nil && *method != "" {
		m = strings.ToLower(*method)
	}
	if sender != nil {
		if n, ok := phone.Normalize(*sender); ok {
			sender = &n
		}
	}
	p, err := s.d.Billing.RecordPayment(ctx, tenantID, uuid.Nil, nil, &customerID,
		billing.PaymentInput{Method: m, Amount: amount, TrxID: trx, SenderNumber: sender}, billing.RecordOptions{Source: billing.SourceStorefront})
	if errors.Is(err, apperr.ErrConflict) {
		return p, apperr.ValidationField("trxId", "this TrxID has already been used")
	}
	return p, err
}

func (s *service) productPlan(ctx context.Context, tenantID, productID uuid.UUID, planID *uuid.UUID) (catalog.Product, catalog.Plan, error) {
	product, err := s.d.Catalog.Get(ctx, tenantID, productID)
	if err != nil {
		return product, catalog.Plan{}, err
	}
	if !product.IsActive {
		return product, catalog.Plan{}, apperr.ValidationField("productId", "this product is not available")
	}
	if planID == nil {
		plans, err := s.d.Catalog.PlansByProducts(ctx, tenantID, []uuid.UUID{productID})
		if err != nil {
			return product, catalog.Plan{}, err
		}
		for _, pl := range plans[productID] {
			if pl.IsActive {
				return product, pl, nil
			}
		}
		return product, catalog.Plan{}, apperr.ValidationField("planId", "no active plan for this product")
	}
	plan, err := s.d.Catalog.GetPlan(ctx, tenantID, *planID)
	if err != nil {
		return product, plan, err
	}
	if plan.ProductID != productID || !plan.IsActive {
		return product, plan, apperr.ValidationField("planId", "this plan is not available")
	}
	return product, plan, nil
}

// customerFor matches a customer by phone or creates one.
func (s *service) customerFor(ctx context.Context, tenantID uuid.UUID, name, ph string, base customer.CreateInput) (customer.Customer, error) {
	c, err := s.d.Customers.FindByPhone(ctx, tenantID, ph)
	if err == nil || !errors.Is(err, apperr.ErrNotFound) {
		return c, err
	}
	base.Name, base.Phone = name, &ph
	return s.d.Customers.Create(ctx, tenantID, uuid.Nil, base)
}

func (s *service) PublicOrder(ctx context.Context, slug string, number int, ph string) (PublicOrder, error) {
	sf, err := s.published(ctx, slug)
	if err != nil {
		return PublicOrder{}, err
	}
	norm, _ := phone.Normalize(ph)
	var out PublicOrder
	err = s.tx(ctx, sf.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		o, err := s.repo.OrderByNumber(ctx, tx, sf.TenantID, number)
		if err != nil {
			return err
		}
		if o.CustomerPhone == nil || *o.CustomerPhone != norm {
			return apperr.NotFound("order")
		}
		p, err := s.d.Catalog.Get(ctx, sf.TenantID, o.ProductID)
		if err != nil {
			return err
		}
		out = s.toPublicOrder(sf, o, p.Name)
		return nil
	})
	return out, err
}

func (s *service) Orders(ctx context.Context, tenantID uuid.UUID, f OrderFilter, after *uuid.UUID, limit int) (out []Order, err error) {
	if limit <= 0 || limit > pagination.MaxFirst {
		limit = pagination.DefaultFirst
	}
	for _, p := range []**string{&f.Status, &f.Source} {
		if *p != nil {
			v := strings.ToLower(**p)
			*p = &v
		}
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		out, err = s.repo.ListOrders(ctx, tx, tenantID, f, after, limit)
		return err
	})
	return out, err
}

func (s *service) Order(ctx context.Context, tenantID, id uuid.UUID) (o Order, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		o, err = s.repo.GetOrder(ctx, tx, tenantID, id, false)
		return err
	})
	return o, err
}

// ConfirmOrder fulfils an order in one transaction: subscription (slot), invoice, payment
// link + verification, and delivery (now when a note is given or the product has no
// delivery template; otherwise the delivery.send job sends the template and marks it).
func (s *service) ConfirmOrder(ctx context.Context, tenantID, actorID, id uuid.UUID, slotID *uuid.UUID, note *string) (o Order, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if o, err = s.repo.GetOrder(ctx, tx, tenantID, id, true); err != nil {
			return err
		}
		switch {
		case o.Status == OrderCancelled || o.Status == OrderDelivered:
			return apperr.Validation("order is already " + o.Status)
		case o.SubscriptionID != nil:
			return apperr.Validation("order is already confirmed")
		}
		product, plan, err := s.productPlan(ctx, tenantID, o.ProductID, o.PlanID)
		if err != nil {
			return err
		}
		if o.CustomerID == nil {
			c, err := s.customerFor(ctx, tenantID, deref(o.CustomerName, "Customer"), deref(o.CustomerPhone, ""), customer.CreateInput{Source: "storefront"})
			if err != nil {
				return err
			}
			o.CustomerID = &c.ID
		}
		price := o.Amount
		sub, err := s.d.Subscriptions.Create(ctx, tenantID, actorID, subscription.CreateInput{CustomerID: *o.CustomerID,
			ProductID: product.ID, PlanID: &plan.ID, SlotID: slotID, Price: &price})
		if err != nil {
			return err
		}
		now := s.now()
		inv, err := s.d.Billing.CreateInvoice(ctx, tenantID, actorID, billing.CreateInvoiceInput{CustomerID: *o.CustomerID, DueAt: &now,
			Notes: strp(fmt.Sprintf("Order #%d", o.Number)), Items: []billing.ItemInput{{SubscriptionID: &sub.ID,
				Description: product.Name + " — " + plan.Name, Quantity: 1, UnitPrice: o.Amount}}})
		if err != nil {
			return err
		}
		o.Status = OrderAwaitingPayment
		if o.PaymentID != nil {
			p, err := s.d.Billing.AttachPayment(ctx, tenantID, *o.PaymentID, inv.ID)
			if err != nil {
				return err
			}
			if p.Status == billing.PaymentPending {
				if p, err = s.d.Billing.VerifyPayment(ctx, tenantID, actorID, p.ID); err != nil {
					return err
				}
			}
			if p.Status == billing.PaymentVerified {
				o.Status = OrderPaid
			}
		}
		if note != nil || product.DeliveryTemplate == nil {
			n := note
			if n == nil {
				n = strp(fmt.Sprintf("Order #%d confirmed", o.Number))
			}
			if _, err := s.d.Subscriptions.MarkDelivered(ctx, tenantID, sub.ID, n); err != nil {
				return err
			}
			o.Status = OrderDelivered
		} else if o.Status != OrderPaid {
			o.Status = OrderPaid // confirmed on credit (baki); delivery job sends the template
		}
		o.SubscriptionID = &sub.ID
		o, err = s.repo.UpdateOrder(ctx, tx, tenantID, o)
		return err
	})
	return o, err
}

func deref(p *string, def string) string {
	if p == nil {
		return def
	}
	return *p
}

func (s *service) CancelOrder(ctx context.Context, tenantID, actorID, id uuid.UUID, reason *string) (o Order, err error) {
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if o, err = s.repo.GetOrder(ctx, tx, tenantID, id, true); err != nil {
			return err
		}
		switch {
		case o.Status == OrderCancelled:
			return apperr.Validation("order is already cancelled")
		case o.SubscriptionID != nil || o.Status == OrderDelivered:
			return apperr.Validation("order is already fulfilled; cancel the subscription instead")
		}
		if o.PaymentID != nil {
			p, err := s.d.Billing.Payment(ctx, tenantID, *o.PaymentID)
			if err != nil {
				return err
			}
			if p.Status == billing.PaymentPending {
				if _, err := s.d.Billing.RejectPayment(ctx, tenantID, actorID, p.ID, reason); err != nil {
					return err
				}
			}
		}
		o.Status = OrderCancelled
		if reason != nil && strings.TrimSpace(*reason) != "" {
			line := "[cancelled: " + strings.TrimSpace(*reason) + "]"
			if o.CustomerNote != nil && *o.CustomerNote != "" {
				line = *o.CustomerNote + "\n" + line
			}
			o.CustomerNote = &line
		}
		o, err = s.repo.UpdateOrder(ctx, tx, tenantID, o)
		return err
	})
	return o, err
}

func (s *service) MarkDeliveredBySubscription(ctx context.Context, tenantID, subscriptionID uuid.UUID) error {
	return s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return s.repo.MarkDeliveredBySubscription(ctx, tx, tenantID, subscriptionID)
	})
}

// ---- portal

func (s *service) portalURL(token string) string {
	return strings.TrimRight(s.d.PublicAppURL, "/") + "/p/" + token
}

func (s *service) CreatePortalLink(ctx context.Context, tenantID, customerID uuid.UUID, sendVia string) (string, error) {
	token, hash, err := NewPortalToken()
	if err != nil {
		return "", err
	}
	err = s.tx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.d.Customers.Get(ctx, tenantID, customerID); err != nil {
			return err
		}
		return s.repo.InsertPortalToken(ctx, tx, tenantID, customerID, hash, s.now().Add(PortalTTL))
	})
	if err != nil {
		return "", err
	}
	u := s.portalURL(token)
	if sendVia = strings.ToLower(strings.TrimSpace(sendVia)); sendVia != "" {
		body := "আপনার সাবস্ক্রিপশন ও বিল দেখুন এবং রিনিউ করুন (৭ দিন বৈধ): " + u
		if err := s.d.Messenger.SendToCustomer(ctx, tenantID, customerID, sendVia, body); err != nil {
			return u, err
		}
	}
	return u, nil
}

func (s *service) portalCustomer(ctx context.Context, token string) (tenantID, customerID uuid.UUID, err error) {
	token = strings.TrimSpace(token)
	if len(token) < 20 || len(token) > 100 {
		return uuid.Nil, uuid.Nil, apperr.NotFound("portal link")
	}
	err = db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		tenantID, customerID, err = s.repo.PortalToken(ctx, tx, HashToken(token), s.now())
		return err
	})
	return tenantID, customerID, err
}

func (s *service) Portal(ctx context.Context, token string) (v PortalView, err error) {
	tid, cid, err := s.portalCustomer(ctx, token)
	if err != nil {
		return v, err
	}
	err = s.tx(ctx, tid, func(ctx context.Context, tx pgx.Tx) error {
		c, err := s.d.Customers.Get(ctx, tid, cid)
		if err != nil {
			return err
		}
		v.CustomerName, v.CustomerPhone, v.DueBalance = c.Name, c.Phone, c.DueBalance
		subs, err := s.d.Subscriptions.ByCustomers(ctx, tid, []uuid.UUID{cid})
		if err != nil {
			return err
		}
		var pids, planIDs []uuid.UUID
		for _, x := range subs[cid] {
			pids = append(pids, x.ProductID)
			if x.PlanID != nil {
				planIDs = append(planIDs, *x.PlanID)
			}
		}
		prods, err := s.d.Catalog.GetByIDs(ctx, tid, pids)
		if err != nil {
			return err
		}
		plans, err := s.d.Catalog.PlansByIDs(ctx, tid, planIDs)
		if err != nil {
			return err
		}
		now := s.now()
		for i, x := range subs[cid] {
			if i >= 50 {
				break
			}
			ps := PortalSubscription{ID: x.ID, ProductName: prods[x.ProductID].Name, StartsAt: x.StartsAt, ExpiresAt: x.ExpiresAt,
				DaysLeft: subscription.DaysLeft(x.ExpiresAt, now), Status: x.Status, DeliveryNote: x.DeliveryNote}
			if x.PlanID != nil {
				if pl, ok := plans[*x.PlanID]; ok {
					ps.PlanName = &pl.Name
				}
			}
			v.Subscriptions = append(v.Subscriptions, ps)
		}
		invs, _, _, err := s.d.Billing.Invoices(ctx, tid, billing.InvoiceFilter{CustomerID: &cid}, pagination.Page{First: 50})
		if err != nil {
			return err
		}
		for _, inv := range invs {
			if inv.Status == billing.InvoiceVoid { // drafts are already on the customer's ledger, so they show
				continue
			}
			v.Invoices = append(v.Invoices, PortalInvoice{ID: inv.ID, Number: inv.Number, Total: inv.Total, PaidAmount: inv.PaidAmount,
				Status: inv.Status, IssuedAt: inv.IssuedAt, DueAt: inv.DueAt})
		}
		sf, err := s.repo.ByTenant(ctx, tx, tid)
		if err != nil {
			return db.MapErr(err, "storefront")
		}
		v.Storefront, err = s.publicView(ctx, sf)
		return err
	})
	return v, err
}

func (s *service) PortalRequestRenewal(ctx context.Context, token string, subscriptionID uuid.UUID, method, trxID *string) (out PublicOrder, err error) {
	tid, cid, err := s.portalCustomer(ctx, token)
	if err != nil {
		return out, err
	}
	err = s.tx(ctx, tid, func(ctx context.Context, tx pgx.Tx) error {
		sub, err := s.d.Subscriptions.Get(ctx, tid, subscriptionID)
		if err != nil || sub.CustomerID != cid {
			return apperr.NotFound("subscription")
		}
		product, plan, err := s.productPlan(ctx, tid, sub.ProductID, sub.PlanID)
		if err != nil && sub.PlanID != nil { // the old plan was retired: fall back to the product's first active plan
			product, plan, err = s.productPlan(ctx, tid, sub.ProductID, nil)
		}
		if err != nil {
			return err
		}
		c, err := s.d.Customers.Get(ctx, tid, cid)
		if err != nil {
			return err
		}
		note := fmt.Sprintf("Renewal request for subscription %s (expires %s)", sub.ID, sub.ExpiresAt.Format("2006-01-02"))
		o := Order{CustomerID: &cid, ProductID: product.ID, PlanID: &plan.ID, Quantity: 1, Amount: plan.Price, Currency: plan.Currency,
			Source: SourcePortal, Status: OrderAwaitingPayment, CustomerName: &c.Name, CustomerPhone: c.Phone, CustomerNote: &note}
		if trxID != nil && strings.TrimSpace(*trxID) != "" {
			p, err := s.recordOrderPayment(ctx, tid, cid, o.Amount, method, trxID, nil)
			if err != nil {
				return err
			}
			o.PaymentID = &p.ID
			if p.Status == billing.PaymentVerified {
				o.Status = OrderPaid
			}
		}
		created, err := s.repo.InsertOrder(ctx, tx, tid, o)
		if err != nil {
			return err
		}
		sf, err := s.repo.ByTenant(ctx, tx, tid)
		if err != nil {
			return db.MapErr(err, "storefront")
		}
		out = s.toPublicOrder(sf, created, product.Name)
		body := product.Name + " — " + plan.Name
		link := "/orders"
		return s.d.Notifier.Notify(ctx, tid, nil, "order.renewal", fmt.Sprintf("রিনিউ অনুরোধ #%d — %s", created.Number, c.Name), &body, &link)
	})
	return out, err
}
