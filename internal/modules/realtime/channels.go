// Package realtime connects the panel to Centrifugo: it issues connection and
// subscription tokens, decides who may read which channel, and turns business
// changes into small events published through the transactional outbox.
//
// Every channel belongs to one tenant. Code publishes and reasons about the
// tenant-relative ("logical") names below; on the wire — in Centrifugo, in
// tokens and in GET /realtime/sync — each carries its tenant as a prefix:
//
//	tenant:<tenant_id>:<logical name>      e.g. tenant:0191…:inbox:all
//
// "tenant" is the only Centrifugo namespace (provision.CentrifugoConfig;
// namespace names need two characters or more, so not "t"). The publisher
// takes the prefix from the tenant that wrote the event (outbox row), and
// tokens are issued only for channels of the session's own tenant, so no
// event or subscription can cross tenants (TenantChannel, SplitChannel).
//
// Logical channels:
//
//	personal:#<user_id>    the user's own channel (server-side subscription)
//	system:notifications   the tenant's notices (server-side subscription)
//	inbox:all              all inbox activity     conversations.read_all
//	inbox:unassigned       unassigned activity    conversations.read or conversations.read_all
//	conversation:<id>      one conversation       conversations.read_all, or conversations.read
//	                                              when the conversation is visible to the user
//	                                              (assigned to them or to nobody)
//	campaign:all           campaign list progress campaigns.read
//	campaign:<id>          one campaign           campaigns.read
//	system:ops             operations center      any of OpsPermissions
//	                       section hints
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

	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Well-known channels.
const (
	InboxChannel         = "inbox:all"
	UnassignedChannel    = "inbox:unassigned"
	CampaignsChannel     = "campaign:all"
	NotificationsChannel = "system:notifications"
	// OpsChannel carries ops.updated hints for the operations center. It
	// lives in the system namespace so existing Centrifugo configurations
	// need no new namespace.
	OpsChannel = "system:ops"
)

// OpsPermissions are the permissions of the operations center's sections;
// any one of them grants OpsChannel. Its events carry a section name only,
// never IDs.
var OpsPermissions = []string{"campaigns.read", "conversations.read", "conversations.read_all", "automations.read", "settings.read"}

// HasAnyOps reports whether p may read at least one section of the
// operations center.
func HasAnyOps(p *principal.Principal) bool {
	for _, perm := range OpsPermissions {
		if p.Has(perm) {
			return true
		}
	}
	return false
}

// PersonalChannel is the user-limited channel of one user.
func PersonalChannel(userID uuid.UUID) string { return "personal:#" + userID.String() }

// CampaignChannel carries the progress of one campaign.
func CampaignChannel(id uuid.UUID) string { return "campaign:" + id.String() }

// ConversationChannel carries the activity of one conversation.
func ConversationChannel(id uuid.UUID) string { return "conversation:" + id.String() }

// Namespace is the Centrifugo namespace of every channel.
const Namespace = "tenant"

// ChannelPrefix is the wire prefix of tenant id's channels.
func ChannelPrefix(id uuid.UUID) string { return Namespace + ":" + id.String() + ":" }

// TenantChannel is the wire name of tenant id's logical channel.
func TenantChannel(id uuid.UUID, logical string) string { return ChannelPrefix(id) + logical }

// SplitChannel splits a wire channel name into its tenant and logical
// channel. The tenant must be a canonical UUID, so a channel has exactly
// one spelling.
func SplitChannel(channel string) (uuid.UUID, string, bool) {
	ns, rest, ok := strings.Cut(channel, ":")
	if !ok || ns != Namespace {
		return uuid.Nil, "", false
	}
	tid, logical, ok := strings.Cut(rest, ":")
	if !ok || logical == "" || !isUUID(tid) {
		return uuid.Nil, "", false
	}
	id := uuid.MustParse(tid)
	if id == uuid.Nil {
		return uuid.Nil, "", false
	}
	return id, logical, true
}

// ErrChannelForbidden is returned for channels the caller may not read,
// including unknown and malformed ones (they are indistinguishable on
// purpose).
var ErrChannelForbidden = apperr.Forbidden("CHANNEL_FORBIDDEN", "You are not allowed to subscribe to this channel.")

// ConversationAccess reports whether p, who holds conversations.read but not
// conversations.read_all, may read conversation id. The conversations module
// provides it; without one, such users are refused.
type ConversationAccess func(ctx context.Context, p *principal.Principal, id uuid.UUID) (bool, error)

// Authorize reports whether p may subscribe to channel (a wire name,
// tenant-prefixed). Only signed-in panel users can subscribe, and only to
// channels of their session's tenant; API keys never reach the realtime
// endpoints.
func Authorize(p *principal.Principal, channel string) error {
	return AuthorizeContext(context.Background(), p, channel, nil)
}

// AuthorizeContext is Authorize with a check of per-conversation access.
// When ctx names a tenant (the request's host), it must be the session's
// too.
func AuthorizeContext(ctx context.Context, p *principal.Principal, channel string, access ConversationAccess) error {
	_, err := authorizeLogical(ctx, p, channel, access)
	return err
}

// authorizeLogical checks channel and returns its logical name.
func authorizeLogical(ctx context.Context, p *principal.Principal, channel string, access ConversationAccess) (string, error) {
	if p == nil || p.Kind != principal.KindUser || p.MFAPending || p.TenantID == uuid.Nil {
		return "", ErrChannelForbidden
	}
	if id, ok := tenant.FromCtx(ctx); ok && id != p.TenantID {
		return "", ErrChannelForbidden
	}
	tid, logical, ok := SplitChannel(channel)
	if !ok || tid != p.TenantID {
		return "", ErrChannelForbidden
	}
	if err := authorize(ctx, p, logical, access); err != nil {
		return "", err
	}
	return logical, nil
}

// authorize checks a logical channel of p's own tenant.
func authorize(ctx context.Context, p *principal.Principal, channel string, access ConversationAccess) error {
	ns, rest, ok := strings.Cut(channel, ":")
	if !ok || rest == "" {
		return ErrChannelForbidden
	}
	allowed := false
	switch ns {
	case "personal":
		allowed = rest == "#"+p.ID.String()
	case "system":
		allowed = rest == "notifications" || (rest == "ops" && HasAnyOps(p))
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
