package license

// Trusted public keys, compiled in. LicenseKeys verify licence keys (the
// private half stays on the issuer's machine, see cmd/messora-license);
// GateKeys verify check-in answers (the private half is a Cloudflare Worker
// secret). Keys are never read from the environment: that would let anyone
// sign their own licence.
var (
	LicenseKeys = Keyring{}
	GateKeys    = Keyring{}
)

// DefaultGateURL is the licence gate installations check in with.
const DefaultGateURL = ""
