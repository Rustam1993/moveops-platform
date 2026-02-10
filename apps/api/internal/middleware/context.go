package middleware

import (
	"context"
	"time"
)

type Actor struct {
	SessionID  string
	UserID     string
	TenantID   string
	Email      string
	FullName   string
	TenantSlug string
	TenantName string
	CSRFToken  string
	ExpiresAt  time.Time
}

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	actorKey     contextKey = "actor"
)

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	v, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return ""
	}
	return v
}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey, actor)
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	v, ok := ctx.Value(actorKey).(Actor)
	return v, ok
}
