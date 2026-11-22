package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// NewToken returns prefix + 32 random bytes (base64url, no padding).
func NewToken(prefix string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken returns SHA-256(token). Tokens have 256 bits of entropy, so a
// fast hash is appropriate (no dictionary to attack).
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// RandomDigits returns n uniformly random decimal digits.
func RandomDigits(n int) (string, error) {
	const alphabet = "0123456789"
	return randomFrom(alphabet, n)
}

// RecoveryCode returns a 10-character code like "k7q2m-x9ft4" from an
// unambiguous alphabet.
func RecoveryCode() (string, error) {
	s, err := randomFrom("abcdefghjkmnpqrstuvwxyz23456789", 10)
	if err != nil {
		return "", err
	}
	return s[:5] + "-" + s[5:], nil
}

func randomFrom(alphabet string, n int) (string, error) {
	out := make([]byte, n)
	maxByte := 256 - (256 % len(alphabet)) // rejection sampling: no modulo bias
	buf := make([]byte, 1)
	for i := 0; i < n; {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		if int(buf[0]) >= maxByte {
			continue
		}
		out[i] = alphabet[int(buf[0])%len(alphabet)]
		i++
	}
	return string(out), nil
}

// Sealer encrypts secrets at rest with AES-256-GCM. The ciphertext format is
// version(1) || nonce(12) || ciphertext+tag. Associated data binds a
// ciphertext to its purpose (e.g. "totp:<user_id>") so it cannot be swapped
// into another column.
type Sealer struct {
	version byte
	aead    cipher.AEAD
	older   map[byte]cipher.AEAD
}

// NewSealer builds a sealer from the current 32-byte key. Older keys (for
// rotation) may be supplied with their version numbers.
func NewSealer(key []byte, older map[byte][]byte) (*Sealer, error) {
	cur, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	s := &Sealer{version: 1, aead: cur, older: map[byte]cipher.AEAD{}}
	for v, k := range older {
		a, err := newAEAD(k)
		if err != nil {
			return nil, fmt.Errorf("crypto: key version %d: %w", v, err)
		}
		s.older[v] = a
		if v >= s.version {
			s.version = v + 1
		}
	}
	return s, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("crypto: key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Seal encrypts plaintext bound to ad.
func (s *Sealer) Seal(plaintext []byte, ad string) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, 1+len(nonce)+len(plaintext)+s.aead.Overhead())
	out = append(out, s.version)
	out = append(out, nonce...)
	return s.aead.Seal(out, nonce, plaintext, []byte(ad)), nil
}

// ErrDecrypt is returned for tampered data, a wrong key or wrong ad.
var ErrDecrypt = errors.New("crypto: cannot decrypt")

// Open decrypts data produced by Seal.
func (s *Sealer) Open(data []byte, ad string) ([]byte, error) {
	if len(data) < 1+12+16 {
		return nil, ErrDecrypt
	}
	aead := s.aead
	if data[0] != s.version {
		older, ok := s.older[data[0]]
		if !ok {
			return nil, ErrDecrypt
		}
		aead = older
	}
	ns := aead.NonceSize()
	pt, err := aead.Open(nil, data[1:1+ns], data[1+ns:], []byte(ad))
	if err != nil {
		return nil, ErrDecrypt
	}
	return pt, nil
}

// MAC returns HMAC-SHA256(key, msg), used for request fingerprints.
func MAC(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}
