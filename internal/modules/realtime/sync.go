package realtime

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Re-sync limits: a client that missed more than this reloads instead.
const (
	SyncConversationLimit = 200
	SyncCampaignLimit     = 50
)

// ConversationScope narrows a re-sync to the conversations one channel
// announces (see ConversationChannels). The zero value is every
// conversation the caller may read.
type ConversationScope struct {
	Unassigned bool       // inbox:unassigned
	Assignee   *uuid.UUID // personal:#<id>
	ID         *uuid.UUID // conversation:<id>
}

// ConversationsSince returns up to limit conversations in scope, changed at
// or after since and visible to p, oldest change first. The conversations
// module provides it.
type ConversationsSince func(ctx context.Context, p *principal.Principal, scope ConversationScope, since time.Time, limit int) ([]db.Conversation, error)

// CampaignsSince returns up to limit campaign events (campaign id, or all
// with nil) for campaigns changed at or after since or still active. The
// campaigns module provides it.
type CampaignsSince func(ctx context.Context, id *uuid.UUID, since time.Time, limit int) ([]CampaignEvent, error)

// Syncer answers GET /realtime/sync: the current state of what a channel
// would have announced since a moment, as that channel's events.
type Syncer struct {
	Access        ConversationAccess
	Conversations ConversationsSince
	Campaigns     CampaignsSince
}

// Sync checks that p may read channel and returns snapshot events, and
// whether more changed than returned.
func (s Syncer) Sync(ctx context.Context, p *principal.Principal, channel string, since time.Time) ([]Event, bool, error) {
	if err := AuthorizeContext(ctx, p, channel, s.Access); err != nil {
		return nil, false, err
	}
	ns, rest, _ := strings.Cut(channel, ":")
	var scope ConversationScope
	switch ns {
	case "inbox":
		scope.Unassigned = rest == "unassigned"
	case "personal":
		scope.Assignee = &p.ID
	case "conversation":
		id := uuid.MustParse(rest) // checked by AuthorizeContext
		scope.ID = &id
	case "campaign":
		if s.Campaigns == nil {
			return []Event{}, false, nil
		}
		var id *uuid.UUID
		if rest != "all" {
			cid := uuid.MustParse(rest)
			id = &cid
		}
		evs, err := s.Campaigns(ctx, id, since, SyncCampaignLimit+1)
		if err != nil {
			return nil, false, err
		}
		more := len(evs) > SyncCampaignLimit
		out := make([]Event, 0, min(len(evs), SyncCampaignLimit))
		for _, e := range evs[:min(len(evs), SyncCampaignLimit)] {
			out = append(out, e)
		}
		return out, more, nil
	default: // system notices are not re-synced
		return []Event{}, false, nil
	}
	if s.Conversations == nil {
		return []Event{}, false, nil
	}
	cs, err := s.Conversations(ctx, p, scope, since, SyncConversationLimit+1)
	if err != nil {
		return nil, false, err
	}
	more := len(cs) > SyncConversationLimit
	out := make([]Event, 0, min(len(cs), SyncConversationLimit))
	for _, c := range cs[:min(len(cs), SyncConversationLimit)] {
		out = append(out, NewConversationEvent(c))
	}
	return out, more, nil
}
