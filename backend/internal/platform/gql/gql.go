// Package gql wires the gqlgen server: transports, complexity limit, introspection
// (dev only), the @auth directive, error presenter and dataloader helpers.
package gql

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"provider_saas/internal/platform/apperr"
	"provider_saas/internal/platform/auth"
	"provider_saas/internal/platform/logger"
)

type Options struct {
	Dev             bool
	ComplexityLimit int
	Logger          *slog.Logger
}

// NewServer builds the GraphQL HTTP handler for an executable schema.
func NewServer(es graphql.ExecutableSchema, opts Options) *handler.Server {
	srv := handler.New(es)
	srv.AddTransport(transport.Options{})
	// POST only (SECURITY L10): GET would put queries and variables (phones, TrxIDs,
	// tokens) into URLs and access logs. The frontend sets preferGetMethod: false (L-017).
	srv.AddTransport(transport.POST{})
	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	if opts.Dev {
		srv.Use(extension.Introspection{})
	}
	srv.Use(extension.AutomaticPersistedQuery{Cache: lru.New[string](100)})
	if opts.ComplexityLimit > 0 {
		srv.Use(extension.FixedComplexityLimit(opts.ComplexityLimit))
	}
	srv.Use(DefaultLimits)
	srv.SetErrorPresenter(ErrorPresenter(opts.Dev))
	srv.SetRecoverFunc(func(ctx context.Context, p any) error {
		logger.From(ctx).Error("graphql panic", "panic", fmt.Sprint(p), "stack", string(debug.Stack()))
		return errors.New("internal server error")
	})
	return srv
}

// ErrorPresenter maps domain errors to extensions.code. Unknown errors are logged and,
// outside development, masked as INTERNAL_SERVER_ERROR.
func ErrorPresenter(dev bool) graphql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		gerr := graphql.DefaultErrorPresenter(ctx, err)
		var ge *gqlerror.Error
		if errors.As(err, &ge) && ge.Err == nil && ge.Extensions != nil {
			return gerr // parser/validation errors already carry a code
		}
		if code := apperr.Code(err); code != "" {
			if gerr.Extensions == nil {
				gerr.Extensions = map[string]any{}
			}
			gerr.Extensions["code"] = code
			if f := apperr.FieldOf(err); f != "" {
				gerr.Extensions["field"] = f
			}
			if code == "RATE_LIMITED" {
				// Seconds until a retry can succeed (at least 1), like the HTTP Retry-After header.
				gerr.Extensions["retryAfter"] = max(1, int(math.Ceil(apperr.RetryAfterOf(err).Seconds())))
			}
			return gerr
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			gerr.Message = "request cancelled"
			gerr.Extensions = map[string]any{"code": "TIMEOUT"}
			return gerr
		}
		if ge != nil && ge.Err == nil {
			return gerr
		}
		logger.From(ctx).Error("graphql internal error", "err", err, "path", gerr.Path.String())
		if gerr.Extensions == nil {
			gerr.Extensions = map[string]any{}
		}
		gerr.Extensions["code"] = "INTERNAL_SERVER_ERROR"
		if !dev {
			gerr.Message = "internal server error"
		}
		return gerr
	}
}

// RequireAuth implements @auth(roles): anonymous -> UNAUTHENTICATED, wrong role ->
// FORBIDDEN. Platform admins pass role checks.
func RequireAuth(ctx context.Context, roles []string) error {
	p, ok := auth.FromContext(ctx)
	if !ok {
		return apperr.Unauthenticated("authentication required")
	}
	if len(roles) == 0 || p.IsPlatformAdmin {
		return nil
	}
	if !p.HasRole(roles...) {
		return apperr.Forbidden("requires role " + strings.ToLower(strings.Join(roles, " or ")))
	}
	return nil
}

// LoaderWait is the batching window for dataloaders.
const LoaderWait = 2 * time.Millisecond

// Middleware installs per-request state built by mk (e.g. dataloaders) into the context.
func Middleware(mk func(ctx context.Context) context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(mk(r.Context())))
		})
	}
}
