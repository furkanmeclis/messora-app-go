package whatsapp_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
)

func newClient(t *testing.T, m *whatsapptest.Server, timeout time.Duration) *whatsapp.Client {
	t.Helper()
	c, err := whatsapp.New(whatsapp.Options{BaseURL: m.URL, Timeout: timeout})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func text(to string) whatsapp.Message {
	return whatsapp.Message{To: to, Type: "text", Text: &whatsapp.Text{Body: "hello"}, BizOpaqueCallbackData: "m:123"}
}

func TestSendSuccessCarriesCallbackData(t *testing.T) {
	m := whatsapptest.New()
	defer m.Close()
	c := newClient(t, m, 5*time.Second)
	res, err := c.SendMessage(context.Background(), "PN1", "tok", text("+905551112233"), 0)
	if err != nil || res.MessageID == "" || res.WaID != "905551112233" {
		t.Fatalf("send: %+v %v", res, err)
	}
	sent := m.Sent()
	if len(sent) != 1 || sent[0].Message.MessagingProduct != "whatsapp" || sent[0].Message.BizOpaqueCallbackData != "m:123" || sent[0].Token != "tok" {
		t.Fatalf("request not as expected: %+v", sent)
	}
}

func TestSendOutcomeClassification(t *testing.T) {
	cases := []struct {
		name    string
		b       whatsapptest.Behavior
		timeout time.Duration
		want    whatsapp.Outcome
		account bool
	}{
		{"throughput", whatsapptest.Behavior{ErrorCode: 130429, HTTPStatus: 429}, 5 * time.Second, whatsapp.Retryable, false},
		{"pair rate", whatsapptest.Behavior{ErrorCode: 131056}, 5 * time.Second, whatsapp.Retryable, false},
		{"unknown meta error", whatsapptest.Behavior{ErrorCode: 131000, HTTPStatus: 500}, 5 * time.Second, whatsapp.Retryable, false},
		{"outside window", whatsapptest.Behavior{ErrorCode: 131047}, 5 * time.Second, whatsapp.Permanent, false},
		{"invalid recipient", whatsapptest.Behavior{ErrorCode: 131026}, 5 * time.Second, whatsapp.Permanent, false},
		{"template mismatch", whatsapptest.Behavior{ErrorCode: 132000}, 5 * time.Second, whatsapp.Permanent, false},
		{"expired token", whatsapptest.Behavior{ErrorCode: 190, HTTPStatus: 401}, 5 * time.Second, whatsapp.Permanent, true},
		{"timeout after sending", whatsapptest.Behavior{Delay: 2 * time.Second}, 300 * time.Millisecond, whatsapp.Ambiguous, false},
		{"connection dropped", whatsapptest.Behavior{DropConnection: true}, 5 * time.Second, whatsapp.Ambiguous, false},
		{"gateway 502 without body", whatsapptest.Behavior{BareStatus: 502}, 5 * time.Second, whatsapp.Ambiguous, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := whatsapptest.New()
			defer m.Close()
			m.Script(tc.b)
			_, err := newClient(t, m, tc.timeout).SendMessage(context.Background(), "PN1", "tok", text("+1555"), 1)
			se, ok := whatsapp.AsSendError(err)
			if !ok {
				t.Fatalf("want SendError, got %v", err)
			}
			if se.Outcome != tc.want || se.AccountProblem != tc.account {
				t.Fatalf("outcome=%s account=%v, want %s/%v (%v)", se.Outcome, se.AccountProblem, tc.want, tc.account, err)
			}
			if se.Outcome == whatsapp.Retryable && se.RetryAfter <= 0 {
				t.Fatal("retryable errors must suggest a delay")
			}
		})
	}
}

func TestConnectionRefusedIsSafeToRetry(t *testing.T) {
	m := whatsapptest.New()
	url := m.URL
	m.Close() // nothing listens: the request is never written
	c, _ := whatsapp.New(whatsapp.Options{BaseURL: url, Timeout: time.Second})
	_, err := c.SendMessage(context.Background(), "PN1", "tok", text("+1555"), 0)
	se, ok := whatsapp.AsSendError(err)
	if !ok || se.Outcome != whatsapp.Retryable {
		t.Fatalf("want retryable, got %v", err)
	}
}

func TestDryRunBlocksMeta(t *testing.T) {
	c, err := whatsapp.New(whatsapp.Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.SendMessage(context.Background(), "PN1", "tok", text("+1555"), 0)
	if !errors.Is(err, whatsapp.ErrDryRun) {
		t.Fatalf("dry-run must refuse graph.facebook.com, got %v", err)
	}
	se, _ := whatsapp.AsSendError(err)
	if se.Outcome != whatsapp.Permanent {
		t.Fatal("a dry-run refusal must never be retried")
	}
}

func TestAccountTemplatesAndMedia(t *testing.T) {
	ctx := context.Background()
	m := whatsapptest.New()
	defer m.Close()
	m.AddPhoneNumber("WABA1", whatsapp.PhoneNumber{ID: "PN1", DisplayPhoneNumber: "+1 555-0100", VerifiedName: "Acme", QualityRating: "GREEN"})
	m.AddTemplate("WABA1", whatsapp.TemplateInfo{ID: "T1", Name: "welcome", Language: "en", Category: "UTILITY", Status: "APPROVED"})
	c := newClient(t, m, 5*time.Second)

	nums, err := c.ListPhoneNumbers(ctx, "WABA1", "tok")
	if err != nil || len(nums) != 1 || nums[0].VerifiedName != "Acme" {
		t.Fatalf("numbers: %+v %v", nums, err)
	}
	if err := c.RegisterPhoneNumber(ctx, "PN1", "tok", "123456"); err != nil {
		t.Fatal(err)
	}
	tpls, err := c.ListTemplates(ctx, "WABA1", "tok")
	if err != nil || len(tpls) != 1 || tpls[0].Status != "APPROVED" {
		t.Fatalf("templates: %+v %v", tpls, err)
	}
	id, status, err := c.CreateTemplate(ctx, "WABA1", "tok", []byte(`{"name":"promo","language":"en","category":"MARKETING","components":[]}`))
	if err != nil || id == "" || status != "PENDING" {
		t.Fatalf("create: %s %s %v", id, status, err)
	}
	if err := c.DeleteTemplate(ctx, "WABA1", "tok", "promo", ""); err != nil {
		t.Fatal(err)
	}

	mid, err := c.UploadMedia(ctx, "PN1", "tok", "a.txt", "text/plain", stringReader("file-content"))
	if err != nil || mid == "" {
		t.Fatalf("upload: %v", err)
	}
	info, err := c.GetMedia(ctx, mid, "tok")
	if err != nil || info.URL == "" || info.MimeType != "text/plain" {
		t.Fatalf("get media: %+v %v", info, err)
	}
	var buf bytesBuffer
	if _, err := c.DownloadMedia(ctx, info.URL, "tok", &buf, 1<<20); err != nil || buf.String() != "file-content" {
		t.Fatalf("download: %q %v", buf.String(), err)
	}
	if _, err := c.DownloadMedia(ctx, info.URL, "tok", &buf, 3); err == nil {
		t.Fatal("size limit not enforced")
	}
}

func TestWebhookSignatureAndChallenge(t *testing.T) {
	body := []byte(`{"object":"whatsapp_business_account"}`)
	sig := whatsapp.Sign(body, "secret-1")
	if !whatsapp.VerifySignature(body, sig, "other", "secret-1") {
		t.Fatal("valid signature rejected")
	}
	if whatsapp.VerifySignature(append(body, ' '), sig, "secret-1") || whatsapp.VerifySignature(body, sig, "wrong") ||
		whatsapp.VerifySignature(body, "sha1=abc", "secret-1") || whatsapp.VerifySignature(body, sig) {
		t.Fatal("invalid signature accepted")
	}
	if c, ok := whatsapp.VerifyChallenge("subscribe", "tok", "42", "tok"); !ok || c != "42" {
		t.Fatal("challenge")
	}
	for _, in := range [][3]string{{"subscribe", "nope", "42"}, {"unsubscribe", "tok", "42"}, {"subscribe", "tok", ""}} {
		if _, ok := whatsapp.VerifyChallenge(in[0], in[1], in[2], "tok"); ok {
			t.Fatalf("accepted %v", in)
		}
	}
	if _, ok := whatsapp.VerifyChallenge("subscribe", "", "42", ""); ok {
		t.Fatal("empty configured token must never verify")
	}
}

type bytesBuffer = strings.Builder

func stringReader(s string) *strings.Reader { return strings.NewReader(s) }
