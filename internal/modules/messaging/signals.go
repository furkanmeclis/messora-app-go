package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/channels"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
)

// TypeUserPreference is the internal event raised when a WhatsApp user
// stops or resumes marketing messages from the business (Meta's
// user_preferences webhook). Consent handling consumes it.
const TypeUserPreference = "meta.user_preference"

// UserPreferenceEvent is the payload of TypeUserPreference.
type UserPreferenceEvent struct {
	ID         uuid.UUID `json:"id"` // meta_user_preferences row
	WaID       string    `json:"wa_id,omitempty"`
	UserID     string    `json:"user_id,omitempty"`
	PhoneE164  string    `json:"phone_e164,omitempty"`
	Category   string    `json:"category"` // marketing_messages
	Value      string    `json:"value"`    // stop | resume
	OccurredAt time.Time `json:"occurred_at"`
}

// ThroughputCeiling returns the throughput Meta reported for a number
// (messages per second; 0 = unknown). It is the send throttle's ceiling.
func (s *Service) ThroughputCeiling(ctx context.Context, pn uuid.UUID) (int, error) {
	v, err := s.q.GetPhoneNumberThroughput(ctx, pn)
	if err != nil || v == nil {
		return 0, err
	}
	return int(*v), nil
}

// highThroughputMPS is the throughput after Meta's automatic upgrade.
const highThroughputMPS = 1000

// accountChange applies the account-level webhooks: quality and limit
// changes update the phone numbers, template quality the template, and
// every alert is kept in whatsapp_account_events. user_preferences are
// stored and announced as TypeUserPreference.
func (s *Service) accountChange(ctx context.Context, tx pgx.Tx, q *db.Queries, e whatsapp.Entry, c whatsapp.Change) error {
	if c.Field == "user_preferences" {
		return s.userPreferences(ctx, tx, q, c)
	}
	acc, err := q.GetWhatsAppAccountByWABA(ctx, e.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		s.log.WarnContext(ctx, "account webhook for a WhatsApp Business Account that is not connected", "waba_id", e.ID, "field", c.Field)
		return nil
	}
	if err != nil {
		return err
	}
	at := e.At()
	record := func(kind string, pn *uuid.UUID) error {
		payload := []byte(c.Value)
		if !json.Valid(payload) {
			payload = []byte("{}")
		}
		return q.InsertWhatsAppAccountEvent(ctx, db.InsertWhatsAppAccountEventParams{AccountID: acc.ID, PhoneNumberID: pn,
			Kind: kind, Payload: payload, OccurredAt: at})
	}
	number := func(display string) (*uuid.UUID, error) {
		if display == "" {
			return nil, nil
		}
		pn, err := q.FindPhoneNumberByDisplay(ctx, db.FindPhoneNumberByDisplayParams{AccountID: acc.ID, DisplayPhoneNumber: display})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &pn.ID, nil
	}
	switch c.Field {
	case "phone_number_quality_update":
		var v whatsapp.QualityValue
		if err := json.Unmarshal(c.Value, &v); err != nil {
			return err
		}
		pn, err := number(v.DisplayPhoneNumber)
		if err != nil {
			return err
		}
		if limit := v.MessagingLimit(); limit != "" {
			// The limit belongs to the business portfolio: every number shares it.
			if err := q.SetAccountMessagingLimit(ctx, db.SetAccountMessagingLimitParams{AccountID: acc.ID, MessagingLimit: limit}); err != nil {
				return err
			}
		}
		if pn != nil && v.Event == "THROUGHPUT_UPGRADE" {
			if err := q.SetPhoneNumberSignals(ctx, db.SetPhoneNumberSignalsParams{ID: *pn, ThroughputMps: ptr(int32(highThroughputMPS))}); err != nil {
				return err
			}
		}
		if err := record("phone_number_quality_update."+strings.ToLower(v.Event), pn); err != nil {
			return err
		}
		// The event does not carry the quality rating: read the numbers'
		// current signals from Meta.
		_, err = outbox.Write(ctx, tx, outbox.TaskTopic(channels.TypeSignalSync), map[string]string{"account_id": acc.ID.String()}, 0)
		return err
	case "account_update":
		var v whatsapp.AccountUpdateValue
		if err := json.Unmarshal(c.Value, &v); err != nil {
			return err
		}
		pn, err := number(v.PhoneNumber)
		if err != nil {
			return err
		}
		switch v.Event {
		case "ACCOUNT_RESTRICTION", "ACCOUNT_VIOLATION", "DISABLED_UPDATE", "ACCOUNT_DELETED", "ACCOUNT_OFFBOARDED", "PARTNER_REMOVED":
			s.log.WarnContext(ctx, "Meta reported an account problem", "account_id", acc.ID, "event", v.Event)
		}
		return record("account_update."+strings.ToLower(v.Event), pn)
	case "message_template_quality_update":
		var v whatsapp.TemplateQualityValue
		if err := json.Unmarshal(c.Value, &v); err != nil {
			return err
		}
		if v.MessageTemplateID.String() != "" && v.NewQualityScore != "" {
			if _, err := q.SetTemplateQualityByMetaID(ctx, db.SetTemplateQualityByMetaIDParams{
				QualityScore: v.NewQualityScore, MetaTemplateID: v.MessageTemplateID.String()}); err != nil {
				return err
			}
		}
		return record("message_template_quality_update", nil)
	case "business_capability_update":
		var v whatsapp.CapabilityValue
		if err := json.Unmarshal(c.Value, &v); err != nil {
			return err
		}
		if limit := v.MessagingLimit(); limit != "" {
			if err := q.SetAccountMessagingLimit(ctx, db.SetAccountMessagingLimitParams{AccountID: acc.ID, MessagingLimit: limit}); err != nil {
				return err
			}
		}
		return record("business_capability_update", nil)
	}
	return nil
}

func (s *Service) userPreferences(ctx context.Context, tx pgx.Tx, q *db.Queries, c whatsapp.Change) error {
	var v whatsapp.UserPreferencesValue
	if err := json.Unmarshal(c.Value, &v); err != nil {
		return err
	}
	var pnID *uuid.UUID
	if v.Metadata.PhoneNumberID != "" {
		pn, err := q.GetPhoneNumberByMetaID(ctx, v.Metadata.PhoneNumberID)
		switch {
		case err == nil:
			pnID = &pn.ID
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
	}
	for _, p := range v.UserPreferences {
		if p.Category == "" || p.Value == "" {
			continue
		}
		at := p.Time()
		if at.IsZero() {
			at = time.Now().UTC()
		}
		var phone *string
		if e164, ok := NormalizePhone(p.WaID); ok && p.WaID != "" {
			phone = &e164
		}
		row, err := q.InsertMetaUserPreference(ctx, db.InsertMetaUserPreferenceParams{PhoneNumberID: pnID,
			WaID: nonEmpty(p.WaID), UserID: nonEmpty(p.UserID), PhoneE164: phone, Category: p.Category, Value: p.Value,
			Detail: truncate(p.Detail, 500), OccurredAt: at})
		if errors.Is(err, pgx.ErrNoRows) {
			continue // already recorded (webhook retried)
		}
		if err != nil {
			return err
		}
		ev := UserPreferenceEvent{ID: row.ID, WaID: p.WaID, UserID: p.UserID, Category: p.Category, Value: p.Value, OccurredAt: at}
		if phone != nil {
			ev.PhoneE164 = *phone
		}
		if _, err := outbox.Write(ctx, tx, outbox.TaskTopic(TypeUserPreference), ev, 0); err != nil {
			return err
		}
	}
	return nil
}
