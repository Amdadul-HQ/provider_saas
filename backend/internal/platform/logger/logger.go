// Package logger builds the process slog.Logger and carries request-scoped loggers in context.
package logger

import (
	"context"
	"log/slog"
	"os"
)

// New returns a JSON logger in production and a text logger otherwise.
func New(env string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	var h slog.Handler
	if env == "production" || env == "staging" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		opts.Level = slog.LevelDebug
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(h)
}

type ctxKey struct{}
type ridKey struct{}

// With stores l in ctx.
func With(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// From returns the logger stored in ctx or slog.Default().
func From(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// WithRequestID stores the request id in ctx and returns a logger annotated with it.
func WithRequestID(ctx context.Context, base *slog.Logger, id string) context.Context {
	ctx = context.WithValue(ctx, ridKey{}, id)
	return With(ctx, base.With("request_id", id))
}

// RequestID returns the request id in ctx, if any.
func RequestID(ctx context.Context) string {
	s, _ := ctx.Value(ridKey{}).(string)
	return s
}
