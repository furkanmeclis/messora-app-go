// Package messaging queues and sends WhatsApp messages, ingests Meta's
// webhooks and downloads inbound media.
//
// Delivery guarantees:
//   - A message is sent at most once. It is claimed (pending → sending)
//     before the Graph API call; a message found in "sending" by a later
//     attempt means a worker died mid-request, so it becomes "unknown"
//     instead of being sent again.
//   - Only outcomes that prove Meta did not accept the message (connection
//     never established, rate limited, Meta-declared transient errors) are
//     retried.
//   - Ambiguous outcomes become "unknown" and are reconciled by the status
//     webhook, which carries Messora's message ID in biz_opaque_callback_data.
package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/channels"
	"github.com/furkanmeclis/messora-app-go/internal/modules/consent"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/licensegate"
)

// Task types.
const (
	TypeSend          = "whatsapp:send"
	TypeWebhook       = "whatsapp:webhook"
	TypeMediaDownload = "whatsapp:media_download"
)

// ServiceWindow is Meta's customer service window.
const ServiceWindow = 24 * time.Hour

// SendPayload is the payload of a TypeSend task. A campaign message names its
// campaign, which routes the task to the campaigns queue (queue.RouteTask);
// inbox, API and automation sends stay in the critical queue.
func SendPayload(messageID uuid.UUID, campaignID *uuid.UUID) map[string]string {
	p := map[string]string{"message_id": messageID.String()}
	if campaignID != nil && *campaignID != uuid.Nil {
		p["campaign_id"] = campaignID.String()
	}
	return p
}

// maxSendAttempts bounds retries of definitely-not-sent messages.
const maxSendAttempts = 8

// SetThrottle enables per-number pacing (nil disables it).
func (s *Service) SetThrottle(t *Throttle) { s.throttle = t }

// CampaignProgress is told, inside the webhook transaction, that a
// campaign message changed state (the campaign engine publishes counts).
type CampaignProgress func(ctx context.Context, tx pgx.Tx, q *db.Queries, campaignID uuid.UUID) error

// SetCampaignProgress registers the campaign progress hook.
func (s *Service) SetCampaignProgress(f CampaignProgress) { s.campaignProgress = f }

// Service implements messaging.
type Service struct {
	throttle         *Throttle
	campaignProgress CampaignProgress
	pool             *pgxpool.Pool
	q                *db.Queries
	ch               *channels.Service
	log              *slog.Logger
	media            *MediaStore
	maxMedia         int64
}

// New returns the service.
func New(pool *pgxpool.Pool, ch *channels.Service, media *MediaStore, log *slog.Logger) *Service {
	return &Service{pool: pool, q: db.New(pool), ch: ch, log: log, media: media, maxMedia: 100 << 20}
}

var e164 = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

// NormalizePhone turns "+90 555 111 22 33" or "905551112233" into E.164.
func NormalizePhone(s string) (string, bool) {
	var b strings.Builder
	for i, r := range strings.TrimSpace(s) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0, r == ' ', r == '-', r == '(', r == ')', r == '.':
		default:
			return "", false
		}
	}
	out := "+" + strings.TrimLeft(b.String(), "+")
	return out, e164.MatchString(out)
}

// SendInput is a message to queue.
type SendInput struct {
	PhoneNumberID *uuid.UUID // sender; nil = default number
	To            string
	Type          string
	Text          *whatsapp.Text
	Template      *whatsapp.Template
	Media         *whatsapp.Media // for image, video, audio, document, sticker
	Location      *whatsapp.Location
	Reaction      *whatsapp.Reaction
	Interactive   json.RawMessage
	ReplyTo       string // wamid
	Source        string
	CreatedBy     *uuid.UUID
}

// build validates input and produces the Graph API message and a preview.
func (in SendInput) build() (whatsapp.Message, string, error) {
	m := whatsapp.Message{Type: in.Type}
	var body string
	switch in.Type {
	case "text":
		if in.Text == nil || strings.TrimSpace(in.Text.Body) == "" {
			return m, "", apperr.Validation("text.body", "Text is required.")
		}
		if len([]rune(in.Text.Body)) > 4096 {
			return m, "", apperr.Validation("text.body", "Text is limited to 4096 characters.")
		}
		m.Text, body = in.Text, in.Text.Body
	case "template":
		if in.Template == nil || in.Template.Name == "" || in.Template.Language.Code == "" {
			return m, "", apperr.Validation("template", "Template name and language are required.")
		}
		m.Template, body = in.Template, "[template] "+in.Template.Name
	case "image", "video", "audio", "document", "sticker":
		md := in.Media
		if md == nil || (md.ID == "") == (md.Link == "") {
			return m, "", apperr.Validation("media", "Give exactly one of media.id or media.link.")
		}
		if md.Link != "" && !strings.HasPrefix(md.Link, "https://") {
			return m, "", apperr.Validation("media.link", "Media links must use https.")
		}
		if (in.Type == "audio" || in.Type == "sticker") && md.Caption != "" {
			return m, "", apperr.Validation("media.caption", "Audio and stickers cannot have a caption.")
		}
		if in.Type != "document" && md.Filename != "" {
			return m, "", apperr.Validation("media.filename", "Only documents have a file name.")
		}
		if len([]rune(md.Caption)) > 1024 {
			return m, "", apperr.Validation("media.caption", "Captions are limited to 1024 characters.")
		}
		switch in.Type {
		case "image":
			m.Image = md
		case "video":
			m.Video = md
		case "audio":
			m.Audio = md
		case "document":
			m.Document = md
		case "sticker":
			m.Sticker = md
		}
		body = md.Caption
	case "location":
		l := in.Location
		if l == nil || l.Latitude < -90 || l.Latitude > 90 || l.Longitude < -180 || l.Longitude > 180 {
			return m, "", apperr.Validation("location", "Latitude and longitude are required and must be valid.")
		}
		m.Location, body = l, strings.TrimSpace(l.Name+" "+l.Address)
	case "reaction":
		if in.Reaction == nil || in.Reaction.MessageID == "" {
			return m, "", apperr.Validation("reaction.message_id", "The message to react to is required.")
		}
		m.Reaction, body = in.Reaction, in.Reaction.Emoji
	case "interactive":
		var probe struct {
			Type string `json:"type"`
			Body struct {
				Text string `json:"text"`
			} `json:"body"`
		}
		if len(in.Interactive) == 0 || json.Unmarshal(in.Interactive, &probe) != nil ||
			(probe.Type != "button" && probe.Type != "list" && probe.Type != "cta_url") || probe.Body.Text == "" {
			return m, "", apperr.Validation("interactive", "Interactive messages need a type (button, list, cta_url) and body.text.")
		}
		m.Interactive, body = in.Interactive, probe.Body.Text
	default:
		return m, "", apperr.Validation("type", "Unsupported message type.")
	}
	if in.ReplyTo != "" {
		m.Context = &whatsapp.Context{MessageID: in.ReplyTo}
	}
	return m, body, nil
}

// Queue validates and stores a message, and schedules sending it in the
// same transaction.
func (s *Service) Queue(ctx context.Context, in SendInput) (db.Message, error) {
	var out db.Message
	err := s.tx(ctx, func(tx pgx.Tx, _ *db.Queries) error {
		var err error
		out, err = s.QueueTx(ctx, tx, in)
		return err
	})
	return out, err
}

// QueueTx is Queue inside the caller's transaction, so callers such as the
// automation runner can record "message queued" and their own progress
// atomically.
func (s *Service) QueueTx(ctx context.Context, tx pgx.Tx, in SendInput) (db.Message, error) {
	if err := licensegate.Check(ctx); err != nil {
		return db.Message{}, err
	}
	to, ok := NormalizePhone(in.To)
	if !ok {
		return db.Message{}, apperr.Validation("to", "Recipient must be a phone number in international format, e.g. +905551112233.")
	}
	msg, body, err := in.build()
	if err != nil {
		return db.Message{}, err
	}
	msg.To = strings.TrimPrefix(to, "+")

	pn, err := s.resolveNumber(ctx, in.PhoneNumberID)
	if err != nil {
		return db.Message{}, err
	}
	// The consent category: a template's own, otherwise a reply inside the
	// customer's window.
	category := consent.Service
	if in.Type == "template" {
		t, err := s.q.FindApprovedTemplate(ctx, db.FindApprovedTemplateParams{AccountID: pn.AccountID, Name: in.Template.Name, Language: in.Template.Language.Code})
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Message{}, apperr.Validation("template.name", "No template with this name and language on the sender's account; sync templates first.")
		}
		if err != nil {
			return db.Message{}, err
		}
		if t.Status != "APPROVED" {
			return db.Message{}, apperr.Validation("template.name", fmt.Sprintf("Template is %s; only approved templates can be sent.", t.Status))
		}
		category = consent.MessageCategory(t.Category)
	}
	if in.Source == "" {
		in.Source = "api"
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return db.Message{}, err
	}
	var out db.Message
	q := db.New(tx)
	err = func() error {
		c, err := q.UpsertContactByPhone(ctx, db.UpsertContactByPhoneParams{PhoneE164: to})
		if err != nil {
			return err
		}
		// The window belongs to the thread between this customer and the
		// sending number (the conversation).
		lastInbound, err := threadLastInbound(ctx, q, c.ID, pn.ID)
		if err != nil {
			return err
		}
		if in.Type != "template" && !WindowOpen(lastInbound, time.Now()) {
			return &apperr.Error{Status: 422, Code: "OUTSIDE_SERVICE_WINDOW",
				Message: "The customer has not written to this number in the last 24 hours; only approved templates can be sent."}
		}
		// Consent applies to every source alike (inbox, API, automations);
		// hard blocks cannot be bypassed.
		if err := s.checkConsent(ctx, q, c, category, lastInbound); err != nil {
			return err
		}
		out, err = q.InsertOutboundMessage(ctx, db.InsertOutboundMessageParams{ID: uuid.Must(uuid.NewV7()), PhoneNumberID: pn.ID,
			ContactID: c.ID, Type: in.Type, Body: body, Payload: payload, ReplyToWamid: nonEmpty(in.ReplyTo),
			Source: in.Source, CreatedBy: in.CreatedBy, ConsentCategory: &category})
		if err != nil {
			return err
		}
		if _, err = outbox.Write(ctx, tx, outbox.TaskTopic(TypeSend), SendPayload(out.ID, nil), 0); err != nil {
			return err
		}
		_, err = recordNew(ctx, tx, q, out)
		return err
	}()
	return out, err
}

func (s *Service) resolveNumber(ctx context.Context, id *uuid.UUID) (db.PhoneNumber, error) {
	var pn db.PhoneNumber
	var err error
	if id != nil {
		pn, err = s.q.GetPhoneNumber(ctx, *id)
	} else {
		pn, err = s.q.GetDefaultPhoneNumber(ctx)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if id == nil {
			return pn, apperr.Conflict("NO_PHONE_NUMBER", "Connect a WhatsApp account before sending messages.")
		}
		return pn, apperr.Validation("phone_number_id", "Unknown phone number.")
	}
	return pn, err
}

// Get returns a message.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (db.Message, error) {
	m, err := s.q.GetMessage(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, apperr.NotFound("Message not found.")
	}
	return m, err
}

// SendResult describes what HandleSend did (useful in tests and logs).
type SendResult struct {
	Outcome    string
	RetryAfter time.Duration
}

// ErrRetry asks the queue to try again after the delay.
type ErrRetry struct {
	After  time.Duration
	Reason string
}

func (e *ErrRetry) Error() string { return fmt.Sprintf("retry in %s: %s", e.After, e.Reason) }

// Send performs one send attempt for a queued message.
func (s *Service) Send(ctx context.Context, id uuid.UUID) error {
	// A paused campaign holds its unsent messages; a cancelled one drops them.
	if st, err := s.q.HoldIfCampaignStopped(ctx, id); err == nil {
		s.log.DebugContext(ctx, "campaign stopped; message not sent", "message_id", id, "status", st)
		return nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if s.throttle != nil {
		cur, err := s.q.GetMessage(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if cur.Status == "pending" || (cur.Status == "deferred" && due(cur)) {
			if wait := s.throttle.Wait(ctx, cur.PhoneNumberID); wait > 0 {
				// Not a failed attempt: the queue retries without counting it.
				return &ErrRetry{After: wait, Reason: "per-number rate limit"}
			}
		}
	}
	m, err := s.q.ClaimMessageForSend(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		cur, gerr := s.q.GetMessage(ctx, id)
		if errors.Is(gerr, pgx.ErrNoRows) {
			return nil // deleted meanwhile
		}
		if gerr != nil {
			return gerr
		}
		if cur.Status == "sending" {
			// A previous attempt was interrupted after claiming: the request
			// may have reached Meta. Never send twice.
			s.log.WarnContext(ctx, "message left in sending state, marking unknown", "message_id", id)
			return s.settle(ctx, id, func(q *db.Queries) error {
				return q.MarkMessageUnknown(ctx, db.MarkMessageUnknownParams{ID: id, ErrorTitle: ptr("Interrupted while sending; awaiting status from Meta.")})
			})
		}
		if cur.Status == "deferred" && cur.NextAttemptAt != nil {
			// Woken early (a duplicate task): wait for the retry time.
			return &ErrRetry{After: max(time.Until(*cur.NextAttemptAt), time.Second), Reason: "deferred by Meta (131049)"}
		}
		return nil // already handled
	}
	if err != nil {
		return err
	}
	// Consent is checked again right before Meta is called: it may have
	// changed since the message was queued. A blocked message is skipped,
	// with the reason.
	if d, err := s.recheckConsent(ctx, m); err != nil {
		_ = s.q.ReleaseMessageForRetry(ctx, db.ReleaseMessageForRetryParams{ID: id})
		return err
	} else if d.Verdict == consent.Block {
		return s.settle(ctx, id, func(q *db.Queries) error {
			return q.MarkMessageSkipped(ctx, db.MarkMessageSkippedParams{ID: id, ErrorCode: &d.Reason, ErrorTitle: ptr(consent.Explain(d.Reason))})
		})
	}

	sender, err := s.ch.SenderFor(ctx, m.PhoneNumberID)
	if err != nil {
		var ae *apperr.Error
		if errors.As(err, &ae) {
			return s.markFailed(ctx, db.MarkMessageFailedParams{ID: id, ErrorCode: &ae.Code, ErrorTitle: &ae.Message, FailureClass: ptr(whatsapp.FailurePermanent)})
		}
		// Database trouble: nothing was sent yet, put it back.
		_ = s.q.ReleaseMessageForRetry(ctx, db.ReleaseMessageForRetryParams{ID: id})
		return err
	}
	if sender.Account.Status == "error" {
		code, title := "ACCOUNT_ERROR", "The WhatsApp account needs attention: "+deref(sender.Account.LastError)
		return s.markFailed(ctx, db.MarkMessageFailedParams{ID: id, ErrorCode: &code, ErrorTitle: &title, FailureClass: ptr(whatsapp.FailurePermanent)})
	}
	var wm whatsapp.Message
	if err := json.Unmarshal(m.Payload, &wm); err != nil {
		code, title := "INVALID_PAYLOAD", "Stored message could not be decoded."
		return s.markFailed(ctx, db.MarkMessageFailedParams{ID: id, ErrorCode: &code, ErrorTitle: &title, FailureClass: ptr(whatsapp.FailurePermanent)})
	}
	wm.BizOpaqueCallbackData = m.ID.String()

	res, err := s.ch.Client().SendMessage(ctx, sender.PhoneNumber.PhoneNumberID, sender.Token, wm, int(m.Attempts)-1)
	if s.throttle != nil {
		if err == nil {
			s.throttle.Succeeded(ctx, m.PhoneNumberID)
		} else if se, ok := whatsapp.AsSendError(err); ok && se.API != nil && (se.API.Code == 130429 || se.API.HTTPStatus == 429) {
			rate := s.throttle.Backoff(ctx, m.PhoneNumberID)
			s.log.WarnContext(ctx, "Meta throughput limit; slowing down", "phone_number_id", m.PhoneNumberID, "rate_per_second", rate)
		}
	}
	if err == nil {
		// Acceptance must be recorded even if the job context is ending.
		bg := context.WithoutCancel(ctx)
		if res.Status != "" && res.Status != "accepted" {
			_ = s.q.SetMessageMetaStatus(bg, db.SetMessageMetaStatusParams{ID: id, MetaStatus: &res.Status})
		}
		return s.settle(bg, id, func(q *db.Queries) error {
			return q.MarkMessageAccepted(bg, db.MarkMessageAcceptedParams{ID: id, Wamid: &res.MessageID})
		})
	}
	se, ok := whatsapp.AsSendError(err)
	if !ok {
		se = &whatsapp.SendError{Outcome: whatsapp.Ambiguous, Err: err}
	}
	bg := context.WithoutCancel(ctx)
	code, title := se.Code(), truncate(se.Error(), 500)
	switch se.Outcome {
	case whatsapp.Retryable:
		if se.Deferred {
			// 131049: Meta limits marketing to this user for ecosystem
			// health. Retry no earlier than a day later, outside the
			// campaign's in-flight window; give up after a few days.
			if m.Attempts >= maxEcosystemAttempts {
				title = "Meta kept withholding the message for ecosystem health: " + title
				return s.markFailed(bg, db.MarkMessageFailedParams{ID: id, ErrorCode: nonEmpty(code), ErrorTitle: &title,
					FailureClass: ptr(whatsapp.FailureEcosystemLimit)})
			}
			if err := s.settle(bg, id, func(q *db.Queries) error {
				return q.DeferMessage(bg, db.DeferMessageParams{ID: id, ErrorCode: nonEmpty(code), ErrorTitle: &title,
					AfterSeconds: se.RetryAfter.Seconds()})
			}); err != nil {
				return err
			}
			return &ErrRetry{After: se.RetryAfter, Reason: title}
		}
		if m.Attempts >= maxSendAttempts {
			title = "Gave up after repeated temporary errors: " + title
			return s.markFailed(bg, db.MarkMessageFailedParams{ID: id, ErrorCode: nonEmpty(code), ErrorTitle: &title,
				FailureClass: ptr(whatsapp.FailureTemporary)})
		}
		if err := s.q.ReleaseMessageForRetry(bg, db.ReleaseMessageForRetryParams{ID: id, ErrorCode: nonEmpty(code), ErrorTitle: &title}); err != nil {
			return err
		}
		return &ErrRetry{After: se.RetryAfter, Reason: title}
	case whatsapp.Ambiguous:
		s.log.WarnContext(ctx, "send outcome unknown; waiting for Meta's status webhook", "message_id", id, "error", err)
		return s.settle(bg, id, func(q *db.Queries) error {
			return q.MarkMessageUnknown(bg, db.MarkMessageUnknownParams{ID: id, ErrorTitle: &title})
		})
	default:
		if se.AccountProblem {
			if se.API != nil {
				s.ch.AccountRefused(bg, sender.Account.ID, se.API)
			} else {
				s.ch.FlagAccountProblem(bg, sender.Account.ID, title)
			}
		}
		return s.settle(bg, id, func(q *db.Queries) error {
			if err := q.MarkMessageFailed(bg, db.MarkMessageFailedParams{ID: id, ErrorCode: nonEmpty(code), ErrorTitle: &title,
				FailureClass: ptr(se.FailureClass())}); err != nil {
				return err
			}
			return applyMetaRefusal(bg, q, m, code)
		})
	}
}

// maxEcosystemAttempts bounds 131049 deferrals (one a day).
const maxEcosystemAttempts = 3

// due reports whether a deferred message may be retried now.
func due(m db.Message) bool { return m.NextAttemptAt == nil || !m.NextAttemptAt.After(time.Now()) }

// settle records the outcome of a send attempt and, in the same
// transaction, announces the message's resulting state to the inbox.
func (s *Service) settle(ctx context.Context, id uuid.UUID, mark func(q *db.Queries) error) error {
	return s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if err := mark(q); err != nil {
			return err
		}
		m, err := q.GetMessage(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // deleted meanwhile
		}
		if err != nil {
			return err
		}
		return announceStatus(ctx, tx, q, m)
	})
}

func (s *Service) markFailed(ctx context.Context, p db.MarkMessageFailedParams) error {
	return s.settle(ctx, p.ID, func(q *db.Queries) error { return q.MarkMessageFailed(ctx, p) })
}

func (s *Service) tx(ctx context.Context, fn func(pgx.Tx, *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx, db.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func ptr[T any](v T) *T { return &v }

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func validation(field, msg string) error { return apperr.Validation(field, msg) }
