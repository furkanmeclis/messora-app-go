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
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/ratelimit"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Security scheme names used in the spec.
const (
	SchemePanel   = "panel"
	SchemeSession = "session"
	SchemeAPIKey  = "apiKey"
)

// APIKeyPrefix distinguishes API keys from session tokens in the bearer header.
const APIKeyPrefix = "msk_"

// Authenticator resolves credentials.
type Authenticator interface {
	AuthenticateSession(ctx context.Context, token string) (*principal.Principal, error)
	AuthenticateAPIKey(ctx context.Context, key string) (*principal.Principal, error)
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
	SessionLimit, APIKeyLimit, AnonymousLimit ratelimit.Limit
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
					if scheme != SchemePanel && scheme != SchemeSession && scheme != SchemeAPIKey {
						return nil, fmt.Errorf("apiguard: %s %s uses unknown scheme %q", method, path, scheme)
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
		}
		r = r.WithContext(ctx)

		if err := g.rateLimit(r, p); err != nil {
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
		switch {
		case token == "":
			return nil, nil
		case scheme == SchemeAPIKey && !isKey, scheme == SchemeSession && isKey:
			return nil, nil
		case scheme == SchemeSession && !fromPanel:
			// Session tokens are only honoured on the internal network.
			return nil, nil
		}
		if authenticated != nil || authErr != nil {
			return authenticated, authErr
		}
		if isKey {
			authenticated, authErr = g.o.Auth.AuthenticateAPIKey(ctx, token)
		} else {
			authenticated, authErr = g.o.Auth.AuthenticateSession(ctx, token)
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
			case SchemeSession, SchemeAPIKey:
				p, err := resolve(scheme)
				if err != nil || p == nil {
					ok = false
					continue
				}
				if (scheme == SchemeSession) != (p.Kind == principal.KindUser) {
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

func (g *Guard) rateLimit(r *http.Request, p *principal.Principal) error {
	if g.o.Limiter == nil {
		return nil
	}
	var key string
	lim := g.o.AnonymousLimit
	switch {
	case p == nil:
		ip := principal.Meta(r.Context()).IP
		if !ip.IsValid() {
			return nil
		}
		key = "anon:" + ip.String()
	case p.Kind == principal.KindAPIKey:
		key, lim = p.Key(), g.o.APIKeyLimit
	default:
		key, lim = p.Key(), g.o.SessionLimit
	}
	if lim.Rate == 0 {
		return nil
	}
	res, err := g.o.Limiter.Allow(r.Context(), "api:"+key, lim)
	if err != nil {
		g.o.Logger.WarnContext(r.Context(), "rate limiter unavailable; allowing request", "err", err)
		return nil // fail open: Redis outages must not take the panel down
	}
	if !res.Allowed {
		return apperr.RateLimited("RATE_LIMITED", "Too many requests", res.RetryAfter)
	}
	return nil
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
