// Package whatsapptest is a programmable stand-in for Meta's Graph API. It
// lets tests (and the local E2E stack) exercise every send outcome —
// success, Meta error codes, slow responses, dropped connections — without
// ever contacting Meta.
package whatsapptest

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
)

// Behavior scripts the response to one send request.
type Behavior struct {
	// ErrorCode/HTTPStatus return a Graph API error (e.g. 130429, 429).
	ErrorCode  int
	HTTPStatus int
	// Delay before answering (exceed the client timeout to simulate one).
	Delay time.Duration
	// DropConnection reads the request, then closes the connection without
	// a response: the classic ambiguous outcome.
	DropConnection bool
	// BareStatus answers with this status and a non-JSON body (gateway error).
	BareStatus int
}

// Sent is a recorded outbound message.
type Sent struct {
	PhoneNumberID string
	Token         string
	Message       whatsapp.Message
	// Raw is the request body exactly as received.
	Raw   json.RawMessage
	WAMID string // empty when the scripted behavior failed
}

// Server is the mock.
type Server struct {
	*httptest.Server
	mu        sync.Mutex
	script    []Behavior
	sent      []Sent
	numbers   map[string][]whatsapp.PhoneNumber // waba → numbers
	templates map[string][]whatsapp.TemplateInfo
	media     map[string][]byte
	mediaType map[string]string
	tokens    map[string]bool // accepted access tokens; empty = accept all
	tokenInfo map[string]TokenInfo
	exchanged map[string]string // short-lived token → long-lived token
	currency  map[string]string // waba → currency
	pricing   map[string][]whatsapp.PricingDataPoint
	pricingQ  []string          // pricing_analytics field expansions requested
	created   []json.RawMessage // template creation bodies, as received
}

// SetCurrency scripts GET /{waba}?fields=currency.
func (s *Server) SetCurrency(waba, currency string) {
	s.mu.Lock()
	s.currency[waba] = currency
	s.mu.Unlock()
}

// SetPricingAnalytics scripts the data points pricing_analytics returns for
// a WABA (a nil Cost omits "cost", as for accounts on a partner's credit
// line).
func (s *Server) SetPricingAnalytics(waba string, points []whatsapp.PricingDataPoint) {
	s.mu.Lock()
	s.pricing[waba] = points
	s.mu.Unlock()
}

// PricingQueries returns the pricing_analytics field expansions requested.
func (s *Server) PricingQueries() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.pricingQ...)
}

// SetThroughput changes a registered number's throughput level.
func (s *Server) SetThroughput(waba, phoneNumberID, level string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.numbers[waba] {
		if s.numbers[waba][i].ID == phoneNumberID {
			s.numbers[waba][i].Throughput.Level = level
		}
	}
}

// wabaFields answers GET /{waba}?fields=… (currency, pricing_analytics);
// false when id is not a known WABA.
func (s *Server) wabaFields(w http.ResponseWriter, r *http.Request, waba string) bool {
	fields := r.URL.Query().Get("fields")
	s.mu.Lock()
	defer s.mu.Unlock()
	_, known := s.numbers[waba]
	if !known {
		_, known = s.currency[waba]
	}
	if !known {
		return false
	}
	switch {
	case strings.HasPrefix(fields, "pricing_analytics"):
		s.pricingQ = append(s.pricingQ, fields)
		points := s.pricing[waba]
		if points == nil {
			points = []whatsapp.PricingDataPoint{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": waba, "pricing_analytics": map[string]any{
			"data": []any{map[string]any{"data_points": points}}}})
	case fields == "currency":
		cur := s.currency[waba]
		if cur == "" {
			cur = "USD"
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": waba, "currency": cur})
	default:
		writeJSON(w, http.StatusOK, map[string]any{"id": waba})
	}
	return true
}

// TokenInfo scripts debug_token for one token. The zero value of a token
// that was never scripted is a valid, permanent System User token with
// both WhatsApp scopes on every asset.
type TokenInfo struct {
	Invalid   bool
	Error     string
	Type      string
	ExpiresAt int64 // unix seconds, 0 = never
	Scopes    []string
	Targets   map[string][]string
}

// SetTokenInfo scripts what debug_token reports for token.
func (s *Server) SetTokenInfo(token string, info TokenInfo) {
	s.mu.Lock()
	s.tokenInfo[token] = info
	s.mu.Unlock()
}

// AllowExchange makes oauth/access_token trade short for long.
func (s *Server) AllowExchange(short, long string) {
	s.mu.Lock()
	s.exchanged[short] = long
	s.mu.Unlock()
}

func (s *Server) debugToken(w http.ResponseWriter, r *http.Request) {
	tok := r.URL.Query().Get("input_token")
	s.mu.Lock()
	info, scripted := s.tokenInfo[tok]
	restricted := len(s.tokens) > 0 && !s.tokens[tok]
	s.mu.Unlock()
	if !scripted {
		info = TokenInfo{Type: "SYSTEM_USER", Invalid: restricted}
		if restricted {
			info.Error = "Error validating access token: Session has expired"
		}
	}
	if info.Type == "" {
		info.Type = "SYSTEM_USER"
	}
	if info.Scopes == nil {
		info.Scopes = []string{"whatsapp_business_management", "whatsapp_business_messaging"}
	}
	granular := []map[string]any{}
	for scope, ids := range info.Targets {
		granular = append(granular, map[string]any{"scope": scope, "target_ids": ids})
	}
	data := map[string]any{"app_id": "1234", "type": info.Type, "is_valid": !info.Invalid,
		"expires_at": info.ExpiresAt, "data_access_expires_at": 0, "scopes": info.Scopes, "granular_scopes": granular}
	if info.Invalid {
		data["error"] = map[string]any{"code": 190, "subcode": 463, "message": info.Error}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Server) exchange(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	long, ok := s.exchanged[r.URL.Query().Get("fb_exchange_token")]
	s.mu.Unlock()
	if !ok {
		graphError(w, http.StatusBadRequest, 190, "Invalid OAuth access token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_token": long, "token_type": "bearer", "expires_in": 5183944})
}

// New starts a mock Graph API.
func New() *Server {
	s := newServer()
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// NewHandler returns the mock as an http.Handler for embedding in another
// server (E2E stack).
func NewHandler() (*Server, http.Handler) {
	s := newServer()
	return s, http.HandlerFunc(s.serve)
}

func newServer() *Server {
	return &Server{numbers: map[string][]whatsapp.PhoneNumber{}, templates: map[string][]whatsapp.TemplateInfo{},
		media: map[string][]byte{}, mediaType: map[string]string{}, tokens: map[string]bool{},
		tokenInfo: map[string]TokenInfo{}, exchanged: map[string]string{}, currency: map[string]string{},
		pricing: map[string][]whatsapp.PricingDataPoint{}}
}

// Script queues behaviors for the next send requests (FIFO); once exhausted
// every send succeeds.
func (s *Server) Script(b ...Behavior) {
	s.mu.Lock()
	s.script = append(s.script, b...)
	s.mu.Unlock()
}

// Sent returns recorded messages.
func (s *Server) Sent() []Sent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Sent(nil), s.sent...)
}

// AddPhoneNumber registers a number under a WABA.
func (s *Server) AddPhoneNumber(waba string, n whatsapp.PhoneNumber) {
	s.mu.Lock()
	s.numbers[waba] = append(s.numbers[waba], n)
	s.mu.Unlock()
}

// AddTemplate registers a template under a WABA.
func (s *Server) AddTemplate(waba string, t whatsapp.TemplateInfo) {
	s.mu.Lock()
	s.templates[waba] = append(s.templates[waba], t)
	s.mu.Unlock()
}

// RequireToken makes the mock reject other tokens with error 190.
func (s *Server) RequireToken(t string) {
	s.mu.Lock()
	s.tokens[t] = true
	s.mu.Unlock()
}

func newID(prefix string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func graphError(w http.ResponseWriter, status, code int, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{
		"message": fmt.Sprintf("(#%d) %s", code, msg), "type": "OAuthException", "code": code,
		"error_data": map[string]any{"messaging_product": "whatsapp", "details": msg}, "fbtrace_id": "mock",
	}})
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if strings.HasPrefix(r.URL.Path, "/media-download/") {
		s.download(w, r, strings.TrimPrefix(r.URL.Path, "/media-download/"))
		return
	}
	if len(parts) < 2 || !strings.HasPrefix(parts[0], "v") {
		http.NotFound(w, r)
		return
	}
	// Token endpoints authenticate with an app token, not the inspected one.
	switch parts[1] {
	case "debug_token":
		s.debugToken(w, r)
		return
	case "oauth":
		s.exchange(w, r)
		return
	}
	// Resumable upload: POST /{app}/uploads opens a session, POST
	// /upload:<id> takes the bytes and returns the handle.
	if strings.HasPrefix(parts[1], "upload:") && r.Method == http.MethodPost {
		data, _ := io.ReadAll(r.Body)
		if r.Header.Get("file_offset") != "0" || len(data) == 0 {
			graphError(w, http.StatusBadRequest, 100, "Invalid upload")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"h": "4::mock-handle-" + parts[1][len("upload:"):]})
		return
	}
	if len(parts) == 3 && parts[2] == "uploads" && r.Method == http.MethodPost {
		writeJSON(w, http.StatusOK, map[string]any{"id": newID("upload:")})
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	s.mu.Lock()
	restricted := len(s.tokens) > 0 && !s.tokens[token]
	s.mu.Unlock()
	if restricted || token == "" {
		graphError(w, http.StatusUnauthorized, 190, "Error validating access token")
		return
	}
	id := parts[1]
	edge := ""
	if len(parts) > 2 {
		edge = parts[2]
	}
	switch {
	case r.Method == http.MethodPost && edge == "messages":
		s.send(w, r, id, token)
	case r.Method == http.MethodGet && edge == "phone_numbers":
		s.mu.Lock()
		data := s.numbers[id]
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"data": data})
	case r.Method == http.MethodGet && edge == "" && strings.HasPrefix(id, "media"):
		s.getMedia(w, r, id)
	case r.Method == http.MethodGet && edge == "" && s.wabaFields(w, r, id):
	case r.Method == http.MethodGet && edge == "":
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, list := range s.numbers {
			for _, n := range list {
				if n.ID == id {
					writeJSON(w, http.StatusOK, n)
					return
				}
			}
		}
		graphError(w, http.StatusBadRequest, 100, "Unsupported get request")
	case r.Method == http.MethodPost && (edge == "register" || edge == "subscribed_apps"):
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
	case edge == "message_templates":
		s.templatesHandler(w, r, id)
	case r.Method == http.MethodPost && edge == "media":
		s.upload(w, r)
	default:
		graphError(w, http.StatusBadRequest, 100, "Unsupported request")
	}
}

func (s *Server) send(w http.ResponseWriter, r *http.Request, pnid, token string) {
	body, _ := io.ReadAll(r.Body)
	var m whatsapp.Message
	if err := json.Unmarshal(body, &m); err != nil || m.To == "" || m.Type == "" {
		graphError(w, http.StatusBadRequest, 131008, "Required parameter is missing")
		return
	}
	s.mu.Lock()
	var b Behavior
	if len(s.script) > 0 {
		b, s.script = s.script[0], s.script[1:]
	}
	s.mu.Unlock()

	if b.Delay > 0 {
		select {
		case <-time.After(b.Delay):
		case <-r.Context().Done():
			s.record(pnid, token, m, body, "") // Meta may still process it
			return
		}
	}
	switch {
	case b.DropConnection:
		s.record(pnid, token, m, body, newID("wamid.")) // processed, but the answer is lost
		if hj, ok := w.(http.Hijacker); ok {
			if c, _, err := hj.Hijack(); err == nil {
				_ = c.(*net.TCPConn).SetLinger(0)
				_ = c.Close()
				return
			}
		}
		panic(http.ErrAbortHandler)
	case b.BareStatus != 0:
		w.WriteHeader(b.BareStatus)
		_, _ = w.Write([]byte("<html>upstream error</html>"))
		return
	case b.ErrorCode != 0:
		status := b.HTTPStatus
		if status == 0 {
			status = http.StatusBadRequest
		}
		s.record(pnid, token, m, body, "")
		graphError(w, status, b.ErrorCode, "scripted error")
		return
	}
	wamid := newID("wamid.")
	s.record(pnid, token, m, body, wamid)
	writeJSON(w, http.StatusOK, map[string]any{
		"messaging_product": "whatsapp",
		"contacts":          []any{map[string]string{"input": m.To, "wa_id": strings.TrimPrefix(m.To, "+")}},
		"messages":          []any{map[string]string{"id": wamid}},
	})
}

func (s *Server) record(pnid, token string, m whatsapp.Message, body []byte, wamid string) {
	s.mu.Lock()
	s.sent = append(s.sent, Sent{PhoneNumberID: pnid, Token: token, Message: m, Raw: append(json.RawMessage(nil), body...), WAMID: wamid})
	s.mu.Unlock()
}

// CreatedTemplates returns the bodies of template creation requests, as
// Meta received them.
func (s *Server) CreatedTemplates() []json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]json.RawMessage(nil), s.created...)
}

// ApproveTemplate marks a created template APPROVED (as Meta's review
// would), so a sync brings it in as sendable.
func (s *Server) ApproveTemplate(waba, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.templates[waba] {
		if t.Name == name {
			s.templates[waba][i].Status = "APPROVED"
		}
	}
}

func (s *Server) templatesHandler(w http.ResponseWriter, r *http.Request, waba string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"data": s.templates[waba], "paging": map[string]any{}})
	case http.MethodPost:
		body, _ := io.ReadAll(r.Body)
		var t whatsapp.TemplateInfo
		if err := json.Unmarshal(body, &t); err != nil || t.Name == "" {
			graphError(w, http.StatusBadRequest, 100, "Invalid template")
			return
		}
		s.created = append(s.created, body)
		t.ID, t.Status = newID("tpl"), "PENDING"
		s.templates[waba] = append(s.templates[waba], t)
		writeJSON(w, http.StatusOK, map[string]any{"id": t.ID, "status": t.Status, "category": t.Category})
	case http.MethodDelete:
		name := r.URL.Query().Get("name")
		keep := s.templates[waba][:0]
		for _, t := range s.templates[waba] {
			if t.Name != name {
				keep = append(keep, t)
			}
		}
		s.templates[waba] = keep
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
	}
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil { //nolint:gosec // G120: body bounded by MaxBytesReader above (test mock)
		graphError(w, http.StatusBadRequest, 131053, "Media upload error")
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		graphError(w, http.StatusBadRequest, 131053, "Media upload error")
		return
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	id := newID("media")
	s.mu.Lock()
	s.media[id], s.mediaType[id] = data, hdr.Header.Get("Content-Type")
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

// AddMedia makes a media ID resolvable (inbound media in tests).
func (s *Server) AddMedia(id, mimeType string, data []byte) {
	s.mu.Lock()
	s.media[id], s.mediaType[id] = data, mimeType
	s.mu.Unlock()
}

func (s *Server) getMedia(w http.ResponseWriter, r *http.Request, id string) {
	s.mu.Lock()
	data, ok := s.media[id]
	mt := s.mediaType[id]
	s.mu.Unlock()
	if !ok {
		graphError(w, http.StatusBadRequest, 100, "Unknown media")
		return
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "mime_type": mt, "file_size": len(data),
		"sha256": "", "url": fmt.Sprintf("%s://%s/media-download/%s", scheme, r.Host, id), "messaging_product": "whatsapp"})
}

func (s *Server) download(w http.ResponseWriter, r *http.Request, id string) {
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	s.mu.Lock()
	data, ok := s.media[id]
	mt := s.mediaType[id]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mt)
	_, _ = w.Write(data)
}

// ── Webhook helpers ────────────────────────────────────────────────────────

// InboundText builds a "messages" notification with one text message.
func InboundText(phoneNumberID, from, wamid, body, profileName string, at time.Time) []byte {
	return notification(phoneNumberID, map[string]any{
		"contacts": []any{map[string]any{"wa_id": from, "profile": map[string]string{"name": profileName}}},
		"messages": []any{map[string]any{"from": from, "id": wamid, "timestamp": fmt.Sprint(at.Unix()), "type": "text",
			"text": map[string]string{"body": body}}},
	})
}

// InboundMedia builds a notification with one media message.
func InboundMedia(phoneNumberID, from, wamid, mediaType, mediaID, mime, caption string, at time.Time) []byte {
	return notification(phoneNumberID, map[string]any{
		"contacts": []any{map[string]any{"wa_id": from, "profile": map[string]string{"name": ""}}},
		"messages": []any{map[string]any{"from": from, "id": wamid, "timestamp": fmt.Sprint(at.Unix()), "type": mediaType,
			mediaType: map[string]string{"id": mediaID, "mime_type": mime, "caption": caption}}},
	})
}

// StatusUpdate builds a "messages" notification with one status, with
// Meta's pricing object when pricing is given (Meta sends it on either the
// "sent" or the "delivered" status).
func StatusUpdate(phoneNumberID, wamid, status, recipient, callback string, at time.Time, errCode int, pricing ...whatsapp.Pricing) []byte {
	st := map[string]any{"id": wamid, "status": status, "timestamp": fmt.Sprint(at.Unix()), "recipient_id": recipient}
	if len(pricing) > 0 {
		st["pricing"] = pricing[0]
	}
	if callback != "" {
		st["biz_opaque_callback_data"] = callback
	}
	if errCode != 0 {
		st["errors"] = []any{map[string]any{"code": errCode, "title": "Message failed", "error_data": map[string]string{"details": "scripted failure"}}}
	}
	return notification(phoneNumberID, map[string]any{"statuses": []any{st}})
}

// TemplateStatus builds a "message_template_status_update" notification.
func TemplateStatus(waba, templateID, name, language, event, reason string) []byte {
	b, _ := json.Marshal(map[string]any{"object": "whatsapp_business_account", "entry": []any{map[string]any{
		"id": waba, "changes": []any{map[string]any{"field": "message_template_status_update", "value": map[string]any{
			"event": event, "message_template_id": json.Number(templateID), "message_template_name": name,
			"message_template_language": language, "reason": reason}}}}}})
	return b
}

// AccountChange builds a WABA-level notification for field (account_update,
// phone_number_quality_update, business_capability_update,
// message_template_quality_update, user_preferences …) with value as is.
func AccountChange(waba, field string, value any) []byte {
	b, _ := json.Marshal(map[string]any{"object": "whatsapp_business_account", "entry": []any{map[string]any{
		"id": waba, "time": time.Now().Unix(), "changes": []any{map[string]any{"field": field, "value": value}}}}})
	return b
}

// QualityUpdate builds a "phone_number_quality_update" notification.
// limit is max_daily_conversations_per_business ("" to omit).
func QualityUpdate(waba, displayPhoneNumber, event, currentLimit, limit string) []byte {
	v := map[string]any{"display_phone_number": displayPhoneNumber, "event": event}
	if currentLimit != "" {
		v["current_limit"] = currentLimit
	}
	if limit != "" {
		v["max_daily_conversations_per_business"] = limit
	}
	return AccountChange(waba, "phone_number_quality_update", v)
}

// AccountUpdate builds an "account_update" notification; extra fields
// (phone_number, restriction_info, ban_info …) are merged into the value.
func AccountUpdate(waba, event string, extra map[string]any) []byte {
	v := map[string]any{"event": event}
	for k, x := range extra {
		v[k] = x
	}
	return AccountChange(waba, "account_update", v)
}

// TemplateQuality builds a "message_template_quality_update" notification.
func TemplateQuality(waba, templateID, name, language, previous, current string) []byte {
	return AccountChange(waba, "message_template_quality_update", map[string]any{
		"previous_quality_score": previous, "new_quality_score": current, "message_template_id": json.Number(templateID),
		"message_template_name": name, "message_template_language": language})
}

// CapabilityUpdate builds a "business_capability_update" notification.
func CapabilityUpdate(waba, limit string, maxPhoneNumbersPerWABA int) []byte {
	return AccountChange(waba, "business_capability_update", map[string]any{
		"max_daily_conversations_per_business": limit, "max_phone_numbers_per_waba": maxPhoneNumbersPerWABA})
}

// UserPreferences builds a "user_preferences" notification (a user stopped
// or resumed marketing messages).
func UserPreferences(waba, phoneNumberID string, prefs ...whatsapp.UserPreference) []byte {
	contacts := make([]any, 0, len(prefs))
	for _, p := range prefs {
		contacts = append(contacts, map[string]any{"wa_id": p.WaID, "user_id": p.UserID, "profile": map[string]string{"name": ""}})
	}
	return AccountChange(waba, "user_preferences", map[string]any{
		"messaging_product": "whatsapp",
		"metadata":          map[string]string{"display_phone_number": "15550000000", "phone_number_id": phoneNumberID},
		"contacts":          contacts, "user_preferences": prefs})
}

func notification(phoneNumberID string, value map[string]any) []byte {
	value["messaging_product"] = "whatsapp"
	value["metadata"] = map[string]string{"display_phone_number": "15550000000", "phone_number_id": phoneNumberID}
	b, _ := json.Marshal(map[string]any{"object": "whatsapp_business_account", "entry": []any{map[string]any{
		"id": "waba", "changes": []any{map[string]any{"field": "messages", "value": value}}}}})
	return b
}

// Deliver POSTs a signed webhook body to url.
func Deliver(ctx context.Context, url, appSecret string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", whatsapp.Sign(body, appSecret))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}
