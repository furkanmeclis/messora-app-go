// Package audit records who did what. Entries are written in the same
// transaction as the change they describe and can never be updated.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Entry is one audited action.
type Entry struct {
	Action     string // "user.login", "api_key.create" …
	EntityType string
	EntityID   *uuid.UUID
	// Changes holds field → [old, new] or other context. Never put secrets
	// here; keys that look sensitive are redacted defensively.
	Changes map[string]any
	// Actor overrides the principal from context (e.g. a login, where the
	// request is anonymous but the actor is the user signing in).
	Actor *principal.Principal
}

// Record writes e using q (pass the caller's transaction).
func Record(ctx context.Context, q db.Querier, e Entry) error {
	actorType, actorID := "system", (*uuid.UUID)(nil)
	p := e.Actor
	if p == nil {
		p = principal.From(ctx)
	}
	if p != nil {
		actorType = string(p.Kind)
		id := p.ID
		actorID = &id
	}
	changes, err := json.Marshal(redact(e.Changes))
	if err != nil {
		return fmt.Errorf("audit: marshal changes: %w", err)
	}
	m := principal.Meta(ctx)
	var ip *netip.Addr
	if m.IP.IsValid() {
		a := m.IP
		ip = &a
	}
	var entityType *string
	if e.EntityType != "" {
		entityType = &e.EntityType
	}
	return q.InsertAuditLog(ctx, db.InsertAuditLogParams{
		ActorType:  actorType,
		ActorID:    actorID,
		Action:     e.Action,
		EntityType: entityType,
		EntityID:   e.EntityID,
		RequestID:  nonEmpty(m.RequestID),
		Ip:         ip,
		UserAgent:  nonEmpty(m.UserAgent),
		Changes:    changes,
	})
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

var sensitive = []string{"password", "secret", "token", "hash", "key_hash", "code"}

func redact(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		lk := strings.ToLower(k)
		hidden := false
		for _, s := range sensitive {
			if strings.Contains(lk, s) {
				hidden = true
				break
			}
		}
		if hidden {
			out[k] = "[redacted]"
		} else {
			out[k] = v
		}
	}
	return out
}
