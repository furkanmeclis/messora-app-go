package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/consent"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
)

// checkConsent applies the consent rules to a message about to be queued.
func (s *Service) checkConsent(ctx context.Context, q *db.Queries, c db.Contact, category string, lastInbound *time.Time) error {
	set, err := consent.CachedSettings(ctx, q)
	if err != nil {
		return err
	}
	st := consent.StateOf(c)
	st.LastInboundAt = lastInbound
	return consent.Decide(st, category, set.Policy()).Err()
}

// categoryOf returns the consent category a message was queued with. Rows
// queued before categories were stored derive it: a template from its
// record (marketing when gone), anything else is a reply.
func categoryOf(ctx context.Context, q *db.Queries, m db.Message) (string, error) {
	if m.ConsentCategory != nil && *m.ConsentCategory != "" {
		return *m.ConsentCategory, nil
	}
	if m.Type != "template" {
		return consent.Service, nil
	}
	var wm whatsapp.Message
	if json.Unmarshal(m.Payload, &wm) != nil || wm.Template == nil {
		return consent.Marketing, nil
	}
	pn, err := q.GetPhoneNumber(ctx, m.PhoneNumberID)
	if err != nil {
		return consent.Marketing, nil //nolint:nilerr // the number is gone: strictest category
	}
	t, err := q.FindApprovedTemplate(ctx, db.FindApprovedTemplateParams{AccountID: pn.AccountID, Name: wm.Template.Name, Language: wm.Template.Language.Code})
	if errors.Is(err, pgx.ErrNoRows) {
		return consent.Marketing, nil
	}
	if err != nil {
		return "", err
	}
	return consent.MessageCategory(t.Category), nil
}

// recheckConsent is the last check right before Meta is called: a contact
// may have opted out after the message was queued (a campaign materialised
// hours ago, a delayed automation). It returns the blocking decision, or a
// zero decision when the message may go.
func (s *Service) recheckConsent(ctx context.Context, m db.Message) (consent.Decision, error) {
	c, err := s.q.GetContact(ctx, m.ContactID)
	if err != nil {
		return consent.Decision{}, err
	}
	category, err := categoryOf(ctx, s.q, m)
	if err != nil {
		return consent.Decision{}, err
	}
	set, err := consent.CachedSettings(ctx, s.q)
	if err != nil {
		return consent.Decision{}, err
	}
	st := consent.StateOf(c)
	if category == consent.Service {
		if st.LastInboundAt, err = threadLastInbound(ctx, s.q, m.ContactID, m.PhoneNumberID); err != nil {
			return consent.Decision{}, err
		}
	}
	d := consent.Decide(st, category, set.Policy())
	if d.Allowed() {
		return consent.Decision{}, nil
	}
	return d, nil
}

// applyMetaRefusal records what a Meta error says about the recipient, in
// the transaction that marks the message failed: 131050 (stopped marketing
// in WhatsApp) for a marketing message, 131026 (undeliverable).
func applyMetaRefusal(ctx context.Context, q *db.Queries, m db.Message, code string) error {
	n, err := strconv.Atoi(code)
	if err != nil {
		return nil //nolint:nilerr // not a Meta code
	}
	switch n {
	case whatsapp.CodeMarketingStopped:
		category, err := categoryOf(ctx, q, m)
		if err != nil {
			return err
		}
		if category != consent.Marketing {
			return nil
		}
		_, err = consent.SetMetaMarketingStopped(ctx, q, m.ContactID, true, time.Now(), consent.SourceMetaError,
			"Meta refused a marketing message (131050).")
		return err
	case whatsapp.CodeUndeliverable:
		_, err := consent.SetUndeliverable(ctx, q, m.ContactID, code)
		return err
	}
	return nil
}
