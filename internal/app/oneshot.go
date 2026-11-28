package app

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/identity"
	"github.com/furkanmeclis/messora-app-go/internal/platform/database"
	"github.com/furkanmeclis/messora-app-go/internal/platform/logger"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/provision"
)

// RunMigrate applies pending migrations and exits.
func RunMigrate(ctx context.Context) error {
	cfg, log, err := bootstrap(config.RoleMigrate)
	if err != nil {
		return reportFatal(log, err)
	}
	pool, err := openOwnerPool(ctx, cfg, config.RoleMigrate, log)
	if err != nil {
		return reportFatal(log, err)
	}
	defer pool.Close()
	v, err := database.Migrate(ctx, pool, log)
	if err != nil {
		return reportFatal(log, err)
	}
	log.Info("database schema up to date", "version", v)
	// The services connect as these roles; their passwords come from the
	// provisioned secrets.
	if err := database.SetRolePasswords(ctx, pool, map[string]string{
		database.AppRole: cfg.Database.App.Password.Reveal(), database.SystemRole: cfg.Database.System.Password.Reveal(),
	}); err != nil {
		return reportFatal(log, err)
	}
	if !cfg.Database.App.Password.IsSet() {
		log.Warn("POSTGRES_APP_PASSWORD is not set: the services cannot log in as messora_app (run messora init)")
	}
	added, err := tenant.BootstrapDomains(ctx, pool, cfg.PublicAppURL, cfg.TenantBaseDomain)
	if err != nil {
		return reportFatal(log, err)
	}
	if len(added) > 0 {
		log.Info("tenant #1 host names registered", "hosts", added)
	}
	// The AI price table is platform data, written by the owner role.
	if err := ai.EnsureSeed(ctx, pool); err != nil {
		// Not fatal: prices can be imported or entered in the panel.
		log.Error("could not store the seed AI prices", "err", err)
	}
	// Without a platform admin, a one-time link creates the first one.
	token, err := identity.IssueBootstrapToken(ctx, db.New(pool), time.Now())
	if err != nil {
		return reportFatal(log, err)
	}
	if token != "" {
		hosts := tenant.NewHosts(cfg.PublicAppURL, cfg.TenantBaseDomain)
		if admin := hosts.AdminURL(); admin != "" {
			log.Warn("No platform admin yet. Open this link to create one (valid 24 hours, single use), or run `messora admin create`",
				"bootstrap_url", admin+"/bootstrap?token="+token)
		} else {
			log.Warn("No platform admin yet and TENANT_BASE_DOMAIN is not set: run `messora admin create --email …`")
		}
	}
	return nil
}

// RunInit provisions secrets into MESSORA_SECRETS_DIR (default /secrets).
func RunInit(_ context.Context) error {
	log := logger.New(os.Stdout, logger.Options{Level: os.Getenv("LOG_LEVEL"), Format: os.Getenv("LOG_FORMAT"), Service: "messora", Role: "init"})
	dir := os.Getenv("MESSORA_SECRETS_DIR")
	if dir == "" {
		dir = "/secrets"
	}
	owners, err := provision.OwnersFromEnv(os.Getenv)
	if err != nil {
		return reportFatal(log, err)
	}
	res, err := provision.Run(provision.Options{
		Dir: dir, Getenv: os.Getenv, PublicAppURL: os.Getenv("PUBLIC_APP_URL"), BaseDomain: os.Getenv("TENANT_BASE_DOMAIN"), Owners: owners, Logger: log,
	})
	if err != nil {
		return reportFatal(log, err)
	}
	// Key names only — never values.
	log.Info("secrets provisioned", "dir", dir, "generated", res.Generated, "overridden_from_env", res.Overrides)
	return nil
}

// RunAdminCreate makes an account a platform admin:
//
//	messora admin create --email ops@example.com [--name "Ops"] [--password-stdin]
//
// A new account needs a password: from stdin with --password-stdin, else
// MESSORA_ADMIN_PASSWORD. An existing account is promoted (and gets the
// password when one is given). Two-factor authentication is set up at the
// first sign-in on the admin host.
func RunAdminCreate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("admin create", flag.ContinueOnError)
	email := fs.String("email", "", "e-mail address of the admin (required)")
	name := fs.String("name", "", "display name (new accounts)")
	fromStdin := fs.Bool("password-stdin", false, "read the password from standard input")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" {
		fs.Usage()
		return errors.New("--email is required")
	}
	password := os.Getenv("MESSORA_ADMIN_PASSWORD")
	if *fromStdin {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return fmt.Errorf("read password: %w", err)
		}
		password = strings.TrimRight(line, "\r\n")
	}
	cfg, log, err := bootstrap(config.RoleAPI)
	if err != nil {
		return reportFatal(log, err)
	}
	pool, err := openAppPool(ctx, cfg, config.RoleAPI, log)
	if err != nil {
		return reportFatal(log, err)
	}
	defer pool.Close()
	rdb, err := openRedis(ctx, cfg, log)
	if err != nil {
		return reportFatal(log, err)
	}
	defer rdb.Close()
	svc, err := newServices(cfg, pool, rdb, log)
	if err != nil {
		return reportFatal(log, err)
	}
	_, created, err := svc.identity.CreatePlatformAdmin(ctx, *email, *name, password)
	if err != nil {
		fmt.Fprintln(os.Stderr, "messora admin create:", err)
		return err
	}
	what := "promoted to platform admin"
	if created {
		what = "created as platform admin"
	}
	fmt.Printf("%s %s. Sign in at %s (two-factor authentication is set up at the first sign-in).\n", *email, what, svc.hosts.AdminURL())
	return nil
}

// RunHealthcheck performs an HTTP GET and exits non-zero unless it returns
// 200. Distroless images have no curl, so Docker healthchecks call this.
func RunHealthcheck(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck %s: status %d", url, resp.StatusCode)
	}
	return nil
}

// reportFatal logs err (when a logger exists) and returns it for the exit code.
func reportFatal(log *slog.Logger, err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return nil
	}
	if log != nil {
		log.Error("fatal", "err", err)
	}
	return err
}
