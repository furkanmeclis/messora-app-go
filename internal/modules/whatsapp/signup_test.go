package whatsapp_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
)

// The Embedded Signup calls of a Tech Provider against the fake Meta.
func TestEmbeddedSignupCalls(t *testing.T) {
	m := whatsapptest.New()
	defer m.Close()
	m.SetApp("77", "secret-of-app-77")
	c := newClient(t, m, 5*time.Second)
	ctx := context.Background()
	code, token := m.AddSignup(whatsapptest.Signup{AppID: "77", WABA: "500", PhoneNumberID: "501", Display: "+90 555", VerifiedName: "Acme"})

	if _, err := c.ExchangeCode(ctx, "77", "wrong", code); err == nil {
		t.Fatal("a wrong app secret traded the code")
	}
	got, err := c.ExchangeCode(ctx, "77", "secret-of-app-77", code)
	if err != nil || got != token {
		t.Fatalf("exchange: %q %v", got, err)
	}
	// Single use.
	var api *whatsapp.APIError
	if _, err := c.ExchangeCode(ctx, "77", "secret-of-app-77", code); !errors.As(err, &api) {
		t.Fatalf("second exchange: %v", err)
	}
	info, err := c.DebugToken(ctx, token, "77|secret-of-app-77")
	if err != nil || !info.Valid || info.AppID != "77" || !slices.Contains(info.Targets["whatsapp_business_management"], "500") {
		t.Fatalf("debug_token: %+v %v", info, err)
	}
	if _, err := c.DebugToken(ctx, token, "77|wrong"); err == nil {
		t.Fatal("debug_token accepted a wrong app token")
	}
	if err := c.SubscribeApp(ctx, "500", token); err != nil || !slices.Equal(m.Subscribed("500"), []string{"77"}) {
		t.Fatalf("subscribe: %v %v", err, m.Subscribed("500"))
	}
	if err := c.RegisterPhoneNumber(ctx, "501", token, "123456"); err != nil || m.Registered("501") != "123456" {
		t.Fatalf("register: %v", err)
	}
	if n, err := c.GetPhoneNumber(ctx, "501", token); err != nil || n.Status != "CONNECTED" {
		t.Fatalf("registered number: %+v %v", n, err)
	}
	if err := c.UnsubscribeApp(ctx, "500", token); err != nil || len(m.Subscribed("500")) != 0 {
		t.Fatalf("unsubscribe: %v %v", err, m.Subscribed("500"))
	}
	m.SetAppSubscription("77", whatsapp.AppSubscription{Object: "whatsapp_business_account", CallbackURL: "https://hooks.test/meta/tp", Active: true, Fields: []string{"messages"}})
	subs, err := c.AppSubscriptions(ctx, "77", "77|secret-of-app-77")
	if err != nil || len(subs) != 1 || subs[0].CallbackURL != "https://hooks.test/meta/tp" || !slices.Contains(subs[0].Fields, "messages") {
		t.Fatalf("app subscriptions: %+v %v", subs, err)
	}
	// Scripted failures hit once.
	m.FailOnce("POST", "register", 133005, "PIN mismatch")
	if err := c.RegisterPhoneNumber(ctx, "501", token, "654321"); !errors.As(err, &api) || api.Code != 133005 {
		t.Fatalf("scripted failure: %v", err)
	}
	if err := c.RegisterPhoneNumber(ctx, "501", token, "654321"); err != nil {
		t.Fatalf("after the scripted failure: %v", err)
	}
	// Another Graph API version for the Tech Provider.
	if v := c.WithVersion("v27.0"); v.Version() != "v27.0" || c.Version() == "v27.0" || c.WithVersion("") != c {
		t.Fatal("WithVersion")
	}
}

// The E2E fake Meta provisions the codes of its fake SDK on first use.
func TestAutoSignupHandler(t *testing.T) {
	_, h := whatsapptest.NewAutoSignupHandler("1234", "https://x/meta/tp")
	srv := newServerFor(t, h)
	c, err := whatsapp.New(whatsapp.Options{BaseURL: srv})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tok, err := c.ExchangeCode(ctx, "1234", "any", "e2e-es.600.601")
	if err != nil || tok == "" {
		t.Fatalf("auto signup: %q %v", tok, err)
	}
	nums, err := c.ListPhoneNumbers(ctx, "600", tok)
	if err != nil || len(nums) != 1 || nums[0].ID != "601" {
		t.Fatalf("numbers: %+v %v", nums, err)
	}
	if _, err := c.ExchangeCode(ctx, "1234", "any", "e2e-es.600.601"); err == nil {
		t.Fatal("an auto code worked twice")
	}
	if _, err := c.ExchangeCode(ctx, "1234", "any", "bogus"); err == nil {
		t.Fatal("an unknown code worked")
	}
}

func newServerFor(t *testing.T, h http.Handler) string {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return s.URL
}
