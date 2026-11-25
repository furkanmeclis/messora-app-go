package consent

import (
	"strings"
	"unicode"
)

// Keyword kinds.
const (
	KindStop    = "stop"    // an exact stop word: opted out of marketing at once
	KindRequest = "request" // a phrase asking to stop: opt-out requested, an agent confirms
	KindResume  = "resume"  // an exact resubscribe word
)

// Keywords are the company's lists. Stop and resume words match the whole
// message; request phrases match anywhere in it (on word boundaries).
type Keywords struct {
	Stop    []string `json:"stop"`
	Request []string `json:"request"`
	Resume  []string `json:"resume"`
}

// DefaultKeywords is the initial Turkish and English list.
func DefaultKeywords() Keywords {
	return Keywords{
		Stop: []string{"İPTAL", "STOP", "DUR", "ABONELİKTEN ÇIK", "UNSUBSCRIBE"},
		Request: []string{"mesaj istemiyorum", "bana tekrar yazmayın", "artık mesaj almak istemiyorum",
			"abonelikten çıkmak istiyorum", "stop sending me messages", "do not message me"},
		Resume: []string{"BAŞLAT", "START", "ABONE OL"},
	}
}

// maxPhraseMessage bounds the messages scanned for request phrases: a long
// message quoting a phrase is not an opt-out request.
const maxPhraseMessage = 300

// fold maps letters to a case- and accent-insensitive form. Turkish has a
// dotted and a dotless i in both cases (İ i, I ı); people type either, so
// all four become "i". Turkish letters with marks lose them (ç→c, ğ→g,
// ö→o, ş→s, ü→u) because customers often type without them.
var fold = map[rune]rune{
	'İ': 'i', 'I': 'i', 'ı': 'i', 'i': 'i',
	'Ç': 'c', 'ç': 'c', 'Ğ': 'g', 'ğ': 'g', 'Ö': 'o', 'ö': 'o', 'Ş': 's', 'ş': 's', 'Ü': 'u', 'ü': 'u',
	'Â': 'a', 'â': 'a', 'Î': 'i', 'î': 'i', 'Û': 'u', 'û': 'u',
}

// Normalize folds case and Turkish letters, turns punctuation and symbols
// into spaces, and collapses whitespace: "  İptal!!! " → "iptal".
func Normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := true // drop leading spaces
	for _, r := range s {
		if f, ok := fold[r]; ok {
			r = f
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			r = unicode.ToLower(r)
		} else if unicode.Is(unicode.Mn, r) {
			continue // a combining mark (decomposed input): İ written as I + U+0307
		} else {
			r = ' '
		}
		if r == ' ' {
			if space {
				continue
			}
			space = true
		} else {
			space = false
		}
		b.WriteRune(r)
	}
	return strings.TrimRight(b.String(), " ")
}

// Match classifies an inbound text: a stop word, a resubscribe word, a
// request phrase, or "" for none. Exact words win over phrases.
func (k Keywords) Match(text string) string {
	n := Normalize(text)
	if n == "" {
		return ""
	}
	for _, w := range k.Stop {
		if Normalize(w) == n {
			return KindStop
		}
	}
	for _, w := range k.Resume {
		if Normalize(w) == n {
			return KindResume
		}
	}
	if len(n) > maxPhraseMessage {
		return ""
	}
	padded := " " + n + " "
	for _, p := range k.Request {
		if p = Normalize(p); p != "" && strings.Contains(padded, " "+p+" ") {
			return KindRequest
		}
	}
	return ""
}
