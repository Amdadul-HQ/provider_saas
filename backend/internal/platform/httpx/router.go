// Package httpx builds the chi router: request id, logging, recover, CORS, rate limit,
// /healthz, /graphql and /playground (dev only).
package httpx

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"provider_saas/internal/platform/logger"
)

type Deps struct {
	Logger       *slog.Logger
	Pool         *pgxpool.Pool
	GraphQL      http.Handler
	Auth         func(http.Handler) http.Handler // principal middleware
	PerRequest   func(http.Handler) http.Handler // dataloaders etc.
	CORSOrigins  []string
	Dev          bool
	RateLimitRPS float64
	RateBurst    int
	// TrustedProxyCIDRs: only these peers may set the client IP via X-Forwarded-For.
	TrustedProxyCIDRs []string
	// Mount lets other modules register REST routes (webhooks).
	Mount func(r chi.Router)
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(TrustedRealIP(d.TrustedProxyCIDRs))
	r.Use(SecurityHeaders)
	r.Use(requestLogger(d.Logger))
	r.Use(recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   d.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-Id"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: true,
		MaxAge:           600,
	}))
	if d.RateLimitRPS > 0 {
		rl := NewRateLimiter(d.RateLimitRPS, d.RateBurst) // bounded: MaxBuckets keys
		go func() {
			for range time.Tick(5 * time.Minute) {
				rl.Cleanup(10 * time.Minute)
			}
		}()
		r.Use(rl.Middleware)
	}

	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		status, code := "ok", http.StatusOK
		if d.Pool != nil {
			if err := d.Pool.Ping(ctx); err != nil {
				status, code = "db_unavailable", http.StatusServiceUnavailable
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequestSize(2 << 20))
		if d.Auth != nil {
			r.Use(d.Auth)
		}
		if d.PerRequest != nil {
			r.Use(d.PerRequest)
		}
		r.Handle("/graphql", d.GraphQL)
	})
	if d.Dev {
		r.Handle("/playground", playground.Handler("provider_saas", "/graphql"))
	}
	if d.Mount != nil {
		d.Mount(r)
	}
	return r
}

func requestLogger(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rid := middleware.GetReqID(r.Context())
			w.Header().Set("X-Request-Id", rid)
			ctx := logger.WithRequestID(r.Context(), base, rid)
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r.WithContext(ctx))
			if r.URL.Path == "/healthz" {
				return
			}
			logger.From(ctx).Info("http", "method", r.Method, "path", r.URL.Path, "status", ww.Status(),
				"bytes", ww.BytesWritten(), "dur_ms", time.Since(start).Milliseconds())
		})
	}
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler {
					panic(p)
				}
				logger.From(r.Context()).Error("http panic", "panic", p, "stack", string(debug.Stack()))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// NewServer returns an http.Server with sane timeouts.
func NewServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}
