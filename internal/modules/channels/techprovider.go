package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Messora as a Meta Tech Provider (docs/PHASE-17.md §3.2, §5.9): the
// platform side. One Meta app of Messora's own business onboards companies
// with Embedded Signup; its settings live in platform_settings.tech_provider
// and are changed only by the platform admin. The app secret, the system
// user token and the webhook verify token are sealed there; nothing secret
// is stored in plain text.

// SettingTechProvider is the platform_settings key.
const SettingTechProvider = "tech_provider"

// TechProviderWebhookPath is the platform webhook of every Tech Provider
// tenant below the webhook base URL (<base>/meta/tp). "tp" is never a
// webhook key (keys are 52 characters).
const TechProviderWebhookPath = WebhookKeyPath + "tp"

// Associated data of the sealed platform secrets.
const (
	tpAppSecretAD   = "platform.tech_provider.app_secret"
	tpSystemTokenAD = "platform.tech_provider.system_user_token"
	tpVerifyTokenAD = "platform.tech_provider.verify_token"
)

// TechProviderScopes are the permissions Messora's app needs (App Review):
// managing and messaging the customers' WhatsApp Business Accounts, and
// the business portfolio access Embedded Signup grants.
var TechProviderScopes = []string{"whatsapp_business_management", "whatsapp_business_messaging", "business_management"}

// tpStored is the JSON of platform_settings.tech_provider. []byte fields
// are sealed and travel as base64.
type tpStored struct {
	Enabled            bool       `json:"enabled"`
	AppID              *string    `json:"app_id"`
	ConfigurationID    *string    `json:"configuration_id"`
	GraphAPIVersion    *string    `json:"graph_api_version"`
	AppSecretEnc       []byte     `json:"app_secret_enc,omitempty"`
	SystemUserTokenEnc []byte     `json:"system_user_token_enc,omitempty"`
	VerifyTokenEnc     []byte     `json:"verify_token_enc,omitempty"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
}

// TechProviderConfig is the platform's Tech Provider set-up with its
// secrets opened. It never leaves the server: the platform API shows only
// whether each secret is set (and the verify token, which Meta's app
// dashboard needs).
type TechProviderConfig struct {
	Enabled         bool
	AppID           string
	ConfigurationID string
	GraphAPIVersion string // "" = the platform default (META_GRAPH_API_VERSION)
	AppSecret       string
	SystemUserToken string
	VerifyToken     string
	UpdatedAt       *time.Time
}

// Ready reports whether the configuration can onboard companies.
func (c TechProviderConfig) Ready() bool {
	return c.AppID != "" && c.AppSecret != "" && c.ConfigurationID != "" && c.VerifyToken != ""
}

// appToken is the app access token "app_id|app_secret".
func (c TechProviderConfig) appToken() string { return c.AppID + "|" + c.AppSecret }

func deref0(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// TechProvider reads the platform's Tech Provider configuration. Every
// request reads it (no cache): turning the switch off takes effect at once.
func (s *Service) TechProvider(ctx context.Context) (TechProviderConfig, error) {
	if _, ok := tenant.FromCtx(ctx); !ok && !tenant.IsSystem(ctx) {
		ctx = tenant.System(ctx)
	}
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT value FROM platform_settings WHERE key = $1`, SettingTechProvider).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return TechProviderConfig{}, nil
	}
	if err != nil {
		return TechProviderConfig{}, err
	}
	return s.openTechProvider(raw)
}

func (s *Service) openTechProvider(raw []byte) (TechProviderConfig, error) {
	var st tpStored
	if err := json.Unmarshal(raw, &st); err != nil {
		return TechProviderConfig{}, fmt.Errorf("tech provider settings: %w", err)
	}
	c := TechProviderConfig{Enabled: st.Enabled, AppID: deref0(st.AppID), ConfigurationID: deref0(st.ConfigurationID),
		GraphAPIVersion: deref0(st.GraphAPIVersion), UpdatedAt: st.UpdatedAt}
	open := func(enc []byte, ad string) (string, error) {
		if len(enc) == 0 {
			return "", nil
		}
		b, err := s.sealer.Open(enc, ad)
		if err != nil {
			return "", fmt.Errorf("decrypt %s: %w", ad, err)
		}
		return string(b), nil
	}
	var err error
	if c.AppSecret, err = open(st.AppSecretEnc, tpAppSecretAD); err != nil {
		return TechProviderConfig{}, err
	}
	if c.SystemUserToken, err = open(st.SystemUserTokenEnc, tpSystemTokenAD); err != nil {
		return TechProviderConfig{}, err
	}
	if c.VerifyToken, err = open(st.VerifyTokenEnc, tpVerifyTokenAD); err != nil {
		return TechProviderConfig{}, err
	}
	return c, nil
}

// TechProviderEnabled reads the platform switch of the Tech Provider
// method. It is on only when the platform admin turned it on and the
// configuration is complete; a broken configuration counts as off.
func (s *Service) TechProviderEnabled(ctx context.Context) bool {
	c, err := s.TechProvider(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "read the tech provider settings", "error", err)
		return false
	}
	return c.Enabled && c.Ready()
}

// enabledTechProvider returns the configuration when the method is on and
// 404 otherwise: while it is off, its endpoints do not exist.
func (s *Service) enabledTechProvider(ctx context.Context) (TechProviderConfig, error) {
	c, err := s.TechProvider(ctx)
	if err != nil {
		return c, err
	}
	if !c.Enabled || !c.Ready() {
		return c, &apperr.Error{Status: 404, Code: "TECH_PROVIDER_DISABLED", Message: "Connecting through Messora's Meta app is not available."}
	}
	return c, nil
}

// tpClient is the Graph client of the Tech Provider's Graph API version.
func (s *Service) tpClient(c TechProviderConfig) *whatsapp.Client {
	return s.wa.WithVersion(c.GraphAPIVersion)
}

// TechProviderPatch is a partial change of the platform configuration; nil
// fields stay. Secrets are write-only.
type TechProviderPatch struct {
	Enabled                        *bool
	AppID, ConfigurationID         *string
	GraphAPIVersion                *string // "" = the platform default
	AppSecret, SystemUserToken     *string // "" removes the system user token
	RotateVerifyToken, ClearSecret bool
}

var graphVersion = regexp.MustCompile(`^v[0-9]{1,3}\.[0-9]{1,2}$`)

// Execer runs the platform write: a connection that may write
// platform_settings (the owner in `messora tech-provider`, the platform
// role on the admin host once WP2's platform sessions exist).
type Execer interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// ErrPlatformWrite is returned when the connection may not write platform
// settings.
var ErrPlatformWrite = apperr.Forbidden("PLATFORM_WRITE_DENIED", "Only the platform admin can change Messora's Tech Provider settings.")

// UpdateTechProvider changes the platform configuration through w. The
// verify token is created with the first change and replaced on request;
// turning the method on needs a complete configuration.
// TODO(WP2-admin): record the change in platform_audit_logs once platform
// work has its audit context (audit.Record under tenant.Platform).
func (s *Service) UpdateTechProvider(ctx context.Context, w Execer, p TechProviderPatch) (TechProviderConfig, []string, error) {
	tx, err := w.Begin(ctx)
	if err != nil {
		return TechProviderConfig{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT value FROM platform_settings WHERE key = $1 FOR UPDATE`, SettingTechProvider).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		raw = []byte(`{"enabled": false}`)
	} else if err != nil {
		return TechProviderConfig{}, nil, platformWriteErr(err)
	}
	var st tpStored
	if err := json.Unmarshal(raw, &st); err != nil {
		return TechProviderConfig{}, nil, err
	}
	var changed []string
	set := func(field string, dst **string, v *string, re *regexp.Regexp, msg string) error {
		if v == nil {
			return nil
		}
		x := strings.TrimSpace(*v)
		if x != "" && re != nil && !re.MatchString(x) {
			return apperr.Validation(field, msg)
		}
		if x == "" {
			*dst = nil
		} else {
			*dst = &x
		}
		changed = append(changed, field)
		return nil
	}
	if err := set("app_id", &st.AppID, p.AppID, digits, "App ID must be numeric."); err != nil {
		return TechProviderConfig{}, nil, err
	}
	if err := set("configuration_id", &st.ConfigurationID, p.ConfigurationID, digits, "The Embedded Signup configuration ID must be numeric."); err != nil {
		return TechProviderConfig{}, nil, err
	}
	if err := set("graph_api_version", &st.GraphAPIVersion, p.GraphAPIVersion, graphVersion, "The Graph API version looks like v26.0."); err != nil {
		return TechProviderConfig{}, nil, err
	}
	seal := func(field, v, ad string, minLen int) ([]byte, error) {
		v = strings.TrimSpace(v)
		if len(v) < minLen || strings.ContainsAny(v, " \t\r\n") {
			return nil, apperr.Validation(field, fmt.Sprintf("This value looks too short (at least %d characters, no spaces).", minLen))
		}
		changed = append(changed, field)
		return s.sealer.Seal([]byte(v), ad)
	}
	if p.ClearSecret {
		st.AppSecretEnc = nil
		changed = append(changed, "app_secret")
	} else if p.AppSecret != nil {
		if st.AppSecretEnc, err = seal("app_secret", *p.AppSecret, tpAppSecretAD, 16); err != nil {
			return TechProviderConfig{}, nil, err
		}
	}
	if p.SystemUserToken != nil {
		if strings.TrimSpace(*p.SystemUserToken) == "" {
			st.SystemUserTokenEnc = nil
			changed = append(changed, "system_user_token")
		} else if st.SystemUserTokenEnc, err = seal("system_user_token", *p.SystemUserToken, tpSystemTokenAD, 20); err != nil {
			return TechProviderConfig{}, nil, err
		}
	}
	if p.RotateVerifyToken || len(st.VerifyTokenEnc) == 0 {
		v, err := newKey()
		if err != nil {
			return TechProviderConfig{}, nil, err
		}
		if st.VerifyTokenEnc, err = s.sealer.Seal([]byte(v), tpVerifyTokenAD); err != nil {
			return TechProviderConfig{}, nil, err
		}
		changed = append(changed, "verify_token")
	}
	if p.Enabled != nil {
		st.Enabled = *p.Enabled
		changed = append(changed, "enabled")
	}
	now := time.Now().UTC()
	st.UpdatedAt = &now
	out, err := json.Marshal(st)
	if err != nil {
		return TechProviderConfig{}, nil, err
	}
	cfg, err := s.openTechProvider(out)
	if err != nil {
		return TechProviderConfig{}, nil, err
	}
	if cfg.Enabled && !cfg.Ready() {
		return TechProviderConfig{}, nil, apperr.Validation("enabled", "Turning the Tech Provider on needs the App ID, the App secret and the Embedded Signup configuration ID.")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO platform_settings (key, value, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, SettingTechProvider, out); err != nil {
		return TechProviderConfig{}, nil, platformWriteErr(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return TechProviderConfig{}, nil, platformWriteErr(err)
	}
	s.log.InfoContext(ctx, "tech provider settings changed", "fields", changed, "enabled", cfg.Enabled)
	return cfg, changed, nil
}

func platformWriteErr(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "42501" {
		return ErrPlatformWrite
	}
	return err
}

// TestTechProvider checks the platform configuration with Meta: the app
// credentials and the system user token (debug_token with the app token),
// the Embedded Signup configuration, and the app's webhook subscription
// (GET /{app-id}/subscriptions), which must point at callbackURL.
func (s *Service) TestTechProvider(ctx context.Context, callbackURL string) ([]ConnectionCheck, error) {
	c, err := s.TechProvider(ctx)
	if err != nil {
		return nil, err
	}
	var out []ConnectionCheck
	add := func(key, status, code, msg string, params ...string) {
		ch := ConnectionCheck{Key: key, Status: status, Code: code, Message: msg}
		if len(params) > 0 {
			ch.Params = map[string]string{}
			for i := 0; i+1 < len(params); i += 2 {
				ch.Params[params[i]] = params[i+1]
			}
		}
		out = append(out, ch)
	}
	wa := s.tpClient(c)
	if c.AppID == "" || c.AppSecret == "" {
		add("app", CheckError, "tp_app_missing", "Enter the App ID and the App secret of Messora's Meta app.")
		return out, nil
	}
	if c.ConfigurationID == "" {
		add("configuration", CheckError, "tp_configuration_missing", "Create an Embedded Signup configuration (Facebook Login for Business → Configurations) and enter its ID.")
	} else {
		add("configuration", CheckOK, "tp_configuration_ok", "An Embedded Signup configuration ID is set.", "configuration_id", c.ConfigurationID)
	}
	// The system user token proves the app token too: debug_token with a
	// wrong app secret fails outright.
	token := c.SystemUserToken
	if token == "" {
		token = c.appToken()
	}
	info, err := wa.DebugToken(ctx, token, c.appToken())
	switch {
	case errors.Is(err, whatsapp.ErrDryRun):
		return nil, metaFailure(err)
	case err != nil:
		add("app", CheckError, "tp_app_invalid", "Meta refused the app credentials: "+metaFailure(err).Error(), "detail", metaFailure(err).Error())
		return out, nil
	default:
		add("app", CheckOK, "tp_app_ok", "Meta accepted the App ID and App secret.")
	}
	switch {
	case c.SystemUserToken == "":
		add("system_user_token", CheckWarning, "tp_system_token_missing", "No system user token of Messora's business is stored; it is needed to manage the app's own assets.")
	case !info.Valid:
		add("system_user_token", CheckError, "tp_system_token_invalid", "Meta says the system user token is not valid: "+info.Error, "detail", info.Error)
	case info.AppID != "" && info.AppID != c.AppID:
		add("system_user_token", CheckError, "tp_system_token_other_app", fmt.Sprintf("The system user token belongs to app %s, not to %s.", info.AppID, c.AppID), "token_app", info.AppID, "app_id", c.AppID)
	default:
		var missing []string
		for _, sc := range TechProviderScopes {
			if !slices.Contains(info.Scopes, sc) {
				missing = append(missing, sc)
			}
		}
		if len(missing) > 0 {
			add("system_user_token", CheckWarning, "tp_system_token_scopes", "The system user token lacks "+strings.Join(missing, ", ")+".", "missing", strings.Join(missing, ", "))
		} else {
			add("system_user_token", CheckOK, "tp_system_token_ok", "The system user token is valid with every permission.", "type", info.Type)
		}
	}
	subs, err := wa.AppSubscriptions(ctx, c.AppID, c.appToken())
	if err != nil {
		add("webhook", CheckError, "tp_subscription_error", "Meta did not return the app's webhook subscriptions: "+metaFailure(err).Error(), "detail", metaFailure(err).Error())
		return out, nil
	}
	i := slices.IndexFunc(subs, func(x whatsapp.AppSubscription) bool { return x.Object == "whatsapp_business_account" })
	switch {
	case i < 0:
		add("webhook", CheckError, "tp_subscription_missing", "The app has no webhook for WhatsApp Business Accounts. In the app dashboard, WhatsApp → Configuration, enter "+callbackURL+" with the verify token.", "callback_url", callbackURL)
	case subs[i].CallbackURL != callbackURL:
		add("webhook", CheckError, "tp_subscription_url", "The app's webhook points at "+subs[i].CallbackURL+", not at "+callbackURL+".", "callback_url", callbackURL, "current", subs[i].CallbackURL)
	case !subs[i].Active:
		add("webhook", CheckError, "tp_subscription_inactive", "The app's WhatsApp webhook is not active.")
	case !slices.Contains(subs[i].Fields, "messages"):
		add("webhook", CheckWarning, "tp_subscription_fields", "The app's webhook does not subscribe the messages field.", "fields", strings.Join(subs[i].Fields, ", "))
	default:
		add("webhook", CheckOK, "tp_subscription_ok", "The app's webhook points at Messora's Tech Provider address.", "callback_url", callbackURL)
	}
	return out, nil
}

// ── Webhook routing ────────────────────────────────────────────────────────

// TechProviderRoutes resolves the WABA ids of a Tech Provider webhook to
// their tenants through the platform routing table (meta_routes), before
// any tenant is known. Unknown ids are missing from the map.
func (s *Service) TechProviderRoutes(ctx context.Context, wabas []string) (map[string]KeyRoute, error) {
	out := map[string]KeyRoute{}
	if len(wabas) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(tenant.System(ctx), `
		SELECT r.waba_id, r.tenant_id, t.status FROM meta_routes r JOIN tenants t ON t.id = r.tenant_id
		WHERE r.waba_id = ANY($1)`, wabas)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var waba string
		var r KeyRoute
		if err := rows.Scan(&waba, &r.TenantID, &r.Status); err != nil {
			return nil, err
		}
		out[waba] = r
	}
	return out, rows.Err()
}

// TechProviderWABAs returns the context tenant's Tech Provider WABAs (the
// entries the platform webhook may store for it).
func (s *Service) TechProviderWABAs(ctx context.Context) ([]string, error) {
	return s.q.ListTechProviderWABAIDs(ctx)
}

// unrouted counts Tech Provider webhook entries for a WABA no tenant has
// (dropped), per process; the platform API shows it.
var unrouted struct {
	n    atomic.Int64
	last atomic.Value // string: the latest unrouted WABA id
}

// RecordUnrouted counts a dropped entry of an unknown WABA.
func RecordUnrouted(waba string) {
	unrouted.n.Add(1)
	unrouted.last.Store(truncateID(waba))
}

// Unrouted returns the count of dropped entries since the process started
// and the latest WABA id among them.
func Unrouted() (int64, string) {
	last, _ := unrouted.last.Load().(string)
	return unrouted.n.Load(), last
}

func truncateID(s string) string {
	if len(s) > 64 {
		return s[:64]
	}
	return s
}
