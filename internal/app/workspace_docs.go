package app

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/platform/apidocs"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// workspaceDocs returns the workspace whose address r came to, for the
// public API document: its name, the origin of its primary address and,
// when r came to another of its addresses, that origin too. ok is false on
// platform and unknown hosts (and for suspended workspaces), which get the
// generic document.
func (s *services) workspaceDocs(r *http.Request) (apidocs.Workspace, bool, error) {
	host := tenant.NormalizeHost(tenant.RequestHost(r))
	if host == "" || s.hosts.Kind(host) != tenant.HostTenant {
		return apidocs.Workspace{}, false, nil
	}
	info, err := s.tenants.Resolve(r.Context(), host)
	if err != nil {
		if errors.Is(err, tenant.ErrUnknownHost) || errors.Is(err, tenant.ErrSuspended) {
			return apidocs.Workspace{}, false, nil
		}
		return apidocs.Workspace{}, false, err
	}
	primary, err := s.queries.PrimaryHost(tenant.System(r.Context()), info.ID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		primary = host
	case err != nil:
		return apidocs.Workspace{}, false, err
	}
	w := apidocs.Workspace{Name: strings.TrimSpace(info.Name), BaseURL: s.hosts.URL(primary)}
	if w.Name == "" {
		w.Name = info.Slug
	}
	if primary != host {
		w.RequestURL = s.hosts.URL(host)
	}
	return w, true, nil
}
