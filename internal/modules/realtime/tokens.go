package realtime

import (
	"context"
	"errors"
	"time"

	"github.com/centrifugal/gocent/v4/token"

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

// Token is a signed JWT and the moment it stops being accepted.
type Token struct {
	Token     string
	ExpiresAt time.Time
}

// Connection issues a connection token for a panel user. The user's personal
// channel and the installation notices are subscribed server-side, so the
// client needs no subscription token for them.
func (i *Issuer) Connection(p *principal.Principal) (Token, error) {
	if p == nil || p.Kind != principal.KindUser {
		return Token{}, ErrChannelForbidden
	}
	now := i.now()
	exp := now.Add(i.ttl)
	tok, err := token.Connection(i.key, token.ConnectionClaims{
		User: p.ID.String(), ExpiresAt: exp, IssuedAt: now,
		Subs: map[string]token.SubscribeOptions{
			PersonalChannel(p.ID): {},
			NotificationsChannel:  {},
		},
	})
	return Token{Token: tok, ExpiresAt: exp}, err
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
		User: p.ID.String(), Channel: channel, ExpiresAt: exp, IssuedAt: now,
	})
	return Token{Token: tok, ExpiresAt: exp}, err
}
