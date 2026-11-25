// Package campaigns sends approved templates to an audience at the
// throughput Meta allows. See docs/ARCHITECTURE.md §7.
//
//	start → preparing ─materialize→ running ─dispatch ticks→ completed
//	                                  ↕ pause/resume          cancel → cancelled
//
// Recipients are materialised once (resumably) into campaign_recipients.
// The dispatcher turns at most a window of them into messages at a time, so
// the queue never holds the whole audience. Sending itself is the messaging
// module's at-most-once path.
package campaigns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/contacts"
	"github.com/furkanmeclis/messora-app-go/internal/modules/contacts/segment"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/modules/webhooks"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/licensegate"
)

// Task types.
const (
	TypeMaterialize = "campaign:materialize"
	TypeDispatch    = "campaign:dispatch"
	TypeReaper      = "campaign:reaper"
	TypeStartDue    = "campaign:start_due"
)

// Tunables.
const (
	materializePage = 1000
	tickInterval    = time.Second
	// inflightSeconds is how many seconds of its share a campaign keeps in
	// flight (queued or being sent): enough to absorb send latency and
	// retries, small enough that the queue never holds the audience.
	inflightSeconds = 10
	// minInflight is the in-flight floor of a campaign with a tiny share.
	minInflight = 20
)

// Service is the campaign engine.
type Service struct {
	pool     *pgxpool.Pool
	q        *db.Queries
	contacts *contacts.Service
	log      *slog.Logger
	// sched is the per-number send scheduler (nil: Meta's default
	// throughput, no shares stored).
	sched *messaging.Throttle
	// TickInterval is the dispatcher period (tests shorten it).
	TickInterval time.Duration
}

// New returns the service.
func New(pool *pgxpool.Pool, c *contacts.Service, log *slog.Logger) *Service {
	return &Service{pool: pool, q: db.New(pool), contacts: c, log: log, TickInterval: tickInterval}
}

// SetScheduler makes the dispatcher size campaigns by the number's
// capacity and publish their fair shares to the send path.
func (s *Service) SetScheduler(t *messaging.Throttle) { s.sched = t }

// Audience selects recipients.
type Audience struct {
	SegmentID *uuid.UUID `json:"segment_id,omitempty"`
	GroupID   *uuid.UUID `json:"group_id,omitempty"`
	TagID     *uuid.UUID `json:"tag_id,omitempty"`
	All       bool       `json:"all,omitempty"`
	// PreviousCampaign targets the customers an earlier campaign did not
	// reach, by why (audience.go).
	PreviousCampaign *PreviousCampaign `json:"previous_campaign,omitempty"`
}

func (a Audience) validate() error {
	n := 0
	for _, set := range []bool{a.SegmentID != nil, a.GroupID != nil, a.TagID != nil, a.All, a.PreviousCampaign != nil} {
		if set {
			n++
		}
	}
	if n != 1 {
		return apperr.Validation("audience", "Choose exactly one audience: a segment, a group, a tag, all contacts or a previous campaign's unreached customers.")
	}
	if a.PreviousCampaign != nil {
		return a.PreviousCampaign.validate()
	}
	return nil
}

// Params maps template components to values. Each value may contain
// placeholders: {{name}}, {{phone}}, {{email}}, {{var1}}…{{var5}},
// {{attr:<key>}}; anything else is literal. Named-parameter templates use
// objects ({"first_name": "{{name}}"}), positional ones arrays.
type Params struct {
	Header json.RawMessage `json:"header,omitempty"`
	Body   json.RawMessage `json:"body,omitempty"`
	// Buttons[i] fills the dynamic suffix of URL button i.
	Buttons []string `json:"buttons,omitempty"`
	// Fallback replaces an empty placeholder; without it the recipient is
	// skipped (Meta rejects empty parameters).
	Fallback map[string]string `json:"fallback,omitempty"`
	// HeaderMedia fills an image, video or document header.
	HeaderMedia *HeaderMedia `json:"header_media,omitempty"`
	Extras
}

// Extras fill the components beyond variables and header media: location
// headers, copy code, limited-time offer, carousel, one-time password and
// Flow buttons. Automations embed them too.
type Extras struct {
	// Location fills a location header; name and address may contain
	// placeholders.
	Location *Location `json:"location,omitempty"`
	// CouponCode fills the copy code button (placeholders allowed).
	CouponCode string `json:"coupon_code,omitempty"`
	// ExpirationTimeMS is when a limited-time offer ends (Unix ms). A
	// campaign linked to an offer takes it from the offer.
	ExpirationTimeMS *int64 `json:"expiration_time_ms,omitempty"`
	// Cards fill a carousel's cards, in order.
	Cards []CardParams `json:"cards,omitempty"`
	// OTPCode is the one-time password of an authentication template.
	OTPCode string `json:"otp_code,omitempty"`
	// Flow fills a Flow button.
	Flow *FlowParams `json:"flow,omitempty"`
}

// Location is the place of a location header, in decimal degrees.
type Location struct {
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
	Name      string `json:"name,omitempty"`
	Address   string `json:"address,omitempty"`
}

// CardParams fills one carousel card: its image or video, body variables
// and the dynamic suffixes of its URL buttons (by button position).
type CardParams struct {
	HeaderMedia *HeaderMedia    `json:"header_media,omitempty"`
	Body        json.RawMessage `json:"body,omitempty"`
	Buttons     []string        `json:"buttons,omitempty"`
}

// FlowParams opens a Flow button's Flow: the token identifies the session
// to the business's Flow endpoint (Meta's default "unused" when empty), the
// data pre-fills its first screen.
type FlowParams struct {
	Token      string         `json:"flow_token,omitempty"`
	ActionData map[string]any `json:"flow_action_data,omitempty"`
}

// HeaderMedia is the media of a template header: an uploaded media ID or a
// public https link.
type HeaderMedia struct {
	Type     string `json:"type"` // image | video | document
	ID       string `json:"id,omitempty"`
	Link     string `json:"link,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// Input creates or edits a campaign.
type Input struct {
	Name             *string
	PhoneNumberID    *uuid.UUID
	TemplateName     *string
	TemplateLanguage *string
	Params           *Params
	Audience         *Audience
	WindowSize       *int32
	// OfferID links the campaign to an offer; ClearOffer (edits) unlinks it.
	OfferID    *uuid.UUID
	ClearOffer bool
	// Priority is high, normal or low (the campaign's weight in the
	// number's send scheduler); MaxMPS caps its rate (ClearMaxMPS removes
	// the cap).
	Priority    *string
	MaxMPS      *float64
	ClearMaxMPS bool
}

// Create stores a draft.
func (s *Service) Create(ctx context.Context, in Input, actor *uuid.UUID) (db.Campaign, error) {
	if in.Name == nil || strings.TrimSpace(*in.Name) == "" {
		return db.Campaign{}, apperr.Validation("name", "Name is required.")
	}
	if in.PhoneNumberID == nil {
		pn, err := s.q.GetDefaultPhoneNumber(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Campaign{}, apperr.Conflict("NO_PHONE_NUMBER", "Connect a WhatsApp account before creating campaigns.")
		}
		if err != nil {
			return db.Campaign{}, err
		}
		in.PhoneNumberID = &pn.ID
	}
	if in.TemplateName == nil || in.TemplateLanguage == nil {
		return db.Campaign{}, apperr.Validation("template", "Template name and language are required.")
	}
	if in.Audience == nil {
		return db.Campaign{}, apperr.Validation("audience", "Audience is required.")
	}
	if err := s.validate(ctx, in); err != nil {
		return db.Campaign{}, err
	}
	params := Params{}
	if in.Params != nil {
		params = *in.Params
	}
	pj, _ := json.Marshal(params)
	aj, _ := json.Marshal(in.Audience)
	var window int32
	if in.WindowSize != nil {
		window = *in.WindowSize
	}
	priority := "normal"
	if in.Priority != nil {
		priority = *in.Priority
	}
	var c db.Campaign
	err := s.tx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		var err error
		c, err = q.InsertCampaign(ctx, db.InsertCampaignParams{Name: strings.TrimSpace(*in.Name), PhoneNumberID: *in.PhoneNumberID,
			TemplateName: *in.TemplateName, TemplateLanguage: *in.TemplateLanguage, TemplateParams: pj, Audience: aj,
			WindowSize: window, CreatedBy: actor, OfferID: in.OfferID, Priority: priority, MaxMps: in.MaxMPS})
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "campaign.create", EntityType: "campaign", EntityID: &c.ID})
	})
	return c, err
}

// validate checks references in the parts of in that are set.
func (s *Service) validate(ctx context.Context, in Input) error {
	if in.Audience != nil {
		if err := in.Audience.validate(); err != nil {
			return err
		}
		if in.Audience.SegmentID != nil {
			if _, err := s.q.GetSegment(ctx, *in.Audience.SegmentID); errors.Is(err, pgx.ErrNoRows) {
				return apperr.Validation("audience.segment_id", "Unknown segment.")
			}
		}
		if in.Audience.GroupID != nil {
			if _, err := s.q.GetContactGroup(ctx, *in.Audience.GroupID); errors.Is(err, pgx.ErrNoRows) {
				return apperr.Validation("audience.group_id", "Unknown group.")
			}
		}
		if in.Audience.TagID != nil {
			if _, err := s.q.GetTag(ctx, *in.Audience.TagID); errors.Is(err, pgx.ErrNoRows) {
				return apperr.Validation("audience.tag_id", "Unknown tag.")
			}
		}
	}
	if in.Audience != nil && in.Audience.PreviousCampaign != nil {
		if err := s.validatePrevious(ctx, *in.Audience.PreviousCampaign); err != nil {
			return err
		}
	}
	if err := validateSettings(in.Priority, in.MaxMPS); err != nil {
		return err
	}
	if in.PhoneNumberID != nil {
		if _, err := s.q.GetPhoneNumber(ctx, *in.PhoneNumberID); errors.Is(err, pgx.ErrNoRows) {
			return apperr.Validation("phone_number_id", "Unknown phone number.")
		}
	}
	if in.Params != nil {
		for _, raw := range []json.RawMessage{in.Params.Header, in.Params.Body} {
			if len(raw) == 0 {
				continue
			}
			var list []string
			var named map[string]string
			if json.Unmarshal(raw, &list) != nil && json.Unmarshal(raw, &named) != nil {
				return apperr.Validation("template_params", "Header and body parameters are a list of strings or an object of strings.")
			}
		}
	}
	if in.Params != nil && in.Params.HeaderMedia != nil {
		if err := checkHeaderMedia("template_params.header_media", in.Params.HeaderMedia); err != nil {
			return err
		}
		if len(in.Params.Header) > 0 {
			return apperr.Validation("template_params.header", "A header is either text variables or media, not both.")
		}
	}
	if in.Params != nil {
		for k, card := range in.Params.Cards {
			if card.HeaderMedia == nil {
				continue
			}
			if err := checkHeaderMedia(fmt.Sprintf("template_params.cards.%d.header_media", k), card.HeaderMedia); err != nil {
				return err
			}
		}
	}
	if in.OfferID != nil {
		o, err := s.q.GetOffer(ctx, *in.OfferID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.Validation("offer_id", "Unknown offer.")
		}
		if err != nil {
			return err
		}
		if offerEnded(o, time.Now()) {
			return apperr.Validation("offer_id", fmt.Sprintf("The offer %q has ended; choose a running or upcoming offer.", o.Title))
		}
	}
	if in.WindowSize != nil && (*in.WindowSize < 0 || *in.WindowSize > 10000) {
		return apperr.Validation("window_size", "Window must be between 1 and 10000 (0 = automatic).")
	}
	return nil
}

func checkHeaderMedia(field string, hm *HeaderMedia) error {
	if hm.Type != "image" && hm.Type != "video" && hm.Type != "document" {
		return apperr.Validation(field+".type", "Header media is an image, video or document.")
	}
	if (hm.ID == "") == (hm.Link == "") {
		return apperr.Validation(field, "Give exactly one of id or link.")
	}
	if hm.Link != "" && !strings.HasPrefix(hm.Link, "https://") {
		return apperr.Validation(field+".link", "Media links must use https.")
	}
	return nil
}

// Update edits a draft or scheduled campaign.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in Input) (db.Campaign, error) {
	if err := s.validate(ctx, in); err != nil {
		return db.Campaign{}, err
	}
	p := db.UpdateCampaignDraftParams{ID: id, Name: in.Name, PhoneNumberID: in.PhoneNumberID, TemplateName: in.TemplateName,
		TemplateLanguage: in.TemplateLanguage, WindowSize: in.WindowSize, Priority: in.Priority, MaxMps: in.MaxMPS, ClearMaxMps: in.ClearMaxMPS}
	if in.Params != nil {
		p.TemplateParams, _ = json.Marshal(in.Params)
	}
	if in.Audience != nil {
		p.Audience, _ = json.Marshal(in.Audience)
	}
	var c db.Campaign
	err := s.tx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		var err error
		c, err = q.UpdateCampaignDraft(ctx, p)
		if errors.Is(err, pgx.ErrNoRows) {
			if _, gerr := q.GetCampaign(ctx, id); errors.Is(gerr, pgx.ErrNoRows) {
				return apperr.NotFound("Campaign not found.")
			}
			return apperr.Conflict("CAMPAIGN_STARTED", "Only draft or scheduled campaigns can be edited.")
		}
		if err != nil || (in.OfferID == nil && !in.ClearOffer) {
			return err
		}
		if err := q.SetCampaignOffer(ctx, db.SetCampaignOfferParams{ID: id, OfferID: in.OfferID}); err != nil {
			return err
		}
		c, err = q.GetCampaign(ctx, id)
		return err
	})
	return c, err
}

// Delete removes a campaign that is not in progress.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := s.q.DeleteCampaign(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		if _, gerr := s.q.GetCampaign(ctx, id); errors.Is(gerr, pgx.ErrNoRows) {
			return apperr.NotFound("Campaign not found.")
		}
		return apperr.Conflict("CAMPAIGN_ACTIVE", "Pause or cancel the campaign before deleting it.")
	}
	return nil
}

// transition changes status and records an event, publishing progress.
func (s *Service) transition(ctx context.Context, q *db.Queries, tx pgx.Tx, id uuid.UUID, to string, from []string,
	kind string, detail map[string]any, actor *uuid.UUID, scheduledAt *time.Time) (db.Campaign, error) {
	c, err := q.SetCampaignStatus(ctx, db.SetCampaignStatusParams{ID: id, Status: to, FromStates: from, ScheduledAt: scheduledAt,
		LastError: lastError(detail)})
	if err == nil {
		c, err = s.recordPauseChange(ctx, q, c, kind, detail, actor)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		cur, gerr := q.GetCampaign(ctx, id)
		if errors.Is(gerr, pgx.ErrNoRows) {
			return c, apperr.NotFound("Campaign not found.")
		}
		return c, apperr.Conflict("INVALID_TRANSITION", fmt.Sprintf("A %s campaign cannot be %s.", cur.Status, kind))
	}
	if err != nil {
		return c, err
	}
	if detail == nil {
		detail = map[string]any{}
	}
	dj, _ := json.Marshal(detail)
	if err := q.InsertCampaignEvent(ctx, db.InsertCampaignEventParams{CampaignID: id, Kind: kind, Detail: dj, ActorID: actor}); err != nil {
		return c, err
	}
	if audited[kind] {
		if err := audit.Record(ctx, q, audit.Entry{Action: "campaign." + kind, EntityType: "campaign", EntityID: &id, Changes: detail}); err != nil {
			return c, err
		}
	}
	if err := webhooks.CampaignFinished(ctx, tx, c); err != nil {
		return c, err
	}
	return c, s.publish(ctx, tx, q, c)
}

func lastError(d map[string]any) *string {
	if v, ok := d["error"].(string); ok {
		return &v
	}
	return nil
}

// ChangedSince returns the current state of campaign id (or of all with
// nil) when it changed at or after since or is still active, oldest change
// first (realtime re-sync).
func (s *Service) ChangedSince(ctx context.Context, id *uuid.UUID, since time.Time, limit int) ([]realtime.CampaignEvent, error) {
	cs, err := s.q.ListCampaignsChangedSince(ctx, db.ListCampaignsChangedSinceParams{Since: since, ID: id, Lim: int32(min(limit, 1000))}) //nolint:gosec // bounded above
	if err != nil {
		return nil, err
	}
	out := make([]realtime.CampaignEvent, 0, len(cs))
	for _, c := range cs {
		counts, err := stats(ctx, s.q, c.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, realtime.NewCampaignEvent(c, counts))
	}
	return out, nil
}

func (s *Service) publish(ctx context.Context, tx pgx.Tx, q *db.Queries, c db.Campaign) error {
	return s.publishAs(ctx, tx, q, c, realtime.TypeCampaignUpdated)
}

// publishAs announces c with its counts, measured rate and estimated time
// left: campaign.updated for state changes, campaign.progress for the
// throttled live counters.
func (s *Service) publishAs(ctx context.Context, tx pgx.Tx, q *db.Queries, c db.Campaign, typ string) error {
	counts, err := stats(ctx, q, c.ID)
	if err != nil {
		return err
	}
	ev := realtime.NewCampaignEvent(c, counts)
	ev.Type = typ
	ev.Rate, ev.ETASeconds = c.SendRate, eta(c, counts)
	return realtime.PublishCampaign(ctx, tx, ev)
}

// Start begins sending now, or schedules the campaign for later.
func (s *Service) Start(ctx context.Context, id uuid.UUID, at *time.Time, actor *uuid.UUID) (db.Campaign, error) {
	if err := licensegate.Check(ctx); err != nil {
		return db.Campaign{}, err
	}
	var c db.Campaign
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		cur, err := q.GetCampaign(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Campaign not found.")
		}
		if err != nil {
			return err
		}
		if _, err := s.checkTemplate(ctx, q, cur); err != nil {
			return err
		}
		if at != nil && at.After(time.Now().Add(30*time.Second)) {
			c, err = s.transition(ctx, q, tx, id, "scheduled", []string{"draft", "scheduled"}, "scheduled",
				map[string]any{"at": at.UTC()}, actor, at)
			return err
		}
		c, err = s.transition(ctx, q, tx, id, "preparing", []string{"draft", "scheduled"}, "started", nil, actor, nil)
		if err != nil {
			return err
		}
		_, err = outbox.Write(ctx, tx, outbox.TaskTopic(TypeMaterialize), map[string]string{"campaign_id": id.String()}, 0)
		return err
	})
	return c, err
}

// checkTemplate verifies that the campaign can be sent: its offer has not
// ended, its template is approved and the parameters (with the offer's
// values) fill it. It returns the template's shape.
func (s *Service) checkTemplate(ctx context.Context, q *db.Queries, c db.Campaign) (Shape, error) {
	offer, err := campaignOffer(ctx, q, c)
	if err != nil {
		return Shape{}, err
	}
	if offer != nil && offerEnded(*offer, time.Now()) {
		return Shape{}, apperr.Conflict("OFFER_EXPIRED", fmt.Sprintf("The offer %q ended on %s; link a running offer or remove it before sending.",
			offer.Title, offer.EndsAt.UTC().Format("2006-01-02 15:04 MST")))
	}
	pn, err := q.GetPhoneNumber(ctx, c.PhoneNumberID)
	if err != nil {
		return Shape{}, apperr.Validation("phone_number_id", "The campaign's phone number no longer exists.")
	}
	t, err := q.FindApprovedTemplate(ctx, db.FindApprovedTemplateParams{AccountID: pn.AccountID, Name: c.TemplateName, Language: c.TemplateLanguage})
	if errors.Is(err, pgx.ErrNoRows) {
		return Shape{}, apperr.Validation("template_name", "The template does not exist on the sender's account; sync templates first.")
	}
	if err != nil {
		return Shape{}, err
	}
	if t.Status != "APPROVED" {
		return Shape{}, apperr.Validation("template_name", fmt.Sprintf("The template is %s; only approved templates can be sent.", t.Status))
	}
	var params Params
	if len(c.TemplateParams) > 0 {
		if err := json.Unmarshal(c.TemplateParams, &params); err != nil {
			return Shape{}, apperr.Validation("template_params", "The template parameters are malformed.")
		}
	}
	sh := ShapeOf(t.Components, t.ParameterFormat, t.Category)
	if offer != nil {
		params = ApplyOffer(params, sh, *offer)
	}
	return sh, CheckParams(sh, params)
}

// templateShape is the shape of the campaign's template, or the zero shape
// when the template is gone (the legacy positional rendering then applies
// and Meta reports what is missing).
func templateShape(ctx context.Context, q *db.Queries, c db.Campaign) (Shape, error) {
	pn, err := q.GetPhoneNumber(ctx, c.PhoneNumberID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Shape{}, nil
	}
	if err != nil {
		return Shape{}, err
	}
	t, err := q.FindApprovedTemplate(ctx, db.FindApprovedTemplateParams{AccountID: pn.AccountID, Name: c.TemplateName, Language: c.TemplateLanguage})
	if errors.Is(err, pgx.ErrNoRows) {
		return Shape{}, nil
	}
	if err != nil {
		return Shape{}, err
	}
	return ShapeOf(t.Components, t.ParameterFormat, t.Category), nil
}

// Pause stops dispatching; queued messages that are not yet sending are
// held until resume.
func (s *Service) Pause(ctx context.Context, id uuid.UUID, actor *uuid.UUID, reason string) (db.Campaign, error) {
	var c db.Campaign
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		var detail map[string]any
		if reason != "" {
			detail = map[string]any{"reason": reason}
		}
		var err error
		c, err = s.transition(ctx, q, tx, id, "paused", []string{"running", "preparing", "paused_limit"}, "paused", detail, actor, nil)
		return err
	})
	return c, err
}

// ResumeOptions are what an operator may change when resuming.
type ResumeOptions struct {
	Settings
	// At schedules the resume for later (the campaign stays paused until
	// then); nil or a past time resumes now.
	At *time.Time
}

// Resume continues a paused campaign (any pause kind): held messages are
// queued again and a new dispatcher chain starts (the old one stops at its
// next tick). Recipients already sent to are never sent again: they are no
// longer pending, and held messages keep their identity.
func (s *Service) Resume(ctx context.Context, id uuid.UUID, actor *uuid.UUID) (db.Campaign, error) {
	return s.ResumeWith(ctx, id, actor, ResumeOptions{})
}

// ResumeWith is Resume with changes and an optional later start.
func (s *Service) ResumeWith(ctx context.Context, id uuid.UUID, actor *uuid.UUID, o ResumeOptions) (db.Campaign, error) {
	if err := licensegate.Check(ctx); err != nil {
		return db.Campaign{}, err
	}
	if err := o.Settings.validate(); err != nil {
		return db.Campaign{}, err
	}
	var c db.Campaign
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		cur, err := q.LockCampaign(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Campaign not found.")
		}
		if err != nil {
			return err
		}
		if !IsPaused(cur.Status) {
			return apperr.Conflict("INVALID_TRANSITION", fmt.Sprintf("A %s campaign cannot be resumed.", cur.Status))
		}
		changes, err := s.applySettings(ctx, q, cur, o.Settings)
		if err != nil {
			return err
		}
		if o.At != nil && o.At.After(time.Now().Add(5*time.Second)) {
			c, err = q.SetCampaignResumeAt(ctx, db.SetCampaignResumeAtParams{ID: id, ResumeAt: o.At})
			if err != nil {
				return err
			}
			detail := map[string]any{"at": o.At.UTC(), "changes": changes}
			dj, _ := json.Marshal(detail)
			if err := q.InsertCampaignEvent(ctx, db.InsertCampaignEventParams{CampaignID: id, Kind: "resume_scheduled", Detail: dj, ActorID: actor}); err != nil {
				return err
			}
			if err := audit.Record(ctx, q, audit.Entry{Action: "campaign.resume_scheduled", EntityType: "campaign", EntityID: &id, Changes: detail}); err != nil {
				return err
			}
			return s.publish(ctx, tx, q, c)
		}
		cur, err = q.GetCampaign(ctx, id)
		if err != nil {
			return err
		}
		if IsPaused(cur.Status) {
			if _, err := s.checkTemplate(ctx, q, cur); err != nil {
				var ae *apperr.Error
				if errors.As(err, &ae) && ae.Code == "OFFER_EXPIRED" {
					return err
				}
			}
		}
		next, topic := "running", TypeDispatch
		if cur.Status == "paused" && cur.MaterializeCursor == nil && cur.TotalRecipients == 0 {
			// Paused while preparing: finish materialising first.
			next, topic = "preparing", TypeMaterialize
		}
		var detail map[string]any
		if len(changes) > 0 {
			detail = map[string]any{"changes": changes}
		}
		c, err = s.transition(ctx, q, tx, id, next, PausedStatuses, "resumed", detail, actor, nil)
		if err != nil {
			return err
		}
		held, err := q.ReleaseHeldMessages(ctx, &id)
		if err != nil {
			return err
		}
		for _, m := range held {
			if _, err := outbox.Write(ctx, tx, outbox.TaskTopic(messaging.TypeSend), messaging.SendPayload(m, &id), 0); err != nil {
				return err
			}
		}
		_, err = outbox.Write(ctx, tx, outbox.TaskTopic(topic), dispatchPayload(c), 0)
		return err
	})
	return c, err
}

// Cancel stops the campaign for good; unsent messages are cancelled.
func (s *Service) Cancel(ctx context.Context, id uuid.UUID, actor *uuid.UUID) (db.Campaign, error) {
	var c db.Campaign
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		var err error
		c, err = s.transition(ctx, q, tx, id, "cancelled", append([]string{"draft", "scheduled", "preparing", "running"}, PausedStatuses...), "cancelled", nil, actor, nil)
		if err != nil {
			return err
		}
		if _, err := q.CancelPendingRecipients(ctx, id); err != nil {
			return err
		}
		_, err = q.CancelUnsentMessages(ctx, &id)
		return err
	})
	return c, err
}

func dispatchPayload(c db.Campaign) map[string]any {
	return map[string]any{"campaign_id": c.ID.String(), "gen": c.DispatchGen}
}

// filter turns an audience into a contact filter.
func (s *Service) filter(ctx context.Context, a Audience) (contacts.Filter, error) {
	f := contacts.Filter{TagID: a.TagID, GroupID: a.GroupID}
	if pc := a.PreviousCampaign; pc != nil {
		f.FromCampaign, f.FromClasses = &pc.CampaignID, pc.Classes
	}
	if a.SegmentID != nil {
		sg, err := s.q.GetSegment(ctx, *a.SegmentID)
		if err != nil {
			return f, fmt.Errorf("segment: %w", err)
		}
		n, err := segment.Parse(sg.Definition)
		if err != nil {
			return f, err
		}
		f.Segment = n
	}
	return f, nil
}

// Materialize resolves the audience into recipients, resuming after the
// last stored contact. Opted-out contacts are recorded as skipped.
func (s *Service) Materialize(ctx context.Context, id uuid.UUID) error {
	c, err := s.q.GetCampaign(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && c.Status != "preparing") {
		return nil
	}
	if err != nil {
		return err
	}
	if stop, err := s.pauseIfOfferEnded(ctx, c); stop || err != nil {
		return err
	}
	var a Audience
	_ = json.Unmarshal(c.Audience, &a)
	f, err := s.filter(ctx, a)
	if err != nil {
		return s.fail(ctx, id, "The audience could not be resolved: "+err.Error())
	}
	rules, err := rulesFor(ctx, s.q, c.PhoneNumberID, c.TemplateName, c.TemplateLanguage)
	if err != nil {
		return err
	}
	cursor := c.MaterializeCursor
	for {
		var page []db.Contact
		err := s.contacts.Page(ctx, f, cursor, materializePage, func(ct db.Contact) { page = append(page, ct) })
		if err != nil {
			return err
		}
		if len(page) == 0 {
			break
		}
		err = s.tx(ctx, func(_ pgx.Tx, q *db.Queries) error {
			cur, err := q.LockCampaign(ctx, id)
			if err != nil {
				return err
			}
			if cur.Status != "preparing" {
				return errStopped
			}
			var added int64
			for _, ct := range page {
				state, reason := "pending", (*string)(nil)
				// Consent blocks are visible and counted as skipped.
				if d := rules.decide(ct); !d.Allowed() {
					state, reason = "skipped", ptr(d.Reason)
				}
				n, err := q.InsertRecipient(ctx, db.InsertRecipientParams{CampaignID: id, ContactID: &ct.ID, PhoneE164: ct.PhoneE164, State: state, SkipReason: reason})
				if err != nil {
					return err
				}
				added += n
			}
			last := page[len(page)-1].ID
			return q.SetMaterializeCursor(ctx, db.SetMaterializeCursorParams{ID: id, Cursor: &last, Added: added})
		})
		if errors.Is(err, errStopped) {
			return nil // paused or cancelled meanwhile; resume continues here
		}
		if err != nil {
			return err
		}
		last := page[len(page)-1].ID
		cursor = &last
		if ctx.Err() != nil {
			return ctx.Err() // progress is saved; the retry resumes
		}
	}
	return s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		c, err := s.transition(ctx, q, tx, id, "running", []string{"preparing"}, "materialized", nil, nil, nil)
		if err != nil {
			var ae *apperr.Error
			if errors.As(err, &ae) {
				return nil // paused or cancelled at the last moment
			}
			return err
		}
		_, err = outbox.Write(ctx, tx, outbox.TaskTopic(TypeDispatch), dispatchPayload(c), 0)
		return err
	})
}

var errStopped = errors.New("campaign stopped")

func (s *Service) fail(ctx context.Context, id uuid.UUID, msg string) error {
	return s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		_, err := s.transition(ctx, q, tx, id, "failed", []string{"preparing", "running", "paused"}, "failed", map[string]any{"error": msg}, nil, nil)
		var ae *apperr.Error
		if errors.As(err, &ae) {
			return nil
		}
		return err
	})
}

// Dispatch is one dispatcher tick: it tops up the in-flight window and
// schedules the next tick, or finishes the campaign.
func (s *Service) Dispatch(ctx context.Context, id uuid.UUID, gen int32) error {
	// Restricted mode: the campaign pauses (and says why) instead of failing
	// its recipients; resuming needs a valid licence.
	if licensegate.Check(ctx) != nil {
		_, err := s.Pause(ctx, id, nil, "license_restricted")
		if e, ok := apperr.As(err); ok && e.Code == "INVALID_TRANSITION" {
			return nil
		}
		return err
	}
	return s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		c, err := q.LockCampaign(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if c.Status != "running" || c.DispatchGen != gen {
			return nil // paused, cancelled or superseded by a newer chain
		}
		offer, err := campaignOffer(ctx, q, c)
		if err != nil {
			return err
		}
		if offer != nil && offerEnded(*offer, time.Now()) {
			// The offer ended mid-campaign: stop with the reason. The
			// remaining recipients wait; resuming is refused while the
			// ended offer is linked, so the campaign is cancelled or
			// recreated with a running offer.
			_, err := s.transition(ctx, q, tx, id, "paused", []string{"running"}, "paused", offerExpiredDetail(*offer), nil, nil)
			return err
		}
		// The scheduler sizes this tick: the campaign's fair share of the
		// number's campaign capacity bounds what it keeps in flight, and the
		// portfolio's messaging limit bounds how many new customers it may
		// reach (engine.go).
		pl, err := s.plan(ctx, q, c)
		if err != nil {
			return err
		}
		inflight, err := q.CountInFlight(ctx, &id)
		if err != nil {
			return err
		}
		queued, limitHit := 0, false
		if free := pl.Inflight - inflight; free > 0 {
			lim := int32(min(free, 1000)) //nolint:gosec // bounded
			// Resends of withheld messages first: they waited a day already.
			retries, err := q.ClaimDueRetries(ctx, db.ClaimDueRetriesParams{CampaignID: id, Lim: lim})
			if err != nil {
				return err
			}
			rows := make([]db.ClaimPendingRecipientsRow, 0, lim)
			for _, r := range retries {
				rows = append(rows, db.ClaimPendingRecipientsRow(r))
			}
			if left := lim - int32(len(rows)); left > 0 { //nolint:gosec // bounded
				pending, err := q.ClaimPendingRecipients(ctx, db.ClaimPendingRecipientsParams{CampaignID: id, Lim: left})
				if err != nil {
					return err
				}
				rows = append(rows, pending...)
			}
			if len(rows) > 0 {
				if queued, limitHit, err = s.queue(ctx, tx, q, c, offer, rows, pl.Budget); err != nil {
					return err
				}
			}
		}
		if limitHit {
			// The next customer would exceed the portfolio's messaging
			// limit: wait (paused_limit) until the oldest counted customer
			// ages out; the scheduler job resumes the campaign then.
			_, err := s.transition(ctx, q, tx, id, "paused_limit", []string{"running"}, "paused_limit", pl.limitDetail(), nil, nil)
			return err
		}
		next := s.TickInterval
		if queued == 0 && inflight == 0 {
			pending, err := q.CountPendingRecipients(ctx, id)
			if err != nil {
				return err
			}
			retries, err := q.CountRetries(ctx, id)
			if err != nil {
				return err
			}
			if pending == 0 && retries.Due == 0 {
				// Messages Meta deferred (131049) and recipients waiting for a
				// resend take no in-flight slot, but the campaign is not done
				// until they are sent or given up.
				deferred, err := q.CountDeferred(ctx, &id)
				if err != nil {
					return err
				}
				if deferred == 0 && retries.Total == 0 {
					_, err := s.transition(ctx, q, tx, id, "completed", []string{"running"}, "completed", nil, nil, nil)
					return err
				}
				next = max(next, deferredTick)
			}
		}
		rd, err := q.RecordDispatch(ctx, db.RecordDispatchParams{ID: id, Queued: int64(queued)})
		if err != nil {
			return err
		}
		if queued > 0 || (rd.Rolled != nil && *rd.Rolled) {
			// At most every two seconds per campaign (ClaimCampaignProgress).
			if err := s.PublishProgress(ctx, tx, q, id); err != nil {
				return err
			}
		}
		_, err = outbox.Write(ctx, tx, outbox.TaskTopic(TypeDispatch), dispatchPayload(c), next)
		return err
	})
}

// deferredTick is the dispatcher period while only deferred messages are
// left: nothing to top up, only completion to notice.
const deferredTick = time.Minute

// queue renders and stores messages for claimed recipients. budget (nil =
// no limit) is how many more customers the portfolio may reach outside
// their service window; when the next recipient would exceed it, queue
// stops (the rest stay pending) and reports limitHit.
func (s *Service) queue(ctx context.Context, tx pgx.Tx, q *db.Queries, c db.Campaign, offer *db.Offer, rows []db.ClaimPendingRecipientsRow,
	budget *limitBudget) (n int, limitHit bool, err error) {
	var params Params
	_ = json.Unmarshal(c.TemplateParams, &params)
	rules, err := rulesFor(ctx, q, c.PhoneNumberID, c.TemplateName, c.TemplateLanguage)
	if err != nil {
		return 0, false, err
	}
	sh, err := templateShape(ctx, q, c)
	if err != nil {
		return 0, false, err
	}
	if offer != nil {
		params = ApplyOffer(params, sh, *offer)
	}
	for _, r := range rows {
		var ct db.Contact
		var err error
		if r.ContactID != nil {
			ct, err = q.GetContact(ctx, *r.ContactID)
		}
		if r.ContactID == nil || errors.Is(err, pgx.ErrNoRows) {
			if err := q.MarkRecipientSkipped(ctx, db.MarkRecipientSkippedParams{ID: r.ID, SkipReason: ptr("contact_deleted")}); err != nil {
				return n, false, err
			}
			continue
		}
		if err != nil {
			return n, false, err
		}
		// Consent may have changed since materialisation.
		if d := rules.decide(ct); !d.Allowed() {
			if err := q.MarkRecipientSkipped(ctx, db.MarkRecipientSkippedParams{ID: r.ID, SkipReason: ptr(d.Reason)}); err != nil {
				return n, false, err
			}
			continue
		}
		tpl, missing := Render(sh, params, ct, c.TemplateName, c.TemplateLanguage)
		if missing != "" {
			if err := q.MarkRecipientSkipped(ctx, db.MarkRecipientSkippedParams{ID: r.ID, SkipReason: ptr("missing_" + missing)}); err != nil {
				return n, false, err
			}
			continue
		}
		if budget != nil {
			counts, err := budget.counts(ctx, q, c.PhoneNumberID, ct.ID)
			if err != nil {
				return n, false, err
			}
			if counts && budget.left <= 0 {
				return n, true, nil
			}
			if counts {
				budget.left--
			}
		}
		var retryOf *uuid.UUID
		if r.State == "retry" {
			retryOf = r.MessageID
		}
		wm := whatsapp.Message{Type: "template", To: strings.TrimPrefix(ct.PhoneE164, "+"), Template: &tpl}
		payload, _ := json.Marshal(wm)
		mid := uuid.Must(uuid.NewV7())
		msg, err := q.InsertCampaignMessage(ctx, db.InsertCampaignMessageParams{ID: mid, PhoneNumberID: c.PhoneNumberID,
			ContactID: ct.ID, Body: "[template] " + c.TemplateName, Payload: payload, CampaignID: &c.ID, ConsentCategory: &rules.category, RetryOf: retryOf})
		if err != nil {
			return n, false, err
		}
		if err := messaging.RecordCampaignMessage(ctx, q, msg); err != nil {
			return n, false, err
		}
		if err := q.MarkRecipientQueued(ctx, db.MarkRecipientQueuedParams{ID: r.ID, MessageID: &mid}); err != nil {
			return n, false, err
		}
		if _, err := outbox.Write(ctx, tx, outbox.TaskTopic(messaging.TypeSend), messaging.SendPayload(mid, &c.ID), 0); err != nil {
			return n, false, err
		}
		n++
	}
	return n, false, nil
}

// PublishProgress publishes a campaign's counts after a delivery status
// change, at most every two seconds per campaign; later changes in a quiet
// period are published by Reap.
func (s *Service) PublishProgress(ctx context.Context, tx pgx.Tx, q *db.Queries, id uuid.UUID) error {
	c, err := q.ClaimCampaignProgress(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	full, err := q.GetCampaign(ctx, c.ID)
	if err != nil {
		return err
	}
	return s.publishAs(ctx, tx, q, full, realtime.TypeCampaignProgress)
}

// StartDue starts scheduled campaigns whose time has come.
func (s *Service) StartDue(ctx context.Context) (int, error) {
	n := 0
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		ids, err := q.DueScheduledCampaigns(ctx)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := q.InsertCampaignEvent(ctx, db.InsertCampaignEventParams{CampaignID: id, Kind: "started", Detail: []byte(`{"scheduled":true}`)}); err != nil {
				return err
			}
			if _, err := outbox.Write(ctx, tx, outbox.TaskTopic(TypeMaterialize), map[string]string{"campaign_id": id.String()}, 0); err != nil {
				return err
			}
		}
		n = len(ids)
		return nil
	})
	if err != nil {
		return n, err
	}
	// Paused campaigns whose resume time has come: paused_limit once the
	// portfolio limit frees up, any pause with a resume scheduled by an
	// operator. A campaign still over the limit pauses again at once.
	due, err := s.q.DueResumes(ctx)
	if err != nil {
		return n, err
	}
	for _, id := range due {
		_, err := s.ResumeWith(ctx, id, nil, ResumeOptions{})
		if e, ok := apperr.As(err); ok {
			s.log.WarnContext(ctx, "scheduled resume refused", "campaign_id", id, "code", e.Code)
			_, _ = s.q.SetCampaignResumeAt(ctx, db.SetCampaignResumeAtParams{ID: id})
			continue
		}
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// Reap repairs work a crashed worker left behind:
//   - messages stuck in "sending" become "unknown" (never re-sent; the
//     status webhook reconciles them);
//   - pending messages whose task may be lost get a fresh send task (the
//     send path claims each message once, so a duplicate task is harmless);
//   - running campaigns whose dispatcher stalled get a new chain;
//   - preparing campaigns whose materialisation stalled are resumed.
func (s *Service) Reap(ctx context.Context, send func(context.Context, uuid.UUID) error) (fixed int, err error) {
	stuck, err := s.q.StuckSendingMessages(ctx, 600)
	if err != nil {
		return 0, err
	}
	for _, id := range stuck {
		if err := send(ctx, id); err != nil { // Send marks "sending" leftovers unknown
			return fixed, err
		}
		fixed++
	}
	err = s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		stale, err := q.StalePendingMessages(ctx, 600)
		if err != nil {
			return err
		}
		for _, m := range stale {
			if err := q.TouchMessage(ctx, m.ID); err != nil {
				return err
			}
			if _, err := outbox.Write(ctx, tx, outbox.TaskTopic(messaging.TypeSend), messaging.SendPayload(m.ID, m.CampaignID), 0); err != nil {
				return err
			}
			fixed++
		}
		// Deferred messages whose retry task was lost.
		due, err := q.DueDeferredMessages(ctx, 600)
		if err != nil {
			return err
		}
		for _, m := range due {
			if _, err := outbox.Write(ctx, tx, outbox.TaskTopic(messaging.TypeSend), messaging.SendPayload(m.ID, m.CampaignID), 0); err != nil {
				return err
			}
			fixed++
		}
		idle, err := q.IdleRunningCampaigns(ctx, 120)
		if err != nil {
			return err
		}
		for _, id := range idle {
			c, err := q.SetCampaignStatus(ctx, db.SetCampaignStatusParams{ID: id, Status: "running", FromStates: []string{"running"}})
			if err != nil {
				return err
			}
			if _, err := outbox.Write(ctx, tx, outbox.TaskTopic(TypeDispatch), dispatchPayload(c), 0); err != nil {
				return err
			}
			fixed++
		}
		dirty, err := q.TakeDirtyCampaigns(ctx)
		if err != nil {
			return err
		}
		for _, c := range dirty {
			if err := s.publish(ctx, tx, q, c); err != nil {
				return err
			}
		}
		prep, err := q.StalePreparingCampaigns(ctx, 300)
		if err != nil {
			return err
		}
		for _, id := range prep {
			if err := q.TouchCampaign(ctx, id); err != nil {
				return err
			}
			if _, err := outbox.Write(ctx, tx, outbox.TaskTopic(TypeMaterialize), map[string]string{"campaign_id": id.String()}, 0); err != nil {
				return err
			}
			fixed++
		}
		return nil
	})
	return fixed, err
}

// Stats returns recipient and message counts.
func (s *Service) Stats(ctx context.Context, id uuid.UUID) (map[string]int64, error) {
	return stats(ctx, s.q, id)
}

func stats(ctx context.Context, q *db.Queries, id uuid.UUID) (map[string]int64, error) {
	out := map[string]int64{}
	rs, err := q.CampaignRecipientStats(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, r := range rs {
		out["recipients_"+r.State] = r.N
		out["recipients"] += r.N
	}
	ms, err := q.CampaignMessageStats(ctx, &id)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		out[m.Status] = m.N
	}
	return out, nil
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
