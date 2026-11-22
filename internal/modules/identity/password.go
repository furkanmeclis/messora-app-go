package identity

import (
	"strings"
	"unicode/utf8"

	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// commonPasswords are rejected outright (lower-cased). The list covers the
// most frequent leaked passwords that satisfy a 12-character minimum.
var commonPasswords = map[string]bool{}

func init() {
	for p := range strings.FieldsSeq(`
123456789012 1234567890123 12345678901234 123456789123 111111111111 000000000000
qwertyuiop12 qwertyuiopasdf qwertyuiop123 qwerty123456 1qaz2wsx3edc 1q2w3e4r5t6y
password1234 password12345 password123456 passw0rd1234 p@ssw0rd1234 p@ssword1234
iloveyou1234 letmein12345 welcome12345 welcome123456 administrator admin1234567
adminadmin12 abc123456789 abcdefghijkl abcd12345678 aaaaaaaaaaaa qazwsxedcrfv
changeme1234 changemeplease trustno11234 football1234 baseball1234 superman1234
monkey123456 dragon123456 sunshine1234 princess1234 starwars1234 whatsapp1234
messora12345 messora123456 messorapassword password password1 123456789 qwerty
`) {
		commonPasswords[p] = true
	}
}

// validatePassword enforces the password policy beyond the schema's length
// checks: not a common password, not derived from the e-mail address, and
// not trivially repetitive.
func validatePassword(field, password, email string) error {
	n := utf8.RuneCountInString(password)
	if n < 12 {
		return apperr.Validation(field, "Use at least 12 characters")
	}
	if n > 256 {
		return apperr.Validation(field, "Use at most 256 characters")
	}
	lower := strings.ToLower(password)
	if commonPasswords[lower] {
		return apperr.Validation(field, "This password is too common")
	}
	if local, _, ok := strings.Cut(strings.ToLower(email), "@"); ok && len(local) >= 4 && strings.Contains(lower, local) {
		return apperr.Validation(field, "Do not use your e-mail address in your password")
	}
	distinct := map[rune]bool{}
	for _, r := range password {
		distinct[r] = true
	}
	if len(distinct) < 5 {
		return apperr.Validation(field, "Use more varied characters")
	}
	return nil
}
