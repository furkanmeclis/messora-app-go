package messaging

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation/trigger"
	"github.com/furkanmeclis/messora-app-go/internal/modules/metacost"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
)

// maxWebhookBody is above Meta's documented 3 MB payload limit.
const maxWebhookBody = 4 << 20

// statusGrace is how long a status for a not-yet-known wamid is retried
// (the send response may still be in flight when the webhook arrives).
const statusGrace = 15 * time.Minute

// WebhookHandler serves GET (subscription handshake) and POST (events) on
// /webhooks/whatsapp. Events are stored before the 200 is sent and
// processed asynchronously.
func (s *Service) WebhookHandler(verifyToken string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			q := r.URL.Query()
			c, ok := whatsapp.VerifyChallenge(q.Get("hub.mode"), q.Get("hub.verify_token"), q.Get("hub.challenge"), verifyToken)
			if !ok {
				http.Error(w, "verification failed", http.StatusForbidden)
				return
			}
			// The challenge is echoed only after the verify token matched; as
			// plain text with nosniff it cannot be interpreted as markup.
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_, _ = io.WriteString(w, c) //nolint:gosec // G705: see above
		case http.MethodPost:
			s.ingest(w, r)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func (s *Service) ingest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	secrets, err := s.ch.AppSecrets(ctx)
	if err != nil {
		// Meta retries; better than dropping the event.
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	if len(secrets) == 0 {
		s.log.ErrorContext(ctx, "webhook rejected: no Meta app secret configured (META_APP_SECRET or account app secret)")
		http.Error(w, "signature verification unavailable", http.StatusUnauthorized)
		return
	}
	if !whatsapp.VerifySignature(body, r.Header.Get("X-Hub-Signature-256"), secrets...) {
		s.log.WarnContext(ctx, "webhook rejected: invalid signature")
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	var probe whatsapp.Notification
	if err := json.Unmarshal(body, &probe); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if probe.Object != "whatsapp_business_account" {
		w.WriteHeader(http.StatusOK) // not ours; acknowledge so Meta stops retrying
		return
	}
	sum := sha256.Sum256(body)
	err = s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		id, err := q.InsertWebhookEvent(ctx, db.InsertWebhookEventParams{PayloadSha256: sum[:], Body: body})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // Meta retried an event we already have
		}
		if err != nil {
			return err
		}
		_, err = outbox.Write(ctx, tx, outbox.TaskTopic(TypeWebhook), map[string]string{"event_id": id.String()}, 0)
		return err
	})
	if err != nil {
		s.log.ErrorContext(ctx, "store webhook", "error", err)
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

var statusRank = map[string]int16{"sent": 3, "delivered": 4, "read": 5, "played": 5, "failed": 6}

// ProcessWebhook applies a stored event. It is idempotent: inbound messages
// are unique by wamid, status changes are monotonic.
func (s *Service) ProcessWebhook(ctx context.Context, id uuid.UUID, attempt, maxAttempts int) error {
	ev, err := s.q.GetWebhookEvent(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && ev.Status != "pending") {
		return nil
	}
	if err != nil {
		return err
	}
	var n whatsapp.Notification
	if err := json.Unmarshal(ev.Body, &n); err != nil {
		return s.q.MarkWebhookAttempt(ctx, db.MarkWebhookAttemptParams{ID: id, LastError: ptr("invalid json: " + err.Error()), Final: true})
	}
	var unmatched int
	var media []uuid.UUID
	err = s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		unmatched, media = 0, nil
		for _, e := range n.Entry {
			for _, c := range e.Changes {
				u, m, err := s.applyChange(ctx, tx, q, e, c)
				if err != nil {
					return fmt.Errorf("%s: %w", c.Field, err)
				}
				unmatched += u
				media = append(media, m...)
			}
		}
		for _, m := range media {
			if _, err := outbox.Write(ctx, tx, outbox.TaskTopic(TypeMediaDownload), map[string]string{"media_id": m.String()}, 0); err != nil {
				return err
			}
		}
		if unmatched > 0 && time.Since(ev.ReceivedAt) < statusGrace {
			return nil // committed what we could; the caller retries the rest
		}
		return q.MarkWebhookProcessed(ctx, id)
	})
	if err != nil {
		final := attempt+1 >= maxAttempts
		_ = s.q.MarkWebhookAttempt(context.WithoutCancel(ctx), db.MarkWebhookAttemptParams{ID: id, LastError: ptr(truncate(err.Error(), 500)), Final: final})
		return err
	}
	if unmatched > 0 && time.Since(ev.ReceivedAt) < statusGrace {
		return &ErrRetry{After: 30 * time.Second, Reason: fmt.Sprintf("%d status update(s) for unknown messages", unmatched)}
	}
	if unmatched > 0 {
		s.log.WarnContext(ctx, "dropped status updates for messages Messora did not send", "event_id", id, "count", unmatched)
	}
	return nil
}

func (s *Service) applyChange(ctx context.Context, tx pgx.Tx, q *db.Queries, e whatsapp.Entry, c whatsapp.Change) (unmatched int, media []uuid.UUID, err error) {
	switch c.Field {
	case "messages":
		var v whatsapp.MessagesValue
		if err := json.Unmarshal(c.Value, &v); err != nil {
			return 0, nil, err
		}
		pn, err := q.GetPhoneNumberByMetaID(ctx, v.Metadata.PhoneNumberID)
		if errors.Is(err, pgx.ErrNoRows) {
			s.log.WarnContext(ctx, "webhook for a phone number that is not connected", "phone_number_id", v.Metadata.PhoneNumberID)
			return 0, nil, nil
		}
		if err != nil {
			return 0, nil, err
		}
		names := map[string]string{}
		for _, ct := range v.Contacts {
			names[ct.WaID] = ct.Profile.Name
		}
		for i := range v.Messages {
			mid, err := s.inbound(ctx, tx, q, pn, &v.Messages[i], names)
			if err != nil {
				return 0, nil, err
			}
			if mid != uuid.Nil {
				media = append(media, mid)
			}
		}
		for i := range v.Statuses {
			ok, err := s.status(ctx, tx, q, &v.Statuses[i])
			if err != nil {
				return 0, nil, err
			}
			if !ok {
				unmatched++
			}
		}
	case "message_template_status_update":
		var v whatsapp.TemplateStatusValue
		if err := json.Unmarshal(c.Value, &v); err != nil {
			return 0, nil, err
		}
		_, err := q.UpdateTemplateStatusByMetaID(ctx, db.UpdateTemplateStatusByMetaIDParams{
			Status: v.Event, Reason: v.Reason, MetaTemplateID: v.MessageTemplateID.String()})
		return 0, nil, err
	case "phone_number_quality_update", "account_update", "message_template_quality_update",
		"business_capability_update", "user_preferences":
		return 0, nil, s.accountChange(ctx, tx, q, e, c)
	}
	return unmatched, media, nil
}

// inbound stores a customer message and announces it to the inbox; it
// returns the media row to download.
func (s *Service) inbound(ctx context.Context, tx pgx.Tx, q *db.Queries, pn db.PhoneNumber, m *whatsapp.InboundMessage, names map[string]string) (uuid.UUID, error) {
	phone, ok := NormalizePhone(m.From)
	if !ok || m.ID == "" {
		s.log.WarnContext(ctx, "inbound message without a usable sender", "wamid", m.ID)
		return uuid.Nil, nil
	}
	c, err := q.UpsertInboundContact(ctx, db.UpsertInboundContactParams{PhoneE164: phone, WaID: &m.From, ProfileName: names[m.From]})
	if err != nil {
		return uuid.Nil, err
	}
	raw, _ := json.Marshal(m)
	at := m.Time()
	if at.IsZero() {
		at = time.Now()
	}
	var reply *string
	if m.Context != nil && m.Context.ID != "" {
		reply = &m.Context.ID
	}
	known, err := q.ContactHasMessages(ctx, c.ID)
	if err != nil {
		return uuid.Nil, err
	}
	row, err := q.InsertInboundMessage(ctx, db.InsertInboundMessageParams{PhoneNumberID: pn.ID, ContactID: c.ID, Wamid: &m.ID,
		Type: m.Type, Body: m.Body(), Payload: raw, ReplyToWamid: reply, OccurredAt: at})
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil // duplicate delivery
	}
	if err != nil {
		return uuid.Nil, err
	}
	conv, err := recordNew(ctx, tx, q, row)
	if err != nil {
		return uuid.Nil, err
	}
	ev := trigger.Event{Type: trigger.MessageReceived, Key: "msg:" + m.ID, ContactID: c.ID, ConversationID: &conv.ID,
		PhoneNumberID: &pn.ID, MessageID: &row.ID, MessageType: m.Type, Body: row.Body}
	if err := trigger.Emit(ctx, tx, q, ev); err != nil {
		return uuid.Nil, err
	}
	if !known {
		ev.Type, ev.Key = trigger.NewContact, "contact:"+c.ID.String()
		if err := trigger.Emit(ctx, tx, q, ev); err != nil {
			return uuid.Nil, err
		}
	}
	if opensWindow(m.Type) {
		if err := q.TouchContactInbound(ctx, db.TouchContactInboundParams{ID: c.ID, At: &at}); err != nil {
			return uuid.Nil, err
		}
	}
	if err := s.inboundConsent(ctx, tx, q, pn, c, conv, row); err != nil {
		return uuid.Nil, fmt.Errorf("consent: %w", err)
	}
	if s.inboundHook != nil {
		if err := s.inboundHook(ctx, tx, q, conv, row); err != nil {
			return uuid.Nil, fmt.Errorf("inbound hook: %w", err)
		}
	}
	if md := m.Media(); md != nil && md.ID != "" {
		f, err := q.InsertMediaFile(ctx, db.InsertMediaFileParams{MessageID: &row.ID, MetaMediaID: &md.ID,
			MimeType: md.MimeType, Filename: md.Filename, Sha256: md.SHA256})
		if err != nil {
			return uuid.Nil, err
		}
		return f.ID, nil
	}
	return uuid.Nil, nil
}

// status applies a delivery update and announces an actual change; false
// means the message is unknown.
func (s *Service) status(ctx context.Context, tx pgx.Tx, q *db.Queries, st *whatsapp.Status) (bool, error) {
	rank, ok := statusRank[st.Status]
	if !ok {
		return true, nil // unknown status kinds are ignored
	}
	status := st.Status
	if status == "played" {
		status = "read"
	}
	m, err := q.GetMessageByWAMID(ctx, &st.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		// The send response may have been lost: our ID travels in the
		// callback data.
		id, perr := uuid.Parse(st.BizOpaqueCallbackData)
		if perr != nil {
			return false, nil
		}
		m, err = q.GetMessage(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
	}
	if err != nil {
		return false, err
	}
	at := st.Time()
	if at.IsZero() {
		at = time.Now()
	}
	p := db.ApplyMessageStatusParams{ID: m.ID, Wamid: &st.ID, Status: status, Rank: rank, At: &at}
	if status == "failed" && len(st.Errors) > 0 {
		code := fmt.Sprint(st.Errors[0].Code)
		title := st.Errors[0].Title
		if d := st.Errors[0].ErrorData.Details; d != "" {
			title += ": " + d
		}
		p.ErrorCode, p.ErrorTitle = &code, ptr(truncate(title, 500))
		p.FailureClass = ptr(whatsapp.FailureClassOf(st.Errors[0].Code))
	} else if status == "failed" {
		p.FailureClass = ptr(whatsapp.FailureRejectedByMeta)
	}
	updated, err := q.ApplyMessageStatus(ctx, p)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil // inbound message id; nothing to do
	}
	if err != nil {
		return false, err
	}
	if p := st.Pricing; p != nil {
		c, err := q.GetContact(ctx, updated.ContactID)
		if err != nil {
			return false, err
		}
		if err := metacost.PriceMessage(ctx, q, updated, metacost.Pricing{Type: p.Type, Category: p.Category,
			Model: p.PricingModel, Billable: p.Billable}, c.PhoneE164); err != nil {
			return false, fmt.Errorf("price message: %w", err)
		}
	}
	if updated.Status != m.Status {
		if updated.Status == "failed" && p.ErrorCode != nil {
			// 131050 and 131026 usually arrive here, after Meta accepted
			// the request.
			if err := applyMetaRefusal(ctx, q, updated, *p.ErrorCode); err != nil {
				return false, err
			}
		}
		if updated.Status == "failed" && m.CampaignID == nil {
			if err := recordFailure(ctx, tx, q, updated); err != nil {
				return false, err
			}
		}
		if err := announceStatus(ctx, tx, q, updated); err != nil {
			return false, err
		}
		if m.CampaignID != nil && s.campaignWithheld != nil && updated.Status == "failed" &&
			updated.FailureClass != nil && *updated.FailureClass == whatsapp.FailureEcosystemLimit {
			if err := s.campaignWithheld(ctx, q, updated); err != nil {
				return false, err
			}
		}
		if m.CampaignID != nil && s.campaignProgress != nil {
			if err := s.campaignProgress(ctx, tx, q, *m.CampaignID); err != nil {
				return false, err
			}
		}
		// Statuses of messages an automation sent never trigger automations.
		if m.Source != "automation" && (updated.Status == "delivered" || updated.Status == "read" || updated.Status == "failed") {
			if err := trigger.Emit(ctx, tx, q, trigger.Event{Type: trigger.MessageStatus, Key: "status:" + m.ID.String() + ":" + updated.Status,
				ContactID: m.ContactID, PhoneNumberID: &m.PhoneNumberID, MessageID: &m.ID, MessageType: m.Type,
				Status: updated.Status, CampaignID: m.CampaignID}); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}
