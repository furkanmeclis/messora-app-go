package license

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGateInterop runs the Cloudflare Worker (deployments/license-gate)
// under Node: it must accept keys signed here, and its answers must verify
// here.
func TestGateInterop(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed")
	}
	licPub, licPriv, _ := ed25519.GenerateKey(rand.Reader)
	gatePub, gatePriv, _ := ed25519.GenerateKey(rand.Reader)
	key, err := IssueLicense(Payload{ID: "lic_x", KeyID: "k1", Customer: "Acme", IssuedAt: time.Now().UTC(), CheckIn: CheckInOnline}, licPriv)
	if err != nil {
		t.Fatal(err)
	}
	gate, _ := filepath.Abs("../../../deployments/license-gate/src/gate.js")
	script := `
const { default: gate } = await import(process.argv[1]);
const [ , , licPub, gateSeed, key] = process.argv;
const kv = new Map();
const env = {
  LICENSE_KEYS: JSON.stringify({ k1: licPub }),
  GATE_KEY: JSON.stringify({ kid: 'g1', seed: gateSeed }),
  KV: { get: async (k) => kv.get(k) ?? null, put: async (k, v) => void kv.set(k, v) },
};
const res = await gate.fetch(new Request('https://gate/v1/check-in', { method: 'POST',
  body: JSON.stringify({ key, instance_id: '0b0c1f0e-3c3a-4f5e-9d6f-1a2b3c4d5e6f', version: 'test' }) }), env, { waitUntil() {} });
process.stdout.write(await res.text());
`
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "--input-type=module", "-e", script, "file://"+gate,
		base64.StdEncoding.EncodeToString(licPub), base64.StdEncoding.EncodeToString(gatePriv.Seed()), key)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	a, err := OpenAnswer(strings.TrimSpace(string(out)), Keyring{"g1": gatePub})
	if err != nil {
		t.Fatalf("answer %q: %v", out, err)
	}
	if a.Status != "ok" || a.LicenseID != "lic_x" || a.InstanceID != "0b0c1f0e-3c3a-4f5e-9d6f-1a2b3c4d5e6f" {
		t.Fatalf("answer: %+v", a)
	}
}
