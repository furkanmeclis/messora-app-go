// Package realtime connects the panel to Centrifugo: it issues connection and
// subscription tokens, decides who may read which channel, and turns business
// changes into small events published through the transactional outbox.
//
// Channels (namespaces are configured by `messora init`, see
// provision.CentrifugoConfig):
//
//	personal:#<user_id>    the user's own channel (server-side subscription)
//	system:notifications   installation-wide notices (server-side subscription)
//	inbox:all              all inbox activity     conversations.read_all
//	inbox:unassigned       unassigned activity    conversations.read or conversations.read_all
//	conversation:<id>      one conversation       conversations.read_all, or conversations.read
//	                                              when the conversation is visible to the user
//	                                              (assigned to them or to nobody)
//	campaign:all           campaign list progress campaigns.read
//	campaign:<id>          one campaign           campaigns.read
//
// Everything else is refused. Events carry IDs and states only, never message
// bodies: the client fetches the details from the API with its own
// permissions.
//
// Conversation and message events are routed by visibility (see
// ConversationChannels): always inbox:all, plus inbox:unassigned while
// nobody is assigned, or the assignee's personal channel. A user with
// conversations.read alone therefore receives events only for the
// conversations they may open; when a conversation is reassigned, the
// previous audience receives its new state once, so it can drop it.
package realtime

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Well-known channels.
const (
	InboxChannel         = "inbox:all"
	UnassignedChannel    = "inbox:unassigned"
	CampaignsChannel     = "campaign:all"
	NotificationsChannel = "system:notifications"
)

// PersonalChannel is the user-limited channel of one user.
func PersonalChannel(userID uuid.UUID) string { return "personal:#" + userID.String() }

// CampaignChannel carries the progress of one campaign.
func CampaignChannel(id uuid.UUID) string { return "campaign:" + id.String() }

// ConversationChannel carries the activity of one conversation.
func ConversationChannel(id uuid.UUID) string { return "conversation:" + id.String() }

// ErrChannelForbidden is returned for channels the caller may not read,
// including unknown and malformed ones (they are indistinguishable on
// purpose).
var ErrChannelForbidden = apperr.Forbidden("CHANNEL_FORBIDDEN", "You are not allowed to subscribe to this channel.")

// ConversationAccess reports whether p, who holds conversations.read but not
// conversations.read_all, may read conversation id. The conversations module
// provides it; without one, such users are refused.
type ConversationAccess func(ctx context.Context, p *principal.Principal, id uuid.UUID) (bool, error)

// Authorize reports whether p may subscribe to channel. Only signed-in panel
// users can subscribe; API keys never reach the realtime endpoints.
func Authorize(p *principal.Principal, channel string) error {
	return AuthorizeContext(context.Background(), p, channel, nil)
}

// AuthorizeContext is Authorize with a check of per-conversation access.
func AuthorizeContext(ctx context.Context, p *principal.Principal, channel string, access ConversationAccess) error {
	if p == nil || p.Kind != principal.KindUser || p.MFAPending {
		return ErrChannelForbidden
	}
	ns, rest, ok := strings.Cut(channel, ":")
	if !ok || rest == "" {
		return ErrChannelForbidden
	}
	allowed := false
	switch ns {
	case "personal":
		allowed = rest == "#"+p.ID.String()
	case "system":
		allowed = rest == "notifications"
	case "inbox":
		switch rest {
		case "all":
			allowed = p.Has("conversations.read_all")
		case "unassigned":
			allowed = p.Has("conversations.read") || p.Has("conversations.read_all")
		}
	case "conversation":
		if !isUUID(rest) {
			break
		}
		switch {
		case p.Has("conversations.read_all"):
			allowed = true
		case p.Has("conversations.read") && access != nil:
			ok, err := access(ctx, p, uuid.MustParse(rest))
			if err != nil {
				return err
			}
			allowed = ok
		}
	case "campaign":
		allowed = (rest == "all" || isUUID(rest)) && p.Has("campaigns.read")
	}
	if !allowed {
		return ErrChannelForbidden
	}
	return nil
}

// isUUID accepts only the canonical lower-case form, so a channel has
// exactly one spelling.
func isUUID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id.String() == s
}
