package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	gen "github.com/moveops-platform/apps/api/internal/gen/db"
)

type Logger struct {
	q *gen.Queries
}

func NewLogger(q *gen.Queries) *Logger {
	return &Logger{q: q}
}

type Entry struct {
	TenantID   uuid.UUID
	UserID     *uuid.UUID
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	RequestID  string
	Metadata   map[string]any
}

func (l *Logger) Log(ctx context.Context, entry Entry) error {
	metadata := []byte("{}")
	if len(entry.Metadata) > 0 {
		encoded, err := json.Marshal(entry.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		metadata = encoded
	}

	params := gen.InsertAuditLogParams{
		TenantID:   entry.TenantID,
		Action:     entry.Action,
		EntityType: entry.EntityType,
		Metadata:   metadata,
	}
	if entry.UserID != nil {
		params.UserID = entry.UserID
	}
	if entry.EntityID != nil {
		params.EntityID = entry.EntityID
	}
	if entry.RequestID != "" {
		params.RequestID = &entry.RequestID
	}

	if err := l.q.InsertAuditLog(ctx, params); err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}
