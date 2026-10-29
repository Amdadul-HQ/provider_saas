package graph

import (
	"context"

	"github.com/99designs/gqlgen/graphql"

	"provider_saas/graph/generated"
	"provider_saas/graph/model"
	"provider_saas/internal/modules/analytics"
	"provider_saas/internal/modules/auth"
	"provider_saas/internal/modules/billing"
	"provider_saas/internal/modules/catalog"
	"provider_saas/internal/modules/content"
	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/inbox"
	"provider_saas/internal/modules/inventory"
	"provider_saas/internal/modules/reminder"
	"provider_saas/internal/modules/social"
	"provider_saas/internal/modules/storefront"
	"provider_saas/internal/modules/subscription"
	"provider_saas/internal/modules/tenant"
	"provider_saas/internal/modules/ticket"
	"provider_saas/internal/platform/gql"
)

// Resolver is the root resolver. Module services are injected by internal/app.Container.
// Resolvers stay thin: parse input -> call service -> map to GraphQL model.
type Resolver struct {
	AuthSvc         auth.Service
	TenantSvc       tenant.Service
	CustomerSvc     customer.Service
	CatalogSvc      catalog.Service
	InventorySvc    inventory.Service
	SubscriptionSvc subscription.Service
	BillingSvc      billing.Service
	AnalyticsSvc    analytics.Service
	SocialSvc       social.Service
	InboxSvc        inbox.Service
	ContentSvc      content.Service
	TicketSvc       ticket.Service
	ReminderSvc     reminder.Service
	StorefrontSvc   storefront.Service
}

// NewExecutableSchema wires resolvers and directives.
func NewExecutableSchema(r *Resolver) graphql.ExecutableSchema {
	cfg := generated.Config{Resolvers: r}
	cfg.Directives.Auth = func(ctx context.Context, _ any, next graphql.Resolver, roles []model.Role) (any, error) {
		rs := make([]string, len(roles))
		for i, role := range roles {
			rs[i] = string(role)
		}
		if err := gql.RequireAuth(ctx, rs); err != nil {
			return nil, err
		}
		return next(ctx)
	}
	return generated.NewExecutableSchema(cfg)
}
