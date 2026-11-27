package realtime

import (
	"context"
	"errors"
	"time"

	"github.com/centrifugal/gocent/v4/token"
	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// TokenTTL is how long a connection or subscription token is accepted. The
// client SDK asks for a fresh one before it expires, so a disabled user or
// a revoked permission loses access within this window.
const TokenTTL = 10 * time.Minute

// Issuer signs Centrifugo tokens with the HMAC secret shared with Centrifugo
// (client.token.hmac_secret_key and client.subscription_token.hmac_secret_key).
type Issuer struct {
	key    token.Key
	ttl    time.Duration
	now    func() time.Time
	access ConversationAccess
}

// WithConversationAccess makes the issuer grant conversation channels to
// users who may read only some conversations.
func (i *Issuer) WithConversationAccess(fn ConversationAccess) *Issuer {
	i.access = fn
	return i
}

// NewIssuer returns an issuer; the secret must not be empty.
func NewIssuer(hmacSecret string) (*Issuer, error) {
	if len(hmacSecret) < 16 {
		return nil, errors.New("realtime: CENTRIFUGO_HMAC_SECRET must be at least 16 characters")
	}
	return &Issuer{key: token.HMAC([]byte(hmacSecret)), ttl: TokenTTL, now: time.Now}, nil
}

// ConnectionUser is the Centrifugo user of p's tokens: the tenant and the
// user. A person who belongs to several tenants (WP2) has one connection
// per tenant session, and Centrifugo accepts a subscription token only on
// a connection of the same user, so a token issued in one tenant's session
// cannot be used on a connection opened for another.
func ConnectionUser(p *principal.Principal) string { return p.TenantID.String() + ":" + p.ID.String() }

// Token is a signed JWT and the moment it stops being accepted.
type Token struct {
	Token     string
	ExpiresAt time.Time
	// ChannelPrefix is the wire prefix of the session tenant's channels
	// (connection tokens only): the client subscribes to prefix + logical
	// name.
	ChannelPrefix string
}

// Connection issues a connection token for a panel user. The user's personal
// channel and the tenant's notices are subscribed server-side, so the
// client needs no subscription token for them; both are channels of the
// session's tenant. The namespace allows no client-side subscription
// without a token, so the token's grants are the whole of the access.
func (i *Issuer) Connection(p *principal.Principal) (Token, error) {
	if p == nil || p.Kind != principal.KindUser || p.TenantID == uuid.Nil {
		return Token{}, ErrChannelForbidden
	}
	now := i.now()
	exp := now.Add(i.ttl)
	tok, err := token.Connection(i.key, token.ConnectionClaims{
		User: ConnectionUser(p), ExpiresAt: exp, IssuedAt: now,
		Subs: map[string]token.SubscribeOptions{
			TenantChannel(p.TenantID, PersonalChannel(p.ID)): {},
			TenantChannel(p.TenantID, NotificationsChannel):  {},
		},
	})
	return Token{Token: tok, ExpiresAt: exp, ChannelPrefix: ChannelPrefix(p.TenantID)}, err
}

// Subscription issues a subscription token for channel after checking that
// p may read it.
func (i *Issuer) Subscription(p *principal.Principal, channel string) (Token, error) {
	return i.SubscriptionContext(context.Background(), p, channel)
}

// SubscriptionContext is Subscription with the request's context, used for
// the per-conversation access check.
func (i *Issuer) SubscriptionContext(ctx context.Context, p *principal.Principal, channel string) (Token, error) {
	if err := AuthorizeContext(ctx, p, channel, i.access); err != nil {
		return Token{}, err
	}
	now := i.now()
	exp := now.Add(i.ttl)
	tok, err := token.Subscription(i.key, token.SubscriptionClaims{
		User: ConnectionUser(p), Channel: channel, ExpiresAt: exp, IssuedAt: now,
	})
	return Token{Token: tok, ExpiresAt: exp}, err
}
