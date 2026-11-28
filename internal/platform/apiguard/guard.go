// Package apiguard enforces the OpenAPI contract at the edge of every API
// request: it finds the operation in the embedded spec, authenticates the
// caller against the operation's declared security requirements, checks
// permissions (scopes), rate-limits, validates the request against the schema
// and handles Idempotency-Key replays. Because authorization is read from the
// spec, an operation cannot be exposed without an explicit requirement.
package apiguard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/entitlements"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/ratelimit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Security scheme names used in the spec.
const (
	SchemePanel    = "panel"
	SchemeSession  = "session"
	SchemeAPIKey   = "apiKey"
	SchemePlatform = "platform"
)

// HostExtension names the kind of host an operation is served on:
// "tenant" (the default: a company's address), "platform" (the admin
// host only) or "any" (sign-up and the other pages that work before a
// workspace exists). Elsewhere the operation answers 404.
const HostExtension = "x-messora-host"

// Host scopes of HostExtension.
const (
	HostTenant   = "tenant"
	HostPlatform = "platform"
	HostAny      = "any"
)

// FeatureExtension names the plan feature (entitlements.Feature) an
// operation needs; without it the operation answers 403
// FEATURE_NOT_IN_PLAN. API keys additionally need the public_api feature
// for every operation.
const FeatureExtension = "x-messora-feature"

func featureOf(op *openapi3.Operation) entitlements.Feature {
	if v, ok := op.Extensions[FeatureExtension].(string); ok {
		return entitlements.Feature(v)
	}
	return ""
}

func hostScope(op *openapi3.Operation) string {
	if v, ok := op.Extensions[HostExtension].(string); ok {
		return v
	}
	return HostTenant
}

// PlatformTokenPrefix distinguishes platform admin sessions.
const PlatformTokenPrefix = "msp_"

// Default request budgets per minute (GCRA, burst = rate). They are
// published in the API documentation, so change them together with it.
const (
	SessionPerMinute   = 600
	APIKeyPerMinute    = 300
	AnonymousPerMinute = 120
)

// Rate-limit response headers (IETF draft-ietf-httpapi-ratelimit-headers).
const (
	HeaderRateLimitLimit     = "RateLimit-Limit"
	HeaderRateLimitRemaining = "RateLimit-Remaining"
	HeaderRateLimitReset     = "RateLimit-Reset"
	HeaderRateLimitPolicy    = "RateLimit-Policy"
)

// APIKeyPrefix distinguishes API keys from session tokens in the bearer header.
const APIKeyPrefix = "msk_"

// Authenticator resolves credentials.
type Authenticator interface {
	AuthenticateSession(ctx context.Context, token string) (*principal.Principal, error)
	AuthenticateAPIKey(ctx context.Context, key string) (*principal.Principal, error)
	AuthenticatePlatformSession(ctx context.Context, token string) (*principal.Principal, error)
}

// Options configures the guard.
type Options struct {
	Auth    Authenticator
	Limiter *ratelimit.Limiter
	Queries db.Querier // idempotency storage
	Logger  *slog.Logger
	// MFAEnrolmentOps may be called by sessions that must still enrol a
	// second factor; every other operation answers MFA_SETUP_REQUIRED.
	MFAEnrolmentOps []string
	// Limits per principal kind; anonymous panel calls are limited per IP.
	// With Plans, the session and API key budgets come from the tenant's
	// plan instead (platform sessions keep SessionLimit).
	SessionLimit, APIKeyLimit, AnonymousLimit ratelimit.Limit
	// Plans answers the tenant's plan: request budgets, the monthly API
	// quota and feature flags (docs/ENTITLEMENTS.md). Nil means no plan
	// checks.
	Plans entitlements.Provider
	// MaxBody caps request bodies (default 1 MiB). An operation with the
	// extension "x-messora-raw-body: <bytes>" accepts a larger streamed
	// body that is not schema-validated (file uploads); its handler parses it.
	MaxBody int64
}

// RawBodyExtension marks operations whose body is a streamed file.
const RawBodyExtension = "x-messora-raw-body"

// rawBodyLimit returns the operation's raw-body limit, or 0.
func rawBodyLimit(op *openapi3.Operation) int64 {
	v, ok := op.Extensions[RawBodyExtension]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}

// Guard is the middleware.
type Guard struct {
	router routers.Router
	o      Options
}

func init() {
	openapi3.DefineStringFormatValidator("email", openapi3.NewCallbackValidator(func(s string) error {
		a, err := mail.ParseAddress(s)
		if err != nil || a.Address != s {
			return errors.New("not a valid e-mail address")
		}
		return nil
	}))
	// Any RFC 9562 version, including the time-ordered v7 IDs Messora uses.
	openapi3.DefineStringFormatValidator("uuid", openapi3.NewRegexpFormatValidator(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`))
}

// New validates the spec (every operation must declare security) and builds
// the guard.
func New(spec *openapi3.T, o Options) (*Guard, error) {
	if o.MaxBody <= 0 {
		o.MaxBody = 1 << 20
	}
	if err := spec.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("apiguard: invalid spec: %w", err)
	}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op.Security == nil {
				return nil, fmt.Errorf("apiguard: %s %s has no explicit security requirement", method, path)
			}
			for _, req := range *op.Security {
				if len(req) == 0 {
					return nil, fmt.Errorf("apiguard: %s %s allows anonymous access; use the panel scheme", method, path)
				}
				for scheme := range req {
					if scheme != SchemePanel && scheme != SchemeSession && scheme != SchemeAPIKey && scheme != SchemePlatform {
						return nil, fmt.Errorf("apiguard: %s %s uses unknown scheme %q", method, path, scheme)
					}
				}
			}
			// Platform operations live on the admin host and only there;
			// nothing else may accept a platform session.
			scope := hostScope(op)
			if scope != HostTenant && scope != HostPlatform && scope != HostAny {
				return nil, fmt.Errorf("apiguard: %s %s has unknown %s %q", method, path, HostExtension, scope)
			}
			if f := featureOf(op); f != "" && !slices.Contains(entitlements.AllFeatures, f) {
				return nil, fmt.Errorf("apiguard: %s %s has unknown %s %q", method, path, FeatureExtension, f)
			}
			platformPath := strings.HasPrefix(path, "/platform/")
			if platformPath != (scope == HostPlatform) {
				return nil, fmt.Errorf("apiguard: %s %s: /platform/ operations, and only they, are %s: platform", method, path, HostExtension)
			}
			for _, req := range *op.Security {
				for scheme := range req {
					if (scheme == SchemePlatform) != platformPath && scheme != SchemePanel {
						return nil, fmt.Errorf("apiguard: %s %s: the platform scheme belongs to /platform/ operations", method, path)
					}
					if platformPath && (scheme == SchemeSession || scheme == SchemeAPIKey) {
						return nil, fmt.Errorf("apiguard: %s %s: platform operations accept no workspace credentials", method, path)
					}
				}
			}
		}
	}
	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		return nil, err
	}
	return &Guard{router: router, o: o}, nil
}

// Middleware wraps the generated API handler.
func (g *Guard) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, pathParams, err := g.router.FindRoute(r)
		if err != nil {
			if errors.Is(err, routers.ErrMethodNotAllowed) {
				httpx.MethodNotAllowed(w, r)
				return
			}
			httpx.NotFound(w, r)
			return
		}
		op := route.Operation
		if span := trace.SpanFromContext(r.Context()); span.IsRecording() {
			span.SetName(r.Method + " /api/v1" + route.Path)
			span.SetAttributes(attribute.String("http.route", "/api/v1"+route.Path), attribute.String("messora.operation", op.OperationID))
		}
		// Each kind of host serves its own operations: a workspace's
		// address never serves the platform panel and vice versa.
		if err := checkHost(r, hostScope(op)); err != nil {
			httpx.WriteError(w, r, err, g.o.Logger)
			return
		}

		p, err := g.authorize(r, *op.Security)
		if err != nil {
			httpx.WriteError(w, r, err, g.o.Logger)
			return
		}
		ctx := r.Context()
		if p != nil {
			ctx = principal.With(ctx, p)
			// The embedded spec may change the case of operation IDs.
			if p.MFAPending && !slices.ContainsFunc(g.o.MFAEnrolmentOps, func(id string) bool { return strings.EqualFold(id, op.OperationID) }) {
				httpx.WriteError(w, r, apperr.Forbidden("MFA_SETUP_REQUIRED", "Set up two-factor authentication to continue"), g.o.Logger)
				return
			}
			// A read-only support session changes nothing (sign-out aside).
			if p.ReadOnly() && r.Method != http.MethodGet && r.Method != http.MethodHead && !slices.ContainsFunc(readOnlyOps, func(id string) bool { return strings.EqualFold(id, op.OperationID) }) {
				httpx.WriteError(w, r, apperr.Forbidden("SUPPORT_READ_ONLY", "This support session is read-only"), g.o.Logger)
				return
			}
		}
		r = r.WithContext(ctx)

		plan, err := g.plan(ctx, p)
		if err != nil {
			httpx.WriteError(w, r, err, g.o.Logger)
			return
		}
		if err := g.rateLimit(w, r, p, plan); err != nil {
			httpx.WriteError(w, r, err, g.o.Logger)
			return
		}
		if err := g.entitled(w, r, p, plan, featureOf(op)); err != nil {
			httpx.WriteError(w, r, err, g.o.Logger)
			return
		}

		limit, raw := g.o.MaxBody, rawBodyLimit(op)
		if raw > 0 {
			limit = raw
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		if err := openapi3filter.ValidateRequest(ctx, &openapi3filter.RequestValidationInput{
			Request: r, PathParams: pathParams, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc, MultiError: true,
				ExcludeRequestBody: raw > 0},
		}); err != nil {
			httpx.WriteError(w, r, validationError(err), g.o.Logger)
			return
		}

		// Streamed uploads are not replayable (the body is not buffered).
		if r.Method == http.MethodPost && r.Header.Get("Idempotency-Key") != "" && p != nil && raw == 0 {
			g.idempotent(w, r, p, next)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// readOnlyOps change nothing despite their method: a read-only support
// session may call them (signing out, realtime connection tokens).
var readOnlyOps = []string{"logout", "createRealtimeConnectionToken", "createRealtimeSubscriptionToken"}

// checkHost refuses an operation on the wrong kind of host with 404, as if
// it did not exist there.
func checkHost(r *http.Request, scope string) error {
	kind := tenant.HostKindOf(r.Context())
	switch scope {
	case HostAny:
		return nil
	case HostPlatform:
		if kind != tenant.HostAdmin {
			return &apperr.Error{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: "Not found"}
		}
	default:
		if _, ok := tenant.FromCtx(r.Context()); !ok || kind == tenant.HostAdmin || kind == tenant.HostPlatform {
			return &apperr.Error{Status: http.StatusNotFound, Code: "TENANT_NOT_FOUND", Message: "There is no Messora workspace at this address"}
		}
	}
	return nil
}

// errWrongTenant is the error for credentials of another tenant; it looks
// exactly like an unknown token.
func errWrongTenant(isKey bool) error {
	if isKey {
		return apperr.Unauthorized("API_KEY_INVALID", "The API key is invalid, expired or revoked")
	}
	return apperr.Unauthorized("SESSION_INVALID", "Your session has ended. Sign in again.")
}

// authorize evaluates the alternatives of the security requirement (OR of
// ANDs) and returns the authenticated principal, if any.
func (g *Guard) authorize(r *http.Request, reqs openapi3.SecurityRequirements) (*principal.Principal, error) {
	ctx := r.Context()
	token := bearer(r)
	fromPanel := httpx.FromBFF(ctx)

	var (
		authenticated *principal.Principal
		authErr       error
		missing       []string
		panelOnly     bool
	)
	// Authenticate once, lazily, for whichever scheme the token belongs to.
	resolve := func(scheme string) (*principal.Principal, error) {
		isKey := strings.HasPrefix(token, APIKeyPrefix)
		isPlatform := strings.HasPrefix(token, PlatformTokenPrefix)
		switch {
		case token == "":
			return nil, nil
		case scheme == SchemeAPIKey && !isKey, scheme == SchemeSession && (isKey || isPlatform),
			scheme == SchemePlatform && !isPlatform:
			return nil, nil
		case (scheme == SchemeSession || scheme == SchemePlatform) && !fromPanel:
			// Session tokens are only honoured on the internal network.
			return nil, nil
		}
		if authenticated != nil || authErr != nil {
			return authenticated, authErr
		}
		switch {
		case isPlatform:
			authenticated, authErr = g.o.Auth.AuthenticatePlatformSession(ctx, token)
			// Platform sessions work on the admin host only.
			if authenticated != nil && (!authenticated.Platform || tenant.HostKindOf(ctx) != tenant.HostAdmin) {
				authenticated, authErr = nil, errWrongTenant(false)
			}
			return authenticated, authErr
		case isKey:
			authenticated, authErr = g.o.Auth.AuthenticateAPIKey(ctx, token)
		default:
			authenticated, authErr = g.o.Auth.AuthenticateSession(ctx, token)
		}
		// A session or key works only at its own tenant's address. The
		// lookup already ran as the host's tenant (row-level security), so
		// a mismatch here means a bug; it is refused all the same.
		if authenticated != nil {
			if host, ok := tenant.FromCtx(ctx); !ok || authenticated.TenantID != host || authenticated.Platform {
				authenticated = nil
				authErr = errWrongTenant(isKey)
			}
		}
		return authenticated, authErr
	}

	for _, req := range reqs {
		ok := true
		var who *principal.Principal
		for scheme, scopes := range req {
			switch scheme {
			case SchemePanel:
				if !fromPanel {
					ok, panelOnly = false, true
				}
			case SchemeSession, SchemeAPIKey, SchemePlatform:
				p, err := resolve(scheme)
				if err != nil || p == nil {
					ok = false
					continue
				}
				if (scheme == SchemeAPIKey) == (p.Kind == principal.KindUser) || (scheme == SchemePlatform) != p.Platform {
					ok = false
					continue
				}
				if !p.Has(scopes...) {
					ok = false
					for _, s := range scopes {
						if !p.Has(s) {
							missing = append(missing, s)
						}
					}
					continue
				}
				who = p
			}
		}
		if ok {
			return who, nil
		}
	}
	switch {
	case authErr != nil:
		return nil, authErr
	case authenticated != nil && len(missing) > 0:
		slices.Sort(missing)
		return nil, apperr.Forbidden("FORBIDDEN", "Missing permission: "+strings.Join(slices.Compact(missing), ", "))
	case authenticated != nil:
		return nil, apperr.Forbidden("FORBIDDEN", "These credentials cannot be used for this operation")
	case strings.HasPrefix(token, APIKeyPrefix) && !acceptsScheme(reqs, SchemeAPIKey):
		return nil, apperr.Forbidden("FORBIDDEN", "API keys cannot be used for this operation")
	case panelOnly && token == "":
		return nil, apperr.Forbidden("FORBIDDEN", "This endpoint is only available to the Messora panel")
	default:
		return nil, apperr.Unauthorized("UNAUTHORIZED", "Authentication required")
	}
}

func acceptsScheme(reqs openapi3.SecurityRequirements, scheme string) bool {
	for _, req := range reqs {
		if _, ok := req[scheme]; ok {
			return true
		}
	}
	return false
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// plan returns the tenant's plan for a workspace principal; nil for
// anonymous callers, platform sessions, or without a provider.
func (g *Guard) plan(ctx context.Context, p *principal.Principal) (*entitlements.Plan, error) {
	if g.o.Plans == nil || p == nil || p.Platform {
		return nil, nil
	}
	id, ok := tenant.FromCtx(ctx)
	if !ok || tenant.IsSystem(ctx) || tenant.IsPlatform(ctx) {
		return nil, nil
	}
	pl, err := g.o.Plans.Limits(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("apiguard: plan of %s: %w", id, err)
	}
	return &pl, nil
}

// Monthly quota headers on API key responses when the plan has a quota.
const (
	HeaderQuotaLimit     = "X-Messora-Quota-Limit"
	HeaderQuotaRemaining = "X-Messora-Quota-Remaining"
)

// entitled applies the plan's feature flags and the monthly API quota:
// API keys need public_api, an operation with x-messora-feature needs its
// feature, and API key calls count against api_calls_per_month.
func (g *Guard) entitled(w http.ResponseWriter, r *http.Request, p *principal.Principal, plan *entitlements.Plan, feature entitlements.Feature) error {
	if plan == nil {
		return nil
	}
	if p.Kind == principal.KindAPIKey {
		if err := plan.Require(entitlements.PublicAPI); err != nil {
			return err
		}
	}
	if feature != "" {
		if err := plan.Require(feature); err != nil {
			return err
		}
	}
	if p.Kind != principal.KindAPIKey || plan.APICallsPerMonth == entitlements.Unlimited || g.o.Limiter == nil {
		return nil
	}
	now := time.Now().UTC()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	next := month.AddDate(0, 1, 0)
	key := tenant.RedisKey(r.Context(), "quota:api:"+month.Format("200601"))
	n, err := g.o.Limiter.Count(r.Context(), key, next.Sub(now)+24*time.Hour)
	if err != nil {
		g.o.Logger.WarnContext(r.Context(), "quota counter unavailable; allowing request", "err", err)
		return nil // fail open, like the rate limiter
	}
	h := w.Header()
	h.Set(HeaderQuotaLimit, strconv.FormatInt(plan.APICallsPerMonth, 10))
	h.Set(HeaderQuotaRemaining, strconv.FormatInt(max(plan.APICallsPerMonth-n, 0), 10))
	if n <= plan.APICallsPerMonth {
		return nil
	}
	wait := next.Sub(now)
	// The budget headers describe the limit that binds now: the month's.
	h.Set(HeaderRateLimitLimit, strconv.FormatInt(plan.APICallsPerMonth, 10))
	h.Set(HeaderRateLimitRemaining, "0")
	h.Set(HeaderRateLimitReset, strconv.Itoa(ceilSeconds(wait)))
	h.Set(HeaderRateLimitPolicy, fmt.Sprintf("%d;w=%d", plan.APICallsPerMonth, ceilSeconds(next.Sub(month))))
	e := apperr.RateLimited(entitlements.CodeQuotaExceeded, "The workspace's monthly API quota is used up", wait)
	e.Details = []apperr.Detail{{Field: "limit", Code: "api_calls_per_month", Message: strconv.FormatInt(plan.APICallsPerMonth, 10)}}
	return e
}

// rateLimit consumes one request from the caller's budget and reports the
// budget in the RateLimit-* headers (on 429 too, next to Retry-After).
func (g *Guard) rateLimit(w http.ResponseWriter, r *http.Request, p *principal.Principal, plan *entitlements.Plan) error {
	if g.o.Limiter == nil {
		return nil
	}
	// Sessions and API keys count per tenant (messora:t:{<tenant>}:rl:api:…);
	// anonymous callers per IP address across tenants, so moving between
	// tenants' addresses gives no extra budget.
	var key string
	lim := g.o.AnonymousLimit
	switch {
	case p == nil:
		ip := principal.Meta(r.Context()).IP
		if !ip.IsValid() {
			return nil
		}
		key = tenant.GlobalRedisKey("rl:api:anon:" + ip.String())
	case p.Kind == principal.KindAPIKey:
		key, lim = tenant.RedisKey(r.Context(), "rl:api:"+p.Key()), g.o.APIKeyLimit
		if plan != nil {
			lim = ratelimit.PerMinute(plan.APIKeyRequestsPerMinute)
		}
	case p.Platform:
		// The admin panel's sessions belong to no tenant.
		key, lim = tenant.GlobalRedisKey("rl:api:platform:"+p.Key()), g.o.SessionLimit
	default:
		key, lim = tenant.RedisKey(r.Context(), "rl:api:"+p.Key()), g.o.SessionLimit
		if plan != nil {
			lim = ratelimit.PerMinute(plan.SessionRequestsPerMinute)
		}
	}
	if lim.Rate == 0 {
		return nil
	}
	res, err := g.o.Limiter.AllowRaw(r.Context(), key, lim)
	if err != nil {
		g.o.Logger.WarnContext(r.Context(), "rate limiter unavailable; allowing request", "err", err)
		return nil // fail open: Redis outages must not take the panel down
	}
	h := w.Header()
	h.Set(HeaderRateLimitLimit, strconv.Itoa(max(lim.Burst, 1)))
	h.Set(HeaderRateLimitRemaining, strconv.Itoa(res.Remaining))
	h.Set(HeaderRateLimitReset, strconv.Itoa(ceilSeconds(res.Reset)))
	h.Set(HeaderRateLimitPolicy, fmt.Sprintf("%d;w=%d", lim.Rate, ceilSeconds(lim.Period)))
	if !res.Allowed {
		return apperr.RateLimited("RATE_LIMITED", "Too many requests", res.RetryAfter)
	}
	return nil
}

func ceilSeconds(d time.Duration) int {
	return int((d + time.Second - 1) / time.Second)
}

// validationError converts kin-openapi errors into a 422 with field details.
func validationError(err error) error {
	out := &httpx.ValidationErrors{}
	var walk func(error)
	walk = func(e error) {
		var multi openapi3.MultiError
		if errors.As(e, &multi) {
			for _, m := range multi {
				walk(m)
			}
			return
		}
		var re *openapi3filter.RequestError
		if errors.As(e, &re) {
			var inner openapi3.MultiError
			if errors.As(re.Err, &inner) {
				for _, m := range inner {
					walk(m)
				}
				return
			}
			var se *openapi3.SchemaError
			if errors.As(re.Err, &se) {
				out.Add(strings.Join(se.JSONPointer(), "."), se.Reason)
				return
			}
			field := ""
			if re.Parameter != nil {
				field = re.Parameter.Name
			}
			out.Add(field, re.Reason)
			return
		}
		var se *openapi3.SchemaError
		if errors.As(e, &se) {
			out.Add(strings.Join(se.JSONPointer(), "."), se.Reason)
			return
		}
		out.Add("", "Invalid request")
	}
	walk(err)
	return out
}
