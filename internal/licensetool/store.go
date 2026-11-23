package licensetool

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Keychain services (login keychain of the issuer's Mac).
const (
	SigningService = "Messora License Signing Key"
	GateService    = "Messora License Gate Key"
)

// Config is the issuer's non-secret settings.
type Config struct {
	SigningKID string     `json:"signing_kid"`
	GateKID    string     `json:"gate_kid"`
	Cloudflare Cloudflare `json:"cloudflare"`
}

// Cloudflare locates the licence gate.
type Cloudflare struct {
	AccountID     string `json:"account_id,omitempty"`
	WorkerName    string `json:"worker_name,omitempty"`
	WorkerURL     string `json:"worker_url,omitempty"`
	KVNamespaceID string `json:"kv_namespace_id,omitempty"`
	R2Bucket      string `json:"r2_bucket,omitempty"`
}

// Entry is one ledger line.
type Entry struct {
	At         time.Time `json:"at"`
	Action     string    `json:"action"` // issued, revoked, superseded
	LicenseID  string    `json:"license_id"`
	Customer   string    `json:"customer,omitempty"`
	Plan       string    `json:"plan,omitempty"`
	Key        string    `json:"key,omitempty"` // issued keys, so they can be re-sent
	ReplacedBy string    `json:"replaced_by,omitempty"`
	Note       string    `json:"note,omitempty"`
}

// Store is the issuer's state directory plus the Keychain.
type Store struct {
	Dir string
	// Keychain reads and writes secrets; tests swap it.
	Keychain Keychain
}

// Keychain stores secrets by service and account.
type Keychain interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
}

// DefaultDir is ~/Library/Application Support/Messora License.
func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Messora License")
}

// ICloudDir is the iCloud Drive folder used for backups, if iCloud Drive
// is on.
func ICloudDir() string {
	home, _ := os.UserHomeDir()
	root := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs")
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return ""
	}
	return filepath.Join(root, "Messora")
}

func (s Store) path(name string) string { return filepath.Join(s.Dir, name) }

// LoadConfig reads config.json (empty when missing).
func (s Store) LoadConfig() (Config, error) {
	var c Config
	b, err := os.ReadFile(s.path("config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}

// SaveConfig writes config.json.
func (s Store) SaveConfig(c Config) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(s.path("config.json"), append(b, '\n'), 0o600)
}

// Append adds a ledger line.
func (s Store) Append(e Entry) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path("ledger.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, _ := json.Marshal(e)
	_, err = f.Write(append(b, '\n'))
	return err
}

// Ledger reads every ledger line.
func (s Store) Ledger() ([]Entry, error) {
	b, err := os.ReadFile(s.path("ledger.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("ledger: %w", err)
		}
		out = append(out, e)
	}
	return out, sc.Err()
}

// PrivateKey loads a key from the Keychain.
func (s Store) PrivateKey(service, kid string) (ed25519.PrivateKey, error) {
	if kid == "" {
		return nil, errors.New("no key yet: run keygen first")
	}
	v, err := s.Keychain.Get(service, kid)
	if err != nil {
		return nil, fmt.Errorf("key %s not in the Keychain (restore a backup?): %w", kid, err)
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(v))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("key %s in the Keychain is damaged", kid)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// SaveKey stores a key's seed in the Keychain.
func (s Store) SaveKey(service, kid string, key ed25519.PrivateKey) error {
	return s.Keychain.Set(service, kid, base64.StdEncoding.EncodeToString(key.Seed()))
}

// MacKeychain uses the macOS `security` tool. Secrets go through stdin,
// never through the command line (visible to other processes).
type MacKeychain struct{}

// Get implements Keychain.
func (MacKeychain) Get(service, account string) (string, error) {
	out, err := exec.CommandContext(context.Background(), "security", "find-generic-password", "-s", service, "-a", account, "-w").Output() //nolint:gosec // fixed tool, arguments are not a shell
	if err != nil {
		return "", errors.New("not found")
	}
	return strings.TrimSpace(string(out)), nil
}

// Set implements Keychain.
func (MacKeychain) Set(service, account, secret string) error {
	for _, v := range []string{service, account, secret} {
		if strings.ContainsAny(v, "\"\n\\") {
			return errors.New("keychain: unsupported characters")
		}
	}
	cmd := exec.CommandContext(context.Background(), "security", "-i")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("add-generic-password -U -s \"%s\" -a \"%s\" -l \"%s\" -w \"%s\"\n", service, account, service, secret))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("keychain: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// FileKeychain keeps secrets in plain files. Only for tests and dry runs
// (MESSORA_LICENSE_TEST_KEYCHAIN); never use it for real keys.
type FileKeychain struct{ Dir string }

func (f FileKeychain) file(service, account string) string {
	return filepath.Join(f.Dir, base64.RawURLEncoding.EncodeToString([]byte(service+"\x00"+account)))
}

// Get implements Keychain.
func (f FileKeychain) Get(service, account string) (string, error) {
	b, err := os.ReadFile(f.file(service, account))
	return string(b), err
}

// Set implements Keychain.
func (f FileKeychain) Set(service, account, secret string) error {
	if err := os.MkdirAll(f.Dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(f.file(service, account), []byte(secret), 0o600)
}
