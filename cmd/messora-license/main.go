// Command messora-license issues and revokes Messora licences. It runs on
// the issuer's Mac: signing keys live in the login Keychain, the ledger in
// ~/Library/Application Support/Messora License, and `backup` writes an
// encrypted copy of both (plus the Cloudflare settings) to iCloud Drive so
// a new Mac can take over with `restore`.
package main

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/furkanmeclis/messora-app-go/internal/licensetool"
	"github.com/furkanmeclis/messora-app-go/internal/platform/license"
)

const wrangler = "wrangler@4.162.0"

const usage = `messora-license: issue and revoke Messora licences

  keygen [--gate]            create a signing key (or the gate's key) in the Keychain
  pubkeys                    print the public keys to compile into Messora
  issue  --customer NAME [--plan pro] [--expires YYYY-MM-DD|never]
         [--updates-until YYYY-MM-DD|all] [--check-in online|offline]
         [--users N] [--numbers N] [--replaces LICENSE_ID] [--note TEXT]
  list                       show the ledger
  show   LICENSE_ID          print an issued key again
  revoke LICENSE_ID [--note TEXT]
  cloudflare [--account ID] [--worker NAME] [--url URL] [--kv ID] [--bucket NAME]
  backup [FILE]              encrypted backup (default: iCloud Drive/Messora)
  restore FILE               load a backup on this Mac
  gate-secret                upload the gate key to the Worker (secret GATE_KEY)
  gate-deploy [--dir D]      deploy the Worker with the current public keys
  publish VERSION [--channel stable] [--notes TEXT] [--released YYYY-MM-DD]
                             copy a release from GHCR into the R2 registry
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	st := licensetool.Store{Dir: licensetool.DefaultDir(), Keychain: licensetool.MacKeychain{}}
	if d := os.Getenv("MESSORA_LICENSE_DIR"); d != "" {
		st.Dir = d
	}
	if d := os.Getenv("MESSORA_LICENSE_TEST_KEYCHAIN"); d != "" {
		st.Keychain = licensetool.FileKeychain{Dir: d}
	}
	cmds := map[string]func(licensetool.Store, []string) error{
		"keygen": keygen, "pubkeys": pubkeys, "issue": issue, "list": list, "show": show,
		"revoke": revoke, "cloudflare": cloudflare, "backup": backup, "restore": restore,
		"gate-secret": gateSecret, "gate-deploy": gateDeploy, "publish": publish,
	}
	fn, ok := cmds[os.Args[1]]
	if !ok {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := fn(st, os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return prefix + time.Now().UTC().Format("060102") + "_" + hex.EncodeToString(b)
}

func keygen(st licensetool.Store, args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	gate := fs.Bool("gate", false, "create the licence gate's key")
	_ = fs.Parse(args)
	cfg, err := st.LoadConfig()
	if err != nil {
		return err
	}
	service, kid := licensetool.SigningService, newID("k")
	if *gate {
		service, kid = licensetool.GateService, newID("g")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := st.SaveKey(service, kid, priv); err != nil {
		return err
	}
	if *gate {
		cfg.GateKID = kid
	} else {
		cfg.SigningKID = kid
	}
	if err := st.SaveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("Created %s %q in the Keychain.\nPublic key: %s\n", service, kid, base64.StdEncoding.EncodeToString(pub))
	fmt.Println("Run `messora-license backup` now so the key survives this Mac.")
	return nil
}

func pubkeys(st licensetool.Store, _ []string) error {
	cfg, err := st.LoadConfig()
	if err != nil {
		return err
	}
	for _, k := range []struct{ service, kid, name string }{
		{licensetool.SigningService, cfg.SigningKID, "LicenseKeys"},
		{licensetool.GateService, cfg.GateKID, "GateKeys"},
	} {
		if k.kid == "" {
			continue
		}
		priv, err := st.PrivateKey(k.service, k.kid)
		if err != nil {
			return err
		}
		pub := priv.Public().(ed25519.PublicKey)
		fmt.Printf("%s: %q: mustKey(%q),\n", k.name, k.kid, base64.StdEncoding.EncodeToString(pub))
	}
	return nil
}

func parseDate(v, never string) (*time.Time, error) {
	if v == "" || v == never {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil, fmt.Errorf("dates are YYYY-MM-DD: %q", v)
	}
	// The whole day counts.
	t = t.Add(24*time.Hour - time.Second).UTC()
	return &t, nil
}

func issue(st licensetool.Store, args []string) error {
	fs := flag.NewFlagSet("issue", flag.ExitOnError)
	customer := fs.String("customer", "", "customer name")
	plan := fs.String("plan", "pro", "plan name")
	expires := fs.String("expires", "never", "YYYY-MM-DD or never")
	updates := fs.String("updates-until", "all", "YYYY-MM-DD or all")
	checkIn := fs.String("check-in", license.CheckInOnline, "online or offline")
	users := fs.Int("users", 0, "user limit (0: unlimited)")
	numbers := fs.Int("numbers", 0, "phone number limit (0: unlimited)")
	replaces := fs.String("replaces", "", "licence id this key replaces (the old one is superseded)")
	note := fs.String("note", "", "ledger note")
	_ = fs.Parse(args)
	if strings.TrimSpace(*customer) == "" {
		return errors.New("--customer is required")
	}
	if *checkIn != license.CheckInOnline && *checkIn != license.CheckInOffline {
		return errors.New("--check-in is online or offline")
	}
	exp, err := parseDate(*expires, "never")
	if err != nil {
		return err
	}
	upd, err := parseDate(*updates, "all")
	if err != nil {
		return err
	}
	cfg, err := st.LoadConfig()
	if err != nil {
		return err
	}
	priv, err := st.PrivateKey(licensetool.SigningService, cfg.SigningKID)
	if err != nil {
		return err
	}
	p := license.Payload{V: 1, ID: newID("lic_"), KeyID: cfg.SigningKID, Customer: strings.TrimSpace(*customer), Plan: *plan,
		IssuedAt: time.Now().UTC().Truncate(time.Second), ExpiresAt: exp, UpdatesUntil: upd, CheckIn: *checkIn,
		Limits: license.Limits{Users: *users, PhoneNumbers: *numbers}}
	key, err := license.IssueLicense(p, priv)
	if err != nil {
		return err
	}
	if err := st.Append(licensetool.Entry{At: p.IssuedAt, Action: "issued", LicenseID: p.ID, Customer: p.Customer, Plan: p.Plan, Key: key, Note: *note}); err != nil {
		return err
	}
	if *replaces != "" {
		if err := mark(st, cfg, *replaces, license.ReasonSuperseded, p.ID, "replaced by "+p.ID); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "Issued %s for %s.\n", p.ID, p.Customer)
	fmt.Println(key)
	return nil
}

func list(st licensetool.Store, _ []string) error {
	entries, err := st.Ledger()
	if err != nil {
		return err
	}
	state := map[string]string{}
	for _, e := range entries {
		if e.Action != "issued" {
			state[e.LicenseID] = e.Action
		}
	}
	for _, e := range entries {
		if e.Action != "issued" {
			continue
		}
		s := state[e.LicenseID]
		if s == "" {
			s = "valid"
		}
		fmt.Printf("%s  %-28s  %-10s  %-10s  %s\n", e.At.Format("2006-01-02"), e.LicenseID, s, e.Plan, e.Customer)
	}
	return nil
}

func show(st licensetool.Store, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: show LICENSE_ID")
	}
	entries, err := st.Ledger()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Action == "issued" && e.LicenseID == args[0] {
			fmt.Println(e.Key)
			return nil
		}
	}
	return fmt.Errorf("%s is not in the ledger", args[0])
}

func revoke(st licensetool.Store, args []string) error {
	if len(args) < 1 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: revoke LICENSE_ID [--note TEXT]")
	}
	id := args[0]
	fs := flag.NewFlagSet("revoke", flag.ExitOnError)
	note := fs.String("note", "", "ledger note")
	_ = fs.Parse(args[1:])
	cfg, err := st.LoadConfig()
	if err != nil {
		return err
	}
	if err := mark(st, cfg, id, license.ReasonRevoked, "", *note); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Revoked %s. Installations stop sending 7 days after their next check-in.\n", id)
	return nil
}

// mark records a revocation and publishes it to the gate's KV store.
func mark(st licensetool.Store, cfg licensetool.Config, id, action, replacedBy, note string) error {
	entries, err := st.Ledger()
	if err != nil {
		return err
	}
	found := false
	for _, e := range entries {
		found = found || (e.Action == "issued" && e.LicenseID == id)
	}
	if !found {
		return fmt.Errorf("%s is not in the ledger", id)
	}
	now := time.Now().UTC().Truncate(time.Second)
	if cfg.Cloudflare.KVNamespaceID != "" {
		val, _ := json.Marshal(map[string]any{"status": action, "at": now, "replaced_by": replacedBy})
		if err := kvPut(cfg, "revoked:"+id, string(val)); err != nil {
			return fmt.Errorf("could not publish to the gate (nothing recorded, try again): %w", err)
		}
	} else {
		fmt.Fprintln(os.Stderr, "warning: no Cloudflare KV configured; recorded locally only")
	}
	return st.Append(licensetool.Entry{At: now, Action: action, LicenseID: id, ReplacedBy: replacedBy, Note: note})
}

func kvPut(cfg licensetool.Config, key, value string) error {
	args := []string{"-y", wrangler, "kv", "key", "put", "--remote", "--namespace-id", cfg.Cloudflare.KVNamespaceID, key, value}
	cmd := exec.CommandContext(context.Background(), "npx", args...) //nolint:gosec // operator tool, no shell
	if cfg.Cloudflare.AccountID != "" {
		cmd.Env = append(os.Environ(), "CLOUDFLARE_ACCOUNT_ID="+cfg.Cloudflare.AccountID)
	}
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	return cmd.Run()
}

func cloudflare(st licensetool.Store, args []string) error {
	cfg, err := st.LoadConfig()
	if err != nil {
		return err
	}
	c := &cfg.Cloudflare
	fs := flag.NewFlagSet("cloudflare", flag.ExitOnError)
	fs.StringVar(&c.AccountID, "account", c.AccountID, "account id")
	fs.StringVar(&c.WorkerName, "worker", c.WorkerName, "worker name")
	fs.StringVar(&c.WorkerURL, "url", c.WorkerURL, "worker URL")
	fs.StringVar(&c.KVNamespaceID, "kv", c.KVNamespaceID, "KV namespace id")
	fs.StringVar(&c.R2Bucket, "bucket", c.R2Bucket, "R2 bucket")
	_ = fs.Parse(args)
	if err := st.SaveConfig(cfg); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	fmt.Println(string(b))
	return nil
}

func readPassphrase(prompt string, confirm bool) (string, error) {
	if v := os.Getenv("MESSORA_LICENSE_PASSPHRASE"); v != "" {
		return v, nil
	}
	read := func(p string) (string, error) {
		fmt.Fprint(os.Stderr, p)
		if term.IsTerminal(int(os.Stdin.Fd())) {
			b, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			return string(b), err
		}
		// No terminal (e.g. run from an editor): ask with a macOS dialog,
		// so the passphrase never lands in a transcript or shell history.
		if v, err := dialog(p); err == nil {
			return v, nil
		}
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		return strings.TrimRight(line, "\r\n"), err
	}
	a, err := read(prompt)
	if err != nil || !confirm {
		return a, err
	}
	b, err := read("Repeat: ")
	if err != nil {
		return "", err
	}
	if a != b {
		return "", errors.New("passphrases do not match")
	}
	return a, nil
}

func backup(st licensetool.Store, args []string) error {
	var path string
	switch dir := licensetool.ICloudDir(); {
	case len(args) > 0:
		path = args[0]
	case dir != "":
		path = filepath.Join(dir, "messora-license-backup.mslb")
	default:
		return errors.New("iCloud Drive is off: give a file path")
	}
	cfg, err := st.LoadConfig()
	if err != nil {
		return err
	}
	ledger, err := st.Ledger()
	if err != nil {
		return err
	}
	b := licensetool.Bundle{Version: 1, CreatedAt: time.Now().UTC(), Config: cfg, Ledger: ledger,
		SigningKeys: map[string]string{}, GateKeys: map[string]string{}}
	// Every key the ledger or config refers to.
	kids := map[string]bool{cfg.SigningKID: true}
	for _, e := range ledger {
		if e.Key != "" {
			if p, err := peekKID(e.Key); err == nil {
				kids[p] = true
			}
		}
	}
	for kid := range kids {
		if kid == "" {
			continue
		}
		priv, err := st.PrivateKey(licensetool.SigningService, kid)
		if err != nil {
			return err
		}
		b.SigningKeys[kid] = base64.StdEncoding.EncodeToString(priv.Seed())
	}
	if cfg.GateKID != "" {
		priv, err := st.PrivateKey(licensetool.GateService, cfg.GateKID)
		if err != nil {
			return err
		}
		b.GateKeys[cfg.GateKID] = base64.StdEncoding.EncodeToString(priv.Seed())
	}
	pass, err := readPassphrase("Backup passphrase (keep it in your password manager): ", true)
	if err != nil {
		return err
	}
	data, err := licensetool.Seal(b, pass)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { //nolint:gosec // the operator chooses the path
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil { //nolint:gosec // the operator chooses the path
		return err
	}
	fmt.Fprintf(os.Stderr, "Backup written to %s (%d keys, %d ledger lines).\n", path, len(b.SigningKeys)+len(b.GateKeys), len(ledger))
	return nil
}

// peekKID reads the key id of an issued key without verifying it.
func peekKID(key string) (string, error) {
	parts := strings.Split(key, ".")
	if len(parts) != 3 {
		return "", errors.New("malformed")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	var p struct {
		KID string `json:"kid"`
	}
	return p.KID, json.Unmarshal(raw, &p)
}

func restore(st licensetool.Store, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: restore FILE")
	}
	data, err := os.ReadFile(args[0]) //nolint:gosec // the operator chooses the path
	if err != nil {
		return err
	}
	pass, err := readPassphrase("Backup passphrase: ", false)
	if err != nil {
		return err
	}
	b, err := licensetool.Open(data, pass)
	if err != nil {
		return err
	}
	if old, _ := st.Ledger(); len(old) > len(b.Ledger) {
		return fmt.Errorf("this Mac's ledger has more entries (%d) than the backup (%d); not overwriting", len(old), len(b.Ledger))
	}
	put := func(service string, keys map[string]string) error {
		ids := make([]string, 0, len(keys))
		for k := range keys {
			ids = append(ids, k)
		}
		sort.Strings(ids)
		for _, kid := range ids {
			if err := st.Keychain.Set(service, kid, keys[kid]); err != nil {
				return err
			}
		}
		return nil
	}
	if err := put(licensetool.SigningService, b.SigningKeys); err != nil {
		return err
	}
	if err := put(licensetool.GateService, b.GateKeys); err != nil {
		return err
	}
	if err := st.SaveConfig(b.Config); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(st.Dir, "ledger.jsonl"))
	for _, e := range b.Ledger {
		if err := st.Append(e); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "Restored %d keys and %d ledger lines from %s.\n", len(b.SigningKeys)+len(b.GateKeys), len(b.Ledger), b.CreatedAt.Format(time.RFC3339))
	return nil
}

// dialog asks for a secret with a hidden-answer macOS dialog.
func dialog(prompt string) (string, error) {
	script := `on run argv
	set r to display dialog (item 1 of argv) default answer "" with hidden answer with title "messora-license" buttons {"Cancel", "OK"} default button "OK"
	return text returned of r
end run`
	out, err := exec.CommandContext(context.Background(), "osascript", "-e", script, strings.TrimSpace(prompt)).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}
