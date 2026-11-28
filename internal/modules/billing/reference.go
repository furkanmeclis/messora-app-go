package billing

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// Payment references (docs/BILLING.md "Reference"): what a company writes
// in a transfer's description so the payment finds its proforma.
//
//	MSR 4F8K 002C 7
//	│   │    │    └ check character (Luhn mod 32 over the eight before it)
//	│   │    └ the company's proforma counter (base32, 4 characters)
//	│   └ the company's code (base32, 4 characters, random, unique)
//	└ prefix
//
// Characters are Crockford base32 (no I, L, O, U), so a reference read
// over the phone or typed from paper survives: input is upper-cased, O is
// read as 0 and I/L as 1, and spaces, dashes and dots are ignored. The
// check character catches every single wrong character and almost every
// swap of two neighbours.

// RefPrefix starts every reference.
const RefPrefix = "MSR"

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// MaxProformaSeq is the highest proforma counter a reference can hold.
const MaxProformaSeq = 32*32*32*32 - 1

func crockfordValue(c byte) int { return strings.IndexByte(crockford, c) }

// checkChar is the Luhn mod 32 check character of s (Crockford digits).
func checkChar(s string) byte {
	factor, sum := 2, 0
	for i := len(s) - 1; i >= 0; i-- {
		addend := factor * crockfordValue(s[i])
		if factor == 2 {
			factor = 1
		} else {
			factor = 2
		}
		sum += addend/32 + addend%32
	}
	return crockford[(32-sum%32)%32]
}

// encode renders n in base32 with width characters.
func encode(n int64, width int) string {
	b := make([]byte, width)
	for i := width - 1; i >= 0; i-- {
		b[i] = crockford[n%32]
		n /= 32
	}
	return string(b)
}

// Reference builds the reference of a company's seq-th proforma.
func Reference(code string, seq int64) (string, error) {
	if len(code) != 4 || strings.Trim(code, crockford) != "" {
		return "", fmt.Errorf("billing: bad company code %q", code)
	}
	if seq < 1 || seq > MaxProformaSeq {
		return "", fmt.Errorf("billing: proforma counter %d out of range", seq)
	}
	body := code + encode(seq, 4)
	return RefPrefix + body + string(checkChar(body)), nil
}

// DisplayReference groups a reference for reading: MSR-4F8K-002C-7.
func DisplayReference(ref string) string {
	if len(ref) != 12 {
		return ref
	}
	return ref[:3] + "-" + ref[3:7] + "-" + ref[7:11] + "-" + ref[11:]
}

// normalize upper-cases s, reads the look-alikes (O → 0, I and L → 1) and
// drops everything that is not a letter or a digit.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		switch {
		case r == 'O':
			b.WriteByte('0')
		case r == 'I' || r == 'L':
			b.WriteByte('1')
		case r >= '0' && r <= '9', r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// valid reports whether ref (normalized, with prefix) is well formed and
// its check character agrees.
func valid(ref string) bool {
	if len(ref) != 12 || !strings.HasPrefix(ref, RefPrefix) {
		return false
	}
	body := ref[3:11]
	if strings.Trim(ref[3:], crockford) != "" {
		return false
	}
	return checkChar(body) == ref[11]
}

// ParseReference normalizes a typed reference and checks it; ok is false
// for anything that is not a valid reference.
func ParseReference(s string) (string, bool) {
	n := normalize(s)
	if !strings.HasPrefix(n, RefPrefix) {
		n = RefPrefix + n
	}
	return n, valid(n)
}

// FindReferences returns the valid references in a free-text transfer
// description, in order and without repeats.
func FindReferences(text string) []string {
	n := normalize(text)
	var out []string
	for i := 0; i+12 <= len(n); i++ {
		if n[i:i+3] != RefPrefix {
			continue
		}
		if c := n[i : i+12]; valid(c) && !contains(out, c) {
			out = append(out, c)
		}
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// NewCode draws a random company code.
func NewCode() (string, error) {
	b := make([]byte, 4)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(32))
		if err != nil {
			return "", err
		}
		b[i] = crockford[n.Int64()]
	}
	return string(b), nil
}
