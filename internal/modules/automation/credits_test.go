package automation

import (
	"strings"
	"testing"

	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
)

// AI steps without credits (or without AI in the plan) stop with a clear,
// non-retryable reason (Phase 17 WP8).
func TestAIFailureWithoutCreditsOrPlan(t *testing.T) {
	e := aiFailure(ai.ErrCreditsExhausted)
	if e.Code != CodeAICreditsExhausted || e.Retryable || !strings.Contains(e.Message, "credits") {
		t.Fatalf("credits: %+v", e)
	}
	e = aiFailure(ai.ErrNotInPlan)
	if e.Code != CodeAINotInPlan || e.Retryable {
		t.Fatalf("plan: %+v", e)
	}
	if e := aiFailure(ai.ErrUnavailable); e.Code != CodeAIError || !e.Retryable {
		t.Fatalf("unavailable stays retryable: %+v", e)
	}
}
