// Package whatsapp is the adapter for Meta's WhatsApp Cloud API (Graph API).
// It has no database access: callers pass credentials per call, which keeps
// it easy to test against the whatsapptest mock server.
package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/furkanmeclis/messora-app-go/internal/platform/tracing"
)

// DefaultVersion is the newest Graph API version verified for Messora
// (Meta's versions table, 2026-11). Configure META_GRAPH_API_VERSION to move.
const DefaultVersion = "v26.0"

// DefaultBaseURL is Meta's Graph API host.
const DefaultBaseURL = "https://graph.facebook.com"

// Options configures a Client.
type Options struct {
	BaseURL string // default DefaultBaseURL; tests point it at whatsapptest
	Version string // default DefaultVersion
	Timeout time.Duration
	// DryRun refuses every request to Meta's real hosts. It is forced on in
	// test environments so tests can never message real customers.
	DryRun bool
}

// Client calls the Graph API.
type Client struct {
	base    *url.URL
	version string
	http    *http.Client
}

// ErrDryRun is returned when DryRun blocks a request to Meta.
var ErrDryRun = errors.New("whatsapp: dry-run mode refuses requests to Meta's servers")

// New builds a client.
func New(o Options) (*Client, error) {
	if o.BaseURL == "" {
		o.BaseURL = DefaultBaseURL
	}
	if o.Version == "" {
		o.Version = DefaultVersion
	}
	if !strings.HasPrefix(o.Version, "v") {
		return nil, fmt.Errorf("whatsapp: graph api version must look like v26.0, got %q", o.Version)
	}
	if o.Timeout == 0 {
		o.Timeout = 20 * time.Second
	}
	u, err := url.Parse(strings.TrimRight(o.BaseURL, "/"))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("whatsapp: invalid base url %q", o.BaseURL)
	}
	var rt http.RoundTripper = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: o.Timeout,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       90 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	if o.DryRun {
		rt = dryRunGuard{next: rt}
	}
	if tracing.Enabled() {
		rt = otelhttp.NewTransport(rt)
	}
	return &Client{base: u, version: o.Version, http: &http.Client{Transport: rt, Timeout: o.Timeout,
		// Media download URLs live on Meta CDNs; redirects are not expected.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// Version returns the configured Graph API version.
func (c *Client) Version() string { return c.version }

type dryRunGuard struct{ next http.RoundTripper }

func (g dryRunGuard) RoundTrip(r *http.Request) (*http.Response, error) {
	h := strings.ToLower(r.URL.Hostname())
	if h == "graph.facebook.com" || strings.HasSuffix(h, ".facebook.com") || strings.HasSuffix(h, ".fbcdn.net") ||
		strings.HasSuffix(h, ".whatsapp.net") || strings.HasSuffix(h, ".fbsbx.com") {
		return nil, ErrDryRun
	}
	return g.next.RoundTrip(r)
}

func (c *Client) endpoint(path string, q url.Values) string {
	u := *c.base
	u.Path = strings.TrimRight(u.Path, "/") + "/" + c.version + "/" + strings.TrimLeft(path, "/")
	u.RawQuery = q.Encode()
	return u.String()
}

// do performs a request, decoding a JSON success body into out. It reports
// whether the request bytes were written to the network, which decides if a
// failure is ambiguous.
func (c *Client) do(ctx context.Context, method, endpoint, token string, body io.Reader, contentType string, out any) (wrote bool, err error) {
	var written atomic.Bool
	trace := &httptrace.ClientTrace{WroteRequest: func(httptrace.WroteRequestInfo) { written.Store(true) }}
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), method, endpoint, body)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return written.Load(), err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return true, err
	}
	if resp.StatusCode >= 400 {
		var env struct {
			Error *APIError `json:"error"`
		}
		if json.Unmarshal(raw, &env) == nil && env.Error != nil {
			env.Error.HTTPStatus = resp.StatusCode
			return true, env.Error
		}
		return true, &httpError{status: resp.StatusCode, body: string(raw[:min(len(raw), 200)])}
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return true, fmt.Errorf("whatsapp: decode response: %w", err)
		}
	}
	return true, nil
}

type httpError struct {
	status int
	body   string
}

func (e *httpError) Error() string { return fmt.Sprintf("meta api http %d: %s", e.status, e.body) }

// SendMessage submits a message. attempt (0-based) shapes the suggested
// retry delay. Every failure is a *SendError carrying its Outcome.
func (c *Client) SendMessage(ctx context.Context, phoneNumberID, token string, m Message, attempt int) (SendResult, error) {
	m.MessagingProduct, m.RecipientType = "whatsapp", "individual"
	body, err := json.Marshal(m)
	if err != nil {
		return SendResult{}, &SendError{Outcome: Permanent, Err: err}
	}
	var resp sendResponse
	wrote, err := c.do(ctx, http.MethodPost, c.endpoint(url.PathEscape(phoneNumberID)+"/messages", nil), token,
		bytes.NewReader(body), "application/json", &resp)
	if err != nil {
		return SendResult{}, classifyTransport(err, wrote, attempt)
	}
	if len(resp.Messages) == 0 || resp.Messages[0].ID == "" {
		// Meta answered 2xx without an ID: it may well have accepted it.
		return SendResult{}, &SendError{Outcome: Ambiguous, Err: errors.New("whatsapp: success response without message id")}
	}
	r := SendResult{MessageID: resp.Messages[0].ID, Status: resp.Messages[0].MessageStatus}
	if len(resp.Contacts) > 0 {
		r.WaID = resp.Contacts[0].WaID
	}
	return r, nil
}

func classifyTransport(err error, wrote bool, attempt int) error {
	var api *APIError
	if errors.As(err, &api) {
		return Classify(api, attempt)
	}
	if errors.Is(err, ErrDryRun) {
		return &SendError{Outcome: Permanent, Err: err}
	}
	var he *httpError
	if errors.As(err, &he) {
		if he.status == http.StatusTooManyRequests {
			return &SendError{Outcome: Retryable, RetryAfter: backoff(attempt, 2*time.Second, 2*time.Minute), Err: err}
		}
		// A gateway error without Meta's error body: the request may have
		// been processed behind the gateway.
		return &SendError{Outcome: Ambiguous, Err: err}
	}
	if !wrote {
		// Never left this machine (DNS, connect, TLS): safe to retry.
		return &SendError{Outcome: Retryable, RetryAfter: backoff(attempt, 2*time.Second, 5*time.Minute), Err: err}
	}
	return &SendError{Outcome: Ambiguous, Err: err}
}

// ── Account and phone numbers ──────────────────────────────────────────────

// ListPhoneNumbers returns the WABA's phone numbers.
func (c *Client) ListPhoneNumbers(ctx context.Context, wabaID, token string) ([]PhoneNumber, error) {
	q := url.Values{"fields": {"id,display_phone_number,verified_name,quality_rating,code_verification_status,status,whatsapp_business_manager_messaging_limit,throughput"}}
	var out struct {
		Data []PhoneNumber `json:"data"`
	}
	_, err := c.do(ctx, http.MethodGet, c.endpoint(url.PathEscape(wabaID)+"/phone_numbers", q), token, nil, "", &out)
	return out.Data, err
}

// GetPhoneNumber returns one number's details.
func (c *Client) GetPhoneNumber(ctx context.Context, phoneNumberID, token string) (PhoneNumber, error) {
	q := url.Values{"fields": {"id,display_phone_number,verified_name,quality_rating,code_verification_status,status,whatsapp_business_manager_messaging_limit,throughput"}}
	var out PhoneNumber
	_, err := c.do(ctx, http.MethodGet, c.endpoint(url.PathEscape(phoneNumberID), q), token, nil, "", &out)
	return out, err
}

// RegisterPhoneNumber registers a number for Cloud API with its two-step
// verification PIN (Meta allows 10 attempts per 72 hours).
func (c *Client) RegisterPhoneNumber(ctx context.Context, phoneNumberID, token, pin string) error {
	body, _ := json.Marshal(map[string]string{"messaging_product": "whatsapp", "pin": pin})
	_, err := c.do(ctx, http.MethodPost, c.endpoint(url.PathEscape(phoneNumberID)+"/register", nil), token, bytes.NewReader(body), "application/json", nil)
	return err
}

// SubscribeApp subscribes the app to the WABA's webhooks.
func (c *Client) SubscribeApp(ctx context.Context, wabaID, token string) error {
	_, err := c.do(ctx, http.MethodPost, c.endpoint(url.PathEscape(wabaID)+"/subscribed_apps", nil), token, nil, "", nil)
	return err
}

// ── Templates ──────────────────────────────────────────────────────────────

// ListTemplates returns every template of the WABA, following pagination.
func (c *Client) ListTemplates(ctx context.Context, wabaID, token string) ([]TemplateInfo, error) {
	q := url.Values{"fields": {"id,name,language,category,status,parameter_format,quality_score,rejected_reason,components"}, "limit": {"100"}}
	var all []TemplateInfo
	for page := 0; page < 100; page++ {
		var out struct {
			Data   []TemplateInfo `json:"data"`
			Paging struct {
				Cursors struct {
					After string `json:"after"`
				} `json:"cursors"`
				Next string `json:"next"`
			} `json:"paging"`
		}
		if _, err := c.do(ctx, http.MethodGet, c.endpoint(url.PathEscape(wabaID)+"/message_templates", q), token, nil, "", &out); err != nil {
			return nil, err
		}
		all = append(all, out.Data...)
		if out.Paging.Next == "" || out.Paging.Cursors.After == "" {
			return all, nil
		}
		q.Set("after", out.Paging.Cursors.After)
	}
	return all, errors.New("whatsapp: too many template pages")
}

// CreateTemplate submits a template for review.
func (c *Client) CreateTemplate(ctx context.Context, wabaID, token string, t json.RawMessage) (id, status string, err error) {
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	_, err = c.do(ctx, http.MethodPost, c.endpoint(url.PathEscape(wabaID)+"/message_templates", nil), token, bytes.NewReader(t), "application/json", &out)
	return out.ID, out.Status, err
}

// DeleteTemplate deletes one language version (hsmID) or every language (name only).
func (c *Client) DeleteTemplate(ctx context.Context, wabaID, token, name, hsmID string) error {
	q := url.Values{"name": {name}}
	if hsmID != "" {
		q.Set("hsm_id", hsmID)
	}
	_, err := c.do(ctx, http.MethodDelete, c.endpoint(url.PathEscape(wabaID)+"/message_templates", q), token, nil, "", nil)
	return err
}

// ── Media ──────────────────────────────────────────────────────────────────

// UploadMedia uploads a file and returns its media ID (valid 30 days).
func (c *Client) UploadMedia(ctx context.Context, phoneNumberID, token, filename, mimeType string, r io.Reader) (string, error) {
	// Stream the multipart body: files may be up to 100 MB.
	pr, pw := io.Pipe()
	w := multipart.NewWriter(pw)
	go func() {
		err := func() error {
			if err := w.WriteField("messaging_product", "whatsapp"); err != nil {
				return err
			}
			if err := w.WriteField("type", mimeType); err != nil {
				return err
			}
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
			h.Set("Content-Type", mimeType)
			part, err := w.CreatePart(h)
			if err != nil {
				return err
			}
			if _, err := io.Copy(part, r); err != nil {
				return err
			}
			return w.Close()
		}()
		_ = pw.CloseWithError(err)
	}()
	var out struct {
		ID string `json:"id"`
	}
	_, err := c.do(ctx, http.MethodPost, c.endpoint(url.PathEscape(phoneNumberID)+"/media", nil), token, pr, w.FormDataContentType(), &out)
	_ = pr.CloseWithError(io.ErrClosedPipe) // stop the writer if the request ended early
	if err == nil && out.ID == "" {
		err = errors.New("whatsapp: upload response without media id")
	}
	return out.ID, err
}

// GetMedia resolves a media ID to a short-lived (5 minute) download URL.
func (c *Client) GetMedia(ctx context.Context, mediaID, token string) (MediaInfo, error) {
	var out MediaInfo
	_, err := c.do(ctx, http.MethodGet, c.endpoint(url.PathEscape(mediaID), nil), token, nil, "", &out)
	return out, err
}

// DownloadMedia streams the file at a URL from GetMedia into w, refusing
// more than maxBytes.
func (c *Client) DownloadMedia(ctx context.Context, mediaURL, token string, w io.Writer, maxBytes int64) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, &httpError{status: resp.StatusCode}
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return n, err
	}
	if n > maxBytes {
		return n, fmt.Errorf("whatsapp: media larger than %d bytes", maxBytes)
	}
	return n, nil
}
