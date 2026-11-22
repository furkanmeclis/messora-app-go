package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/platform/database"
	"github.com/furkanmeclis/messora-app-go/internal/platform/logger"
	"github.com/furkanmeclis/messora-app-go/internal/provision"
)

// RunMigrate applies pending migrations and exits.
func RunMigrate(ctx context.Context) error {
	cfg, log, err := bootstrap(config.RoleMigrate)
	if err != nil {
		return reportFatal(log, err)
	}
	pool, err := openPostgres(ctx, cfg, config.RoleMigrate, log)
	if err != nil {
		return reportFatal(log, err)
	}
	defer pool.Close()
	v, err := database.Migrate(ctx, pool, log)
	if err != nil {
		return reportFatal(log, err)
	}
	log.Info("database schema up to date", "version", v)
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
		Dir: dir, Getenv: os.Getenv, PublicAppURL: os.Getenv("PUBLIC_APP_URL"), Owners: owners, Logger: log,
	})
	if err != nil {
		return reportFatal(log, err)
	}
	// Key names only — never values.
	log.Info("secrets provisioned", "dir", dir, "generated", res.Generated, "overridden_from_env", res.Overrides)
	return nil
}

// RunSetupToken prints a fresh one-time setup link (replacing any previous
// one) while no user exists: `docker compose exec messora-api messora setup token`.
func RunSetupToken(ctx context.Context) error {
	cfg, log, err := bootstrap(config.RoleAPI)
	if err != nil {
		return reportFatal(log, err)
	}
	pool, err := openPostgres(ctx, cfg, config.RoleAPI, log)
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
	token, err := svc.identity.EnsureSetupToken(ctx, true)
	if err != nil {
		return reportFatal(log, err)
	}
	if token == "" {
		fmt.Println("Setup is already complete; sign in or ask an owner for an invitation.")
		return nil
	}
	fmt.Println("Open this link to create the owner account (valid 24 hours, single use):")
	fmt.Println(svc.identity.SetupURL(token))
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
