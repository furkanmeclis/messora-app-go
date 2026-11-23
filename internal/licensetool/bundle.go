// Package licensetool is the issuer side of Messora licensing: the signing
// keys, the ledger of issued licences and the encrypted backup bundle.
package licensetool

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/scrypt"
)

const bundleMagic = "MSLB1"

// Bundle is everything needed to keep issuing licences on another machine.
type Bundle struct {
	Version     int               `json:"version"`
	CreatedAt   time.Time         `json:"created_at"`
	SigningKeys map[string]string `json:"signing_keys"` // kid -> base64 seed
	GateKeys    map[string]string `json:"gate_keys"`    // kid -> base64 seed
	Config      Config            `json:"config"`
	Ledger      []Entry           `json:"ledger"`
}

// scrypt parameters: ~0.5 s on a laptop, 128 MiB.
const (
	scryptN = 1 << 17
	scryptR = 8
	scryptP = 1
)

// Seal encrypts the bundle with a passphrase.
func Seal(b Bundle, passphrase string) ([]byte, error) {
	if len(passphrase) < 12 {
		return nil, errors.New("passphrase must be at least 12 characters")
	}
	plain, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	gcm, err := aead(passphrase, salt)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append(append(salt, nonce...), gcm.Seal(nil, nonce, plain, []byte(bundleMagic))...)
	return []byte(bundleMagic + "\n" + base64.StdEncoding.EncodeToString(out) + "\n"), nil
}

// ErrBadPassphrase means the bundle could not be decrypted.
var ErrBadPassphrase = errors.New("wrong passphrase or damaged backup")

// Open decrypts a bundle.
func Open(data []byte, passphrase string) (Bundle, error) {
	var b Bundle
	head, body, ok := strings.Cut(strings.TrimSpace(string(data)), "\n")
	if !ok || head != bundleMagic {
		return b, errors.New("not a Messora licence backup")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(body))
	if err != nil || len(raw) < 16+12+16 {
		return b, ErrBadPassphrase
	}
	gcm, err := aead(passphrase, raw[:16])
	if err != nil {
		return b, err
	}
	n := gcm.NonceSize()
	plain, err := gcm.Open(nil, raw[16:16+n], raw[16+n:], []byte(bundleMagic))
	if err != nil {
		return b, ErrBadPassphrase
	}
	if err := json.Unmarshal(plain, &b); err != nil {
		return b, ErrBadPassphrase
	}
	return b, nil
}

func aead(passphrase string, salt []byte) (cipher.AEAD, error) {
	key, err := scrypt.Key([]byte(passphrase), salt, scryptN, scryptR, scryptP, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
