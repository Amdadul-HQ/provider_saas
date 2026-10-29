package graph

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"provider_saas/internal/modules/auth"
	"provider_saas/internal/modules/billing"
	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/content"
	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/inbox"
	"provider_saas/internal/modules/inventory"
	"provider_saas/internal/modules/reminder"
	"provider_saas/internal/modules/social"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/modules/ticket"
	"provider_saas/internal/platform/gql"
)

type ID = uuid.UUID

// Loaders batch the N+1-prone lookups of one request. Entity loaders resolve to nil
// for missing ids; count loaders resolve to 0.
type Loaders struct {
	User          *gql.Loader[ID, *auth.User]
	Customer      *gql.Loader[ID, *customer.Customer]
	Product       *gql.Loader[ID, *catalog.Product]
	Plan          *gql.Loader[ID, *catalog.Plan]
	Slot          *gql.Loader[ID, *inventory.Slot]
	Supplier      *gql.Loader[ID, *inventory.Supplier]
	Subscription  *gql.Loader[ID, *subscription.Subscription]
	Invoice       *gql.Loader[ID, *billing.Invoice]
	PlansByProd   *gql.Loader[ID, []catalog.Plan]
	IdentitiesBy  *gql.Loader[ID, []customer.Identity]
	SubsByCust    *gql.Loader[ID, []subscription.Subscription]
	SlotsByMaster *gql.Loader[ID, []inventory.Slot]
	ItemsByInv    *gql.Loader[ID, []billing.InvoiceItem]
	PaysByInv     *gql.Loader[ID, []billing.Payment]
	FreeByProd    *gql.Loader[ID, int]
	FreeByMaster  *gql.Loader[ID, int]
	ActiveByProd  *gql.Loader[ID, int]
	ActiveByCust  *gql.Loader[ID, int]
	AcctsBySupp   *gql.Loader[ID, int]

	SocialAccount  *gql.Loader[ID, *social.Account]
	OpenConvByAcct *gql.Loader[ID, int]
	Template       *gql.Loader[ID, *inbox.Template]
	Rule           *gql.Loader[ID, *reminder.Rule]
	Payment        *gql.Loader[ID, *billing.Payment]
	CommentsByTkt  *gql.Loader[ID, []ticket.Comment]
	TargetsByPost  *gql.Loader[ID, []content.Target]
}

type loadersKey struct{}

// tenantFetch adapts a tenant-scoped batch function to a loader fetch function.
func tenantFetch[V any](fn func(ctx context.Context, tid ID, ids []ID) (map[ID]V, error)) func(context.Context, []ID) (map[ID]V, error) {
	return func(ctx context.Context, ids []ID) (map[ID]V, error) {
		tid, _, err := tenantOf(ctx)
		if err != nil {
			return nil, err
		}
		return fn(ctx, tid, ids)
	}
}

// entity turns map[ID]T into a loader of *T.
func entity[T any](fn func(ctx context.Context, tid ID, ids []ID) (map[ID]T, error)) *gql.Loader[ID, *T] {
	return gql.NewLoader(tenantFetch(func(ctx context.Context, tid ID, ids []ID) (map[ID]*T, error) {
		m, err := fn(ctx, tid, ids)
		if err != nil {
			return nil, err
		}
		out := make(map[ID]*T, len(m))
		for k, v := range m {
			out[k] = &v
		}
		return out, nil
	}))
}

func (r *Resolver) newLoaders() *Loaders {
	return &Loaders{
		User: gql.NewLoader(func(ctx context.Context, ids []ID) (map[ID]*auth.User, error) {
			m, err := r.AuthSvc.UsersByIDs(ctx, ids)
			out := make(map[ID]*auth.User, len(m))
			for k, v := range m {
				out[k] = &v
			}
			return out, err
		}),
		Customer:      entity(r.CustomerSvc.GetByIDs),
		Product:       entity(r.CatalogSvc.GetByIDs),
		Plan:          entity(r.CatalogSvc.PlansByIDs),
		Slot:          entity(r.InventorySvc.SlotsByIDs),
		Supplier:      entity(r.InventorySvc.SuppliersByIDs),
		Subscription:  entity(r.SubscriptionSvc.GetByIDs),
		Invoice:       entity(r.BillingSvc.InvoicesByIDs),
		PlansByProd:   gql.NewLoader(tenantFetch(r.CatalogSvc.PlansByProducts)),
		IdentitiesBy:  gql.NewLoader(tenantFetch(r.CustomerSvc.IdentitiesByCustomers)),
		SubsByCust:    gql.NewLoader(tenantFetch(r.SubscriptionSvc.ByCustomers)),
		SlotsByMaster: gql.NewLoader(tenantFetch(r.InventorySvc.SlotsByMasters)),
		ItemsByInv:    gql.NewLoader(tenantFetch(r.BillingSvc.ItemsByInvoices)),
		PaysByInv:     gql.NewLoader(tenantFetch(r.BillingSvc.PaymentsByInvoices)),
		FreeByProd:    gql.NewLoader(tenantFetch(r.InventorySvc.FreeCountsByProduct)),
		FreeByMaster:  gql.NewLoader(tenantFetch(r.InventorySvc.FreeCountsByMaster)),
		ActiveByProd:  gql.NewLoader(tenantFetch(r.SubscriptionSvc.ActiveCountByProducts)),
		ActiveByCust:  gql.NewLoader(tenantFetch(r.SubscriptionSvc.ActiveCountByCustomers)),
		AcctsBySupp:   gql.NewLoader(tenantFetch(r.InventorySvc.AccountCountsBySupplier)),

		SocialAccount:  entity(r.SocialSvc.GetByIDs),
		OpenConvByAcct: gql.NewLoader(tenantFetch(r.InboxSvc.OpenCountsByAccount)),
		Template:       entity(r.InboxSvc.TemplatesByIDs),
		Rule:           entity(r.ReminderSvc.RulesByIDs),
		Payment:        entity(r.BillingSvc.PaymentsByIDs),
		CommentsByTkt:  gql.NewLoader(tenantFetch(r.TicketSvc.CommentsByTickets)),
		TargetsByPost:  gql.NewLoader(tenantFetch(r.ContentSvc.TargetsByPosts)),
	}
}

// Middleware installs per-request loaders and client metadata.
func (r *Resolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), loadersKey{}, r.newLoaders())
		ctx = WithClientMeta(ctx, req)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

func (r *Resolver) loaders(ctx context.Context) *Loaders {
	if l, ok := ctx.Value(loadersKey{}).(*Loaders); ok {
		return l
	}
	return r.newLoaders() // e.g. tests calling resolvers directly
}
