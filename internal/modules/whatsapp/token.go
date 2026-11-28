package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// TokenInfo is what Graph's debug_token says about an access token.
type TokenInfo struct {
	Valid               bool
	Type                string // SYSTEM_USER, USER, PAGE, APP …
	AppID               string
	ExpiresAt           *time.Time // nil: never expires
	DataAccessExpiresAt *time.Time
	Scopes              []string
	// Targets lists, per scope, the asset ids the token is limited to
	// (empty: every asset the user can reach).
	Targets map[string][]string
	Error   string // why Meta considers it invalid
}

type debugTokenResponse struct {
	Data struct {
		AppID               string   `json:"app_id"`
		Type                string   `json:"type"`
		IsValid             bool     `json:"is_valid"`
		ExpiresAt           int64    `json:"expires_at"`
		DataAccessExpiresAt int64    `json:"data_access_expires_at"`
		Scopes              []string `json:"scopes"`
		GranularScopes      []struct {
			Scope     string   `json:"scope"`
			TargetIDs []string `json:"target_ids"`
		} `json:"granular_scopes"`
		Error *struct {
			Code    int    `json:"code"`
			Subcode int    `json:"subcode"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"data"`
}

func unix(n int64) *time.Time {
	if n <= 0 {
		return nil
	}
	t := time.Unix(n, 0).UTC()
	return &t
}

// DebugToken inspects token. inspector authorises the call: an app token
// ("app_id|app_secret") when known, otherwise the token itself.
func (c *Client) DebugToken(ctx context.Context, token, inspector string) (TokenInfo, error) {
	if inspector == "" {
		inspector = token
	}
	var resp debugTokenResponse
	if _, err := c.do(ctx, http.MethodGet, c.endpoint("debug_token", url.Values{"input_token": {token}}), inspector, nil, "", &resp); err != nil {
		return TokenInfo{}, err
	}
	d := resp.Data
	info := TokenInfo{Valid: d.IsValid, Type: d.Type, AppID: d.AppID, ExpiresAt: unix(d.ExpiresAt),
		DataAccessExpiresAt: unix(d.DataAccessExpiresAt), Scopes: d.Scopes, Targets: map[string][]string{}}
	for _, g := range d.GranularScopes {
		info.Targets[g.Scope] = g.TargetIDs
	}
	if d.Error != nil {
		info.Error = d.Error.Message
	}
	return info, nil
}

// ExchangeToken trades a short-lived user token for a long-lived one
// (about 60 days). System User tokens do not need this.
func (c *Client) ExchangeToken(ctx context.Context, appID, appSecret, token string) (string, *time.Time, error) {
	q := url.Values{"grant_type": {"fb_exchange_token"}, "client_id": {appID}, "client_secret": {appSecret}, "fb_exchange_token": {token}}
	var resp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if _, err := c.do(ctx, http.MethodGet, c.endpoint("oauth/access_token", q), token, nil, "", &resp); err != nil {
		return "", nil, err
	}
	var exp *time.Time
	if resp.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second).UTC()
		exp = &t
	}
	return resp.AccessToken, exp, nil
}

// ExchangeCode trades the code of an Embedded Signup for the customer's
// business token (server to server, no redirect_uri: Meta's Tech Provider
// onboarding guide). The code is single use and lives five minutes.
func (c *Client) ExchangeCode(ctx context.Context, appID, appSecret, code string) (string, error) {
	q := url.Values{"client_id": {appID}, "client_secret": {appSecret}, "code": {code}}
	var resp struct {
		AccessToken string `json:"access_token"`
	}
	if _, err := c.do(ctx, http.MethodGet, c.endpoint("oauth/access_token", q), "", nil, "", &resp); err != nil {
		return "", err
	}
	if resp.AccessToken == "" {
		return "", fmt.Errorf("whatsapp: the code exchange returned no token")
	}
	return resp.AccessToken, nil
}

// AppSubscription is one object of an app's webhook subscriptions
// (GET /{app-id}/subscriptions with the app token).
type AppSubscription struct {
	Object      string
	CallbackURL string
	Active      bool
	Fields      []string
}

// AppSubscriptions lists the app's webhook subscriptions; appToken is
// "app_id|app_secret".
func (c *Client) AppSubscriptions(ctx context.Context, appID, appToken string) ([]AppSubscription, error) {
	var resp struct {
		Data []struct {
			Object      string `json:"object"`
			CallbackURL string `json:"callback_url"`
			Active      bool   `json:"active"`
			Fields      []struct {
				Name string `json:"name"`
			} `json:"fields"`
		} `json:"data"`
	}
	if _, err := c.do(ctx, http.MethodGet, c.endpoint(url.PathEscape(appID)+"/subscriptions", nil), appToken, nil, "", &resp); err != nil {
		return nil, err
	}
	out := make([]AppSubscription, 0, len(resp.Data))
	for _, d := range resp.Data {
		s := AppSubscription{Object: d.Object, CallbackURL: d.CallbackURL, Active: d.Active}
		for _, f := range d.Fields {
			s.Fields = append(s.Fields, f.Name)
		}
		out = append(out, s)
	}
	return out, nil
}

// UploadTemplateSample uploads an example file for a template's media
// header with Meta's Resumable Upload API (it needs the app id) and returns
// the handle to put in the header's example.header_handle.
func (c *Client) UploadTemplateSample(ctx context.Context, appID, token, filename, mimeType string, data []byte) (string, error) {
	q := url.Values{"file_name": {filename}, "file_length": {strconv.Itoa(len(data))}, "file_type": {mimeType}}
	var session struct {
		ID string `json:"id"`
	}
	if _, err := c.do(ctx, http.MethodPost, c.endpoint(url.PathEscape(appID)+"/uploads", q), token, nil, "", &session); err != nil {
		return "", err
	}
	if !strings.HasPrefix(session.ID, "upload:") {
		return "", fmt.Errorf("whatsapp: unexpected upload session %q", session.ID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(session.ID, nil), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "OAuth "+token)
	req.Header.Set("file_offset", "0")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		var env struct {
			Error *APIError `json:"error"`
		}
		if json.Unmarshal(raw, &env) == nil && env.Error != nil {
			env.Error.HTTPStatus = resp.StatusCode
			return "", env.Error
		}
		return "", &httpError{status: resp.StatusCode, body: string(raw[:min(len(raw), 200)])}
	}
	var out struct {
		H string `json:"h"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.H == "" {
		return "", fmt.Errorf("whatsapp: upload returned no handle")
	}
	return out.H, nil
}
