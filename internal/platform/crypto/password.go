// Package crypto holds Messora's cryptographic primitives: password hashing,
// random tokens and authenticated encryption of secrets at rest.
package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2Params are the argon2id cost parameters.
type Argon2Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

// DefaultArgon2 follows the OWASP recommendation (m=19 MiB, t=2, p=1) with a
// higher memory cost; hashing takes ~30–60 ms on a typical server core.
var DefaultArgon2 = Argon2Params{MemoryKiB: 64 * 1024, Iterations: 2, Parallelism: 1, SaltLen: 16, KeyLen: 32}

// ErrMismatch means the password does not match the hash.
var ErrMismatch = errors.New("crypto: password mismatch")

// HashPassword returns a PHC-format argon2id hash:
// $argon2id$v=19$m=65536,t=2,p=1$<salt>$<key>
func HashPassword(password string, p Argon2Params) (string, error) {
	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, p.Iterations, p.MemoryKiB, p.Parallelism, p.KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.MemoryKiB, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword checks password against an encoded hash in constant time.
// needsRehash is true when the hash used weaker parameters than want.
func VerifyPassword(password, encoded string, want Argon2Params) (needsRehash bool, err error) {
	p, salt, key, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, p.Iterations, p.MemoryKiB, p.Parallelism, uint32(len(key))) //nolint:gosec // key length comes from our own hash format (32)
	if subtle.ConstantTimeCompare(got, key) != 1 {
		return false, ErrMismatch
	}
	return p.MemoryKiB < want.MemoryKiB || p.Iterations < want.Iterations || len(key) < int(want.KeyLen), nil
}

// dummyHash is verified against when the user does not exist, so that the
// response time does not reveal which e-mail addresses are registered.
var dummyHash, _ = HashPassword("messora-timing-equalizer", DefaultArgon2)

// BurnVerify spends the same time as a real verification.
func BurnVerify(password string) {
	_, _ = VerifyPassword(password, dummyHash, DefaultArgon2)
}

func decodeHash(encoded string) (Argon2Params, []byte, []byte, error) {
	var p Argon2Params
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return p, nil, nil, errors.New("crypto: unsupported hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return p, nil, nil, errors.New("crypto: unsupported argon2 version")
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Iterations, &p.Parallelism); err != nil {
		return p, nil, nil, errors.New("crypto: malformed argon2 parameters")
	}
	if p.MemoryKiB < 8*1024 || p.MemoryKiB > 1<<20 || p.Iterations < 1 || p.Iterations > 20 || p.Parallelism < 1 {
		return p, nil, nil, errors.New("crypto: argon2 parameters out of range")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return p, nil, nil, errors.New("crypto: malformed salt")
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) < 16 || len(key) > 64 {
		return p, nil, nil, errors.New("crypto: malformed key")
	}
	return p, salt, key, nil
}
