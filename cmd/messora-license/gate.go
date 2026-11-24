package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/licensetool"
	"github.com/furkanmeclis/messora-app-go/internal/platform/license"
)

// Image repositories a release consists of.
var releaseRepos = []string{"messora", "messora-web"}

const (
	mediaIndex    = "application/vnd.oci.image.index.v1+json"
	mediaManifest = "application/vnd.oci.image.manifest.v1+json"
	mediaDockerML = "application/vnd.docker.distribution.manifest.list.v2+json"
	mediaDockerM  = "application/vnd.docker.distribution.manifest.v2+json"
)

func wranglerCmd(cfg licensetool.Config, dir string, stdin io.Reader, args ...string) error {
	cmd := exec.CommandContext(context.Background(), "npx", append([]string{"-y", wrangler}, args...)...) //nolint:gosec // operator tool, no shell
	cmd.Dir = dir
	cmd.Stdin = stdin
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if cfg.Cloudflare.AccountID != "" {
		cmd.Env = append(os.Environ(), "CLOUDFLARE_ACCOUNT_ID="+cfg.Cloudflare.AccountID)
	}
	return cmd.Run()
}

// gateSecret uploads the gate's private key as the Worker secret GATE_KEY.
func gateSecret(st licensetool.Store, _ []string) error {
	cfg, err := st.LoadConfig()
	if err != nil {
		return err
	}
	if cfg.Cloudflare.WorkerName == "" {
		return errors.New("set the worker first: messora-license cloudflare --worker NAME")
	}
	priv, err := st.PrivateKey(licensetool.GateService, cfg.GateKID)
	if err != nil {
		return err
	}
	val, _ := json.Marshal(map[string]string{"kid": cfg.GateKID, "seed": base64.StdEncoding.EncodeToString(priv.Seed())})
	return wranglerCmd(cfg, "", strings.NewReader(string(val)), "secret", "put", "GATE_KEY", "--name", cfg.Cloudflare.WorkerName)
}

// gateDeploy deploys deployments/license-gate with the current public keys.
func gateDeploy(st licensetool.Store, args []string) error {
	fs := flag.NewFlagSet("gate-deploy", flag.ExitOnError)
	dir := fs.String("dir", "deployments/license-gate", "worker directory")
	_ = fs.Parse(args)
	cfg, err := st.LoadConfig()
	if err != nil {
		return err
	}
	keys := map[string]string{}
	entries, err := st.Ledger()
	if err != nil {
		return err
	}
	kids := map[string]bool{cfg.SigningKID: true}
	for _, e := range entries {
		if k, err := peekKID(e.Key); err == nil && k != "" {
			kids[k] = true
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
		keys[kid] = base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey))
	}
	b, _ := json.Marshal(keys)
	return wranglerCmd(cfg, *dir, nil, "deploy", "--var", "LICENSE_KEYS:"+string(b))
}

// registryClient reads images from a source registry (GHCR).
type registryClient struct {
	host, owner, token string
	http               *http.Client
}

func (c *registryClient) auth(repo string) error {
	u := fmt.Sprintf("https://%s/token?scope=repository:%s/%s:pull", c.host, c.owner, repo)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	if t := os.Getenv("GHCR_TOKEN"); t != "" {
		req.SetBasicAuth("token", t)
	}
	res, err := c.http.Do(req) //nolint:gosec // the operator chooses the source registry
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var v struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil || v.Token == "" {
		return fmt.Errorf("registry token for %s: %s", repo, res.Status)
	}
	c.token = v.Token
	return nil
}

func (c *registryClient) get(repo, kind, ref string, accept ...string) ([]byte, string, error) {
	u := fmt.Sprintf("https://%s/v2/%s/%s/%s/%s", c.host, c.owner, repo, kind, ref)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil) //nolint:gosec // the operator chooses the source registry
	req.Header.Set("Authorization", "Bearer "+c.token)
	if len(accept) > 0 {
		req.Header.Set("Accept", strings.Join(accept, ", "))
	}
	res, err := c.http.Do(req) //nolint:gosec // the operator chooses the source registry
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("%s %s/%s: %s", kind, repo, ref, res.Status)
	}
	b, err := io.ReadAll(res.Body)
	return b, res.Header.Get("Content-Type"), err
}

func digestOf(b []byte) string {
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}

type ociManifest struct {
	MediaType string `json:"mediaType"`
	Manifests []struct {
		Digest    string `json:"digest"`
		MediaType string `json:"mediaType"`
	} `json:"manifests"`
	Config struct {
		Digest string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		Digest string `json:"digest"`
	} `json:"layers"`
}

// publish copies a release's images from GHCR into the R2 registry and
// announces it on the update channel.
func publish(st licensetool.Store, args []string) error {
	if len(args) < 1 || strings.HasPrefix(args[0], "-") {
		return errors.New("usage: publish VERSION [--channel stable] [--notes TEXT] [--released YYYY-MM-DD] [--source ghcr.io/OWNER]")
	}
	version := strings.TrimPrefix(args[0], "v")
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	channel := fs.String("channel", "stable", "update channel")
	notes := fs.String("notes", "", "release notes (one line)")
	released := fs.String("released", "", "release date (default: today)")
	source := fs.String("source", "ghcr.io/furkanmeclis", "source registry/owner")
	_ = fs.Parse(args[1:])
	cfg, err := st.LoadConfig()
	if err != nil {
		return err
	}
	if cfg.Cloudflare.R2Bucket == "" || cfg.Cloudflare.KVNamespaceID == "" {
		return errors.New("set the bucket and KV first: messora-license cloudflare --bucket NAME --kv ID")
	}
	at := time.Now().UTC().Truncate(time.Second)
	if *released != "" {
		t, err := time.Parse("2006-01-02", *released)
		if err != nil {
			return err
		}
		at = t.UTC()
	}
	host, owner, ok := strings.Cut(*source, "/")
	if !ok {
		return errors.New("--source is HOST/OWNER")
	}
	src := &registryClient{host: host, owner: owner, http: &http.Client{Timeout: 10 * time.Minute}}
	tmp, err := os.MkdirTemp("", "messora-publish-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	// Blobs already uploaded (by digest), so re-publishing is cheap.
	donePath := filepath.Join(st.Dir, "r2-objects.txt")
	done := map[string]bool{}
	if b, err := os.ReadFile(donePath); err == nil { //nolint:gosec // fixed path under the state dir
		for _, l := range strings.Split(string(b), "\n") {
			done[l] = true
		}
	}
	put := func(key string, body []byte, contentType string) error {
		if done[key] {
			return nil
		}
		f := filepath.Join(tmp, "obj")
		if err := os.WriteFile(f, body, 0o600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "upload %s (%d KiB)\n", key, len(body)/1024)
		if err := wranglerCmd(cfg, "", nil, "r2", "object", "put", cfg.Cloudflare.R2Bucket+"/"+key, "--file", f, "--content-type", contentType, "--remote"); err != nil {
			return err
		}
		done[key] = true
		af, err := os.OpenFile(donePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // fixed path under the state dir
		if err != nil {
			return err
		}
		defer af.Close()
		_, err = af.WriteString(key + "\n")
		return err
	}
	accept := []string{mediaIndex, mediaManifest, mediaDockerML, mediaDockerM}
	for _, repo := range releaseRepos {
		if err := src.auth(repo); err != nil {
			return err
		}
		top, ct, err := src.get(repo, "manifests", version, accept...)
		if err != nil {
			return err
		}
		queue := []struct {
			body []byte
			ct   string
		}{{top, ct}}
		for len(queue) > 0 {
			m := queue[0]
			queue = queue[1:]
			var om ociManifest
			if err := json.Unmarshal(m.body, &om); err != nil {
				return err
			}
			for _, child := range om.Manifests {
				body, cct, err := src.get(repo, "manifests", child.Digest, accept...)
				if err != nil {
					return err
				}
				if digestOf(body) != child.Digest {
					return fmt.Errorf("digest mismatch for %s", child.Digest)
				}
				queue = append(queue, struct {
					body []byte
					ct   string
				}{body, cct})
			}
			blobs := []string{}
			if om.Config.Digest != "" {
				blobs = append(blobs, om.Config.Digest)
			}
			for _, l := range om.Layers {
				blobs = append(blobs, l.Digest)
			}
			for _, d := range blobs {
				key := fmt.Sprintf("v2/%s/blobs/%s", repo, d)
				if done[key] {
					continue
				}
				body, _, err := src.get(repo, "blobs", d)
				if err != nil {
					return err
				}
				if digestOf(body) != d {
					return fmt.Errorf("digest mismatch for blob %s", d)
				}
				if err := put(key, body, "application/octet-stream"); err != nil {
					return err
				}
			}
			if err := put(fmt.Sprintf("v2/%s/manifests/%s", repo, digestOf(m.body)), m.body, m.ct); err != nil {
				return err
			}
		}
		// The tag last, so it never points at missing objects. Tags move,
		// so always re-upload.
		delete(done, fmt.Sprintf("v2/%s/manifests/%s", repo, version))
		if err := put(fmt.Sprintf("v2/%s/manifests/%s", repo, version), top, ct); err != nil {
			return err
		}
	}
	// The install file customers download (served by the gate).
	gateURL := cfg.Cloudflare.WorkerURL
	if gateURL == "" {
		gateURL = license.DefaultGateURL
	}
	install, err := buildInstallCompose(".", gateURL, version)
	if err != nil {
		return fmt.Errorf("install file (run from the repository root): %w", err)
	}
	for _, key := range []string{"install/" + version + "/compose.yml", "install/" + *channel + "/compose.yml"} {
		delete(done, key)
		if err := put(key, install, "application/yaml"); err != nil {
			return err
		}
	}
	rel, _ := json.Marshal(license.Release{Version: version, ReleasedAt: at, Channel: *channel, Notes: *notes})
	if err := kvPut(cfg, "release:"+version, string(rel)); err != nil {
		return err
	}
	if err := kvPut(cfg, "release:latest:"+*channel, string(rel)); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Published %s on %s.\n", version, *channel)
	return nil
}

// installCompose prints the customer install file for a version.
func installCompose(_ licensetool.Store, args []string) error {
	fs := flag.NewFlagSet("install-compose", flag.ExitOnError)
	registry := fs.String("registry", license.DefaultGateURL, "image registry (host or URL)")
	version := fs.String("version", "", "release version, e.g. 0.10.0")
	dir := fs.String("dir", ".", "repository root")
	_ = fs.Parse(args)
	out, err := buildInstallCompose(*dir, *registry, *version)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(out)
	return err
}

func buildInstallCompose(dir, registry, version string) ([]byte, error) {
	compose, err := os.ReadFile(filepath.Join(dir, "docker-compose.yml")) //nolint:gosec // repository file
	if err != nil {
		return nil, err
	}
	caddy, err := os.ReadFile(filepath.Join(dir, "deployments", "caddy", "Caddyfile")) //nolint:gosec // repository file
	if err != nil {
		return nil, err
	}
	registry = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(registry, "https://"), "http://"), "/")
	return licensetool.InstallCompose(compose, caddy, registry, strings.TrimPrefix(version, "v"))
}
