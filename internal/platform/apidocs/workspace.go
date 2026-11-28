package apidocs

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"
)

// Workspace is the company a public document is served for: every company
// has its own API address (docs/PHASE-17.md §2), so its document names that
// address as the server, and the introduction and code samples use it.
type Workspace struct {
	// Name is the company's display name.
	Name string
	// BaseURL is the origin of its primary address, e.g.
	// https://acme.messora.tech (no trailing slash, no /api/v1).
	BaseURL string
	// RequestURL is the origin the document was requested at, when it is
	// another of the company's addresses (a custom domain or the
	// sub-domain); it is listed first, the primary address second.
	RequestURL string
}

// APIBase returns the workspace's API base URL (BaseURL + "/api/v1").
func (w Workspace) APIBase() string { return strings.TrimRight(w.BaseURL, "/") + "/api/v1" }

// introData is what intro.md sees.
type introData struct {
	APIKeyPerMinute int
	// BaseURL is the API base used in the examples.
	BaseURL string
	// Workspace is the company's name ("" in the generic document).
	Workspace string
}

// workspaceOrigin is the origin pattern of a workspace's sub-domain.
func (o Options) workspaceOrigin() string {
	if o.WorkspaceOrigin != "" {
		return strings.TrimRight(o.WorkspaceOrigin, "/")
	}
	return "https://{workspace}.messora.tech"
}

// genericBase is the placeholder address in the introduction of the
// document served outside a workspace (platform hosts, the generated
// bin/openapi.public.json).
func (o Options) genericBase() string {
	return strings.Replace(o.workspaceOrigin(), "{workspace}", "<workspace>", 1) + "/api/v1"
}

// genericServers name the address pattern with a server variable.
func (o Options) genericServers() []any {
	return []any{map[string]any{
		"url":         o.workspaceOrigin() + "/api/v1",
		"description": "Your workspace's own API address (or its custom domain)",
		"variables": map[string]any{"workspace": map[string]any{
			"default": "acme", "description": "Your workspace's sub-domain",
		}},
	}}
}

// ForWorkspace returns pub (a document made by Public) for w: its servers
// are the workspace's own API address(es) and the introduction speaks about
// that workspace's keys. pub is not modified; only the top-level fields
// that differ are replaced.
func ForWorkspace(pub Doc, o Options, w Workspace) Doc {
	out := maps.Clone(pub)
	// The address the reader opened comes first: Scalar sends "try it"
	// requests to the first server, and the panel's CSP allows only its own
	// origin. Another address of the same workspace follows.
	base := w.APIBase()
	servers := []any{map[string]any{"url": base, "description": fmt.Sprintf("The API of %s", w.Name)}}
	if w.RequestURL != "" && strings.TrimRight(w.RequestURL, "/") != strings.TrimRight(w.BaseURL, "/") {
		base = Workspace{BaseURL: w.RequestURL}.APIBase()
		servers = []any{
			map[string]any{"url": base, "description": fmt.Sprintf("The API of %s at this address", w.Name)},
			map[string]any{"url": w.APIBase(), "description": fmt.Sprintf("The API of %s at its primary address", w.Name)},
		}
	}
	out["servers"] = servers
	info := maps.Clone(asMap(pub["info"]))
	if info == nil {
		info = map[string]any{}
	}
	info["description"] = renderIntro(introData{APIKeyPerMinute: o.APIKeyPerMinute, BaseURL: base, Workspace: w.Name})
	out["info"] = info
	if comps, ok := pub["components"].(map[string]any); ok {
		c := maps.Clone(comps)
		if ss, ok := c["securitySchemes"].(map[string]any); ok {
			ss2 := maps.Clone(ss)
			if k, ok := ss2[SchemeAPIKey].(map[string]any); ok {
				k2 := maps.Clone(k)
				k2["description"] = fmt.Sprintf("An API key of the **%s** workspace, created under **API keys** in its panel: "+
					"`Authorization: Bearer msk_live_…`. Keys work only at this workspace's address (%s); "+
					"the key's scopes decide which operations it may call.", w.Name, w.APIBase())
				ss2[SchemeAPIKey] = k2
			}
			c["securitySchemes"] = ss2
		}
		out["components"] = c
	}
	return out
}

// Published serves the public document: the generic one outside a
// workspace and one per workspace address, rendered on first use and kept
// for a few minutes (a renamed company or a new primary domain shows up
// within the TTL).
type Published struct {
	pub     Doc
	o       Options
	generic []byte
	ttl     time.Duration

	mu    sync.Mutex
	cache map[Workspace]cached
}

type cached struct {
	b   []byte
	exp time.Time
}

// maxCachedWorkspaces bounds the per-workspace cache.
const maxCachedWorkspaces = 2000

// NewPublished derives the public document from raw (the full spec).
func NewPublished(raw []byte, o Options) (*Published, error) {
	full, err := Load(raw)
	if err != nil {
		return nil, err
	}
	pub, err := Public(full, o)
	if err != nil {
		return nil, err
	}
	g, err := marshal(pub)
	if err != nil {
		return nil, err
	}
	return &Published{pub: pub, o: o, generic: g, ttl: 5 * time.Minute, cache: map[Workspace]cached{}}, nil
}

// Generic returns the document served outside a workspace.
func (p *Published) Generic() []byte { return p.generic }

// For returns the document of w.
func (p *Published) For(w Workspace) ([]byte, error) {
	now := time.Now()
	p.mu.Lock()
	if c, ok := p.cache[w]; ok && now.Before(c.exp) {
		p.mu.Unlock()
		return c.b, nil
	}
	p.mu.Unlock()
	b, err := marshal(ForWorkspace(p.pub, p.o, w))
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	if len(p.cache) >= maxCachedWorkspaces {
		clear(p.cache)
	}
	p.cache[w] = cached{b: b, exp: now.Add(p.ttl)}
	p.mu.Unlock()
	return b, nil
}

func marshal(d Doc) ([]byte, error) {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("apidocs: %w", err)
	}
	return append(b, '\n'), nil
}
