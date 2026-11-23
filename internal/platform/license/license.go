// Package license verifies Messora licence keys and decides what an
// installation may do.
//
// A key is "msl1.<payload>.<signature>": the payload is base64url JSON and
// the signature is Ed25519 over "msl1.<payload>". The signing key never
// leaves the issuer's machine; installations only carry public keys.
// Check-in answers from the licence gate use the same envelope with the
// prefix "msg1" and the gate's own key.
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Prefixes of the two signed envelopes.
const (
	LicensePrefix = "msl1"
	GatePrefix    = "msg1"
)

// Check-in modes.
const (
	CheckInOnline  = "online"
	CheckInOffline = "offline"
)

// Limits caps an installation; zero means unlimited.
type Limits struct {
	Users        int `json:"users,omitempty"`
	PhoneNumbers int `json:"phone_numbers,omitempty"`
}

// Payload is what a licence key says.
type Payload struct {
	V            int        `json:"v"`
	ID           string     `json:"id"`
	KeyID        string     `json:"kid"`
	Customer     string     `json:"customer"`
	Plan         string     `json:"plan"`
	IssuedAt     time.Time  `json:"iat"`
	ExpiresAt    *time.Time `json:"exp,omitempty"` // nil: never
	UpdatesUntil *time.Time `json:"upd,omitempty"` // nil: all updates
	CheckIn      string     `json:"chk"`
	Limits       Limits     `json:"lim"`
}

// Keyring maps key ids to public keys.
type Keyring map[string]ed25519.PublicKey

// Errors of Parse.
var (
	ErrMalformed  = errors.New("license: malformed key")
	ErrSignature  = errors.New("license: signature does not match")
	ErrUnknownKey = errors.New("license: signed with an unknown key")
)

var b64 = base64.RawURLEncoding

// Sign creates an envelope (prefix "msl1" or "msg1") for v; the key id
// travels inside the payload.
func Sign(prefix string, v any, key ed25519.PrivateKey) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	head := prefix + "." + b64.EncodeToString(raw)
	sig := ed25519.Sign(key, []byte(head))
	return head + "." + b64.EncodeToString(sig), nil
}

// Open verifies an envelope and decodes its payload into v. kidOf extracts
// the key id from the decoded payload.
func Open(prefix, token string, keys Keyring, v any, kidOf func() string) error {
	token = strings.TrimSpace(token)
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != prefix {
		return ErrMalformed
	}
	raw, err := b64.DecodeString(parts[1])
	if err != nil {
		return ErrMalformed
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil || len(sig) != ed25519.SignatureSize {
		return ErrMalformed
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return ErrMalformed
	}
	pub, ok := keys[kidOf()]
	if !ok || len(pub) != ed25519.PublicKeySize {
		return ErrUnknownKey
	}
	if !ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig) {
		return ErrSignature
	}
	return nil
}

// Parse verifies a licence key.
func Parse(token string, keys Keyring) (Payload, error) {
	var p Payload
	if err := Open(LicensePrefix, token, keys, &p, func() string { return p.KeyID }); err != nil {
		return Payload{}, err
	}
	if p.V != 1 || p.ID == "" || p.IssuedAt.IsZero() {
		return Payload{}, fmt.Errorf("%w: missing fields", ErrMalformed)
	}
	if p.CheckIn != CheckInOnline && p.CheckIn != CheckInOffline {
		return Payload{}, fmt.Errorf("%w: unknown check-in mode", ErrMalformed)
	}
	return p, nil
}

// IssueLicense signs a licence payload.
func IssueLicense(p Payload, key ed25519.PrivateKey) (string, error) {
	if p.V == 0 {
		p.V = 1
	}
	return Sign(LicensePrefix, p, key)
}

// ParsePublicKey decodes a base64 (std or url) Ed25519 public key.
func ParsePublicKey(s string) (ed25519.PublicKey, error) {
	s = strings.TrimSpace(s)
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		b, err = b64.DecodeString(s)
	}
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil, errors.New("license: invalid public key")
	}
	return ed25519.PublicKey(b), nil
}
