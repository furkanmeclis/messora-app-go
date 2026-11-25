package ai

import (
	"regexp"
	"strings"
)

// keyLike matches the shapes of provider API keys, so a key that is not the
// one in hand (e.g. echoed in a provider message) is hidden too:
// OpenAI/DeepSeek "sk-…", Anthropic "sk-ant-…", Google "AIza…".
var keyLike = regexp.MustCompile(`(sk-(?:ant-)?[A-Za-z0-9_\-]{12,}|AIza[0-9A-Za-z_\-]{20,})`)

const redacted = "[redacted]"

// scrub removes API keys from text that may be stored (status_detail,
// ai_usage), logged or shown to admins.
func scrub(s string, keys ...string) string {
	for _, k := range keys {
		if len(k) >= 4 {
			s = strings.ReplaceAll(s, k, redacted)
		}
	}
	return keyLike.ReplaceAllString(s, redacted)
}

// Last4 is the part of a key the panel may show.
func Last4(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 8 {
		return ""
	}
	return key[len(key)-4:]
}
