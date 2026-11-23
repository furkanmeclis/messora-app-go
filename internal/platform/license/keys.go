package license

import "crypto/ed25519"

// Trusted public keys, compiled in. LicenseKeys verify licence keys (the
// private half stays on the issuer's Mac, see cmd/messora-license);
// GateKeys verify check-in answers (the private half is a Cloudflare Worker
// secret). Keys are never read from the environment: that would let anyone
// sign their own licence. `messora-license pubkeys` prints these lines.
var (
	LicenseKeys = Keyring{
		"k261123_80340919f23a": mustKey("R5VgSnYXmiJwsIj+iCKs0FqnPPF5wsf5mIOL1Rc2Bo4="),
	}
	GateKeys = Keyring{
		"g261123_c150a0103328": mustKey("dedGjANex7WnwcCDYnt11Zjbr1Yguwy5WVe52DoguxM="),
	}
)

// DefaultGateURL is the licence gate installations check in with.
const DefaultGateURL = "https://messora-license-gate.messora-license-gate.workers.dev"

// mustKey decodes a compiled-in public key.
func mustKey(s string) ed25519.PublicKey {
	k, err := ParsePublicKey(s)
	if err != nil {
		panic(err)
	}
	return k
}
