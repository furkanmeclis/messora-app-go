package app

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/platform/apidocs"
	"github.com/furkanmeclis/messora-app-go/internal/platform/apiguard"
)

// The published endpoint list matches the server's real routes one to one
// (phase 16C acceptance 21): every operation in the public document is a
// registered route, and every registered route an API key may call is in
// the public document.
func TestPublicDocumentMatchesRoutes(t *testing.T) {
	r := chi.NewRouter()
	// Registration only stores the handler; nothing is called.
	api.HandlerWithOptions(apiServer{}, api.ChiServerOptions{BaseURL: "/api/v1", BaseRouter: r})

	raw, err := api.GetSpecJSON()
	if err != nil {
		t.Fatal(err)
	}
	full, err := apidocs.Load(raw)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := apidocs.Public(full, apidocs.Options{Version: "test", APIKeyPerMinute: apiguard.APIKeyPerMinute})
	if err != nil {
		t.Fatal(err)
	}
	specOps := map[string]map[string]any{}
	for _, o := range apidocs.Operations(full) {
		specOps[strings.ToUpper(o.Method)+" /api/v1"+o.Path] = o.Op
	}
	var documented []string
	for _, o := range apidocs.Operations(pub) {
		documented = append(documented, strings.ToUpper(o.Method)+" /api/v1"+o.Path)
	}

	var routes, publicRoutes []string
	err = chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		id := method + " " + route
		routes = append(routes, id)
		op, ok := specOps[id]
		if !ok {
			t.Errorf("route %s is not in the OpenAPI spec", id)
			return nil
		}
		if apidocs.IsPublic(op) {
			publicRoutes = append(publicRoutes, id)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(publicRoutes)
	slices.Sort(documented)
	for _, d := range documented {
		if !slices.Contains(routes, d) {
			t.Errorf("documented %s has no route", d)
		}
	}
	if !slices.Equal(publicRoutes, documented) {
		t.Fatalf("API key routes and public document differ:\nroutes     %v\ndocumented %v", publicRoutes, documented)
	}
	if len(documented) == 0 {
		t.Fatal("no public operations")
	}
}
