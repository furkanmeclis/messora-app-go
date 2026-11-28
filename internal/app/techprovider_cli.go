package app

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/modules/channels"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/logger"
)

// TechProviderUsage documents `messora tech-provider`.
const TechProviderUsage = `usage: messora tech-provider <show|set|test> [flags]

Messora's Tech Provider set-up (Meta Embedded Signup), written as the
database owner like "messora migrate" (docs/TECH_PROVIDER.md). Secrets are
read from the environment, never from flags:
  MESSORA_TP_APP_SECRET          the App secret of Messora's Meta app
  MESSORA_TP_SYSTEM_USER_TOKEN   a system user token of Messora's business

set flags:
  -enable / -disable             turn the method on or off for every company
  -app-id ID                     Messora's Meta app
  -configuration-id ID           the Embedded Signup configuration
  -graph-api-version vNN.N       empty: META_GRAPH_API_VERSION
  -rotate-verify-token           a new webhook verify token
  -app-secret-stdin              read the App secret from standard input
`

// RunTechProvider runs `messora tech-provider`.
func RunTechProvider(ctx context.Context, args []string) error {
	if len(args) == 0 {
		_, _ = fmt.Fprint(os.Stderr, TechProviderUsage)
		return errors.New("missing sub-command")
	}
	cfg, log, err := bootstrap(config.RoleMigrate)
	if err != nil {
		return reportFatal(log, err)
	}
	if len(cfg.EncryptionKey) != 32 {
		return reportFatal(log, errors.New("APP_ENCRYPTION_KEY is required to seal the Tech Provider secrets"))
	}
	pool, err := openOwnerPool(ctx, cfg, config.RoleMigrate, log)
	if err != nil {
		return reportFatal(log, err)
	}
	defer pool.Close()
	sealer, err := crypto.NewSealer(cfg.EncryptionKey, nil)
	if err != nil {
		return reportFatal(log, err)
	}
	wa, err := whatsapp.New(whatsapp.Options{BaseURL: cfg.Meta.GraphBaseURL, Version: cfg.Meta.GraphVersion, DryRun: cfg.Meta.DryRun})
	if err != nil {
		return reportFatal(log, err)
	}
	svc := channels.New(pool, sealer, wa, log)
	callback := cfg.Meta.WebhookBaseURL + channels.TechProviderWebhookPath
	out := os.Stdout

	switch args[0] {
	case "show":
		c, err := svc.TechProvider(ctx)
		if err != nil {
			return reportFatal(log, err)
		}
		printTechProvider(out, c, callback, wa.Version())
	case "set":
		fs := flag.NewFlagSet("tech-provider set", flag.ContinueOnError)
		enable := fs.Bool("enable", false, "turn the method on")
		disable := fs.Bool("disable", false, "turn the method off")
		appID := fs.String("app-id", "", "Messora's Meta app ID")
		confID := fs.String("configuration-id", "", "Embedded Signup configuration ID")
		version := fs.String("graph-api-version", "", "Graph API version")
		rotate := fs.Bool("rotate-verify-token", false, "new webhook verify token")
		secretStdin := fs.Bool("app-secret-stdin", false, "read the App secret from standard input")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *enable && *disable {
			return errors.New("-enable and -disable exclude each other")
		}
		p := channels.TechProviderPatch{RotateVerifyToken: *rotate}
		fs.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "app-id":
				p.AppID = appID
			case "configuration-id":
				p.ConfigurationID = confID
			case "graph-api-version":
				p.GraphAPIVersion = version
			}
		})
		if *enable || *disable {
			on := *enable
			p.Enabled = &on
		}
		if v, ok := os.LookupEnv("MESSORA_TP_APP_SECRET"); ok {
			p.AppSecret = &v
		}
		if *secretStdin {
			line, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			v := strings.TrimSpace(line)
			p.AppSecret = &v
		}
		if v, ok := os.LookupEnv("MESSORA_TP_SYSTEM_USER_TOKEN"); ok {
			p.SystemUserToken = &v
		}
		c, changed, err := svc.UpdateTechProvider(ctx, pool, p)
		if err != nil {
			return reportFatal(log, err)
		}
		_, _ = fmt.Fprintf(out, "Changed: %s\n", strings.Join(changed, ", "))
		printTechProvider(out, c, callback, wa.Version())
	case "test":
		checks, err := svc.TestTechProvider(ctx, callback)
		if err != nil {
			return reportFatal(log, err)
		}
		failed := false
		for _, c := range checks {
			_, _ = fmt.Fprintf(out, "%-8s %-18s %s\n", strings.ToUpper(c.Status), c.Key, c.Message)
			failed = failed || c.Status == channels.CheckError
		}
		if failed {
			return errors.New("the Tech Provider set-up has errors")
		}
	default:
		_, _ = fmt.Fprint(os.Stderr, TechProviderUsage)
		return fmt.Errorf("unknown sub-command %q", args[0])
	}
	return nil
}

func printTechProvider(w io.Writer, c channels.TechProviderConfig, callback, defaultVersion string) {
	set := func(v string) string {
		if v == "" {
			return "not set"
		}
		return "set"
	}
	or := func(v, d string) string {
		if v == "" {
			return d
		}
		return v
	}
	_, _ = fmt.Fprintf(w, "Enabled:            %v (ready: %v)\n", c.Enabled, c.Ready())
	_, _ = fmt.Fprintf(w, "App ID:             %s\n", or(c.AppID, "not set"))
	_, _ = fmt.Fprintf(w, "Configuration ID:   %s\n", or(c.ConfigurationID, "not set"))
	_, _ = fmt.Fprintf(w, "Graph API version:  %s\n", or(c.GraphAPIVersion, defaultVersion+" (platform default)"))
	_, _ = fmt.Fprintf(w, "App secret:         %s\n", set(c.AppSecret))
	_, _ = fmt.Fprintf(w, "System user token:  %s\n", set(c.SystemUserToken))
	_, _ = fmt.Fprintf(w, "Callback URL:       %s\n", callback)
	_, _ = fmt.Fprintf(w, "Verify token:       %s\n", or(c.VerifyToken, "not set"))
}

// RunFakeMeta serves the scriptable Graph API stand-in for the E2E stack
// (`messora fake-meta`): every Embedded Signup code of the E2E fake SDK
// provisions its WABA and number. Refused in production.
func RunFakeMeta(ctx context.Context) error {
	log := logger.New(os.Stdout, logger.Options{Level: os.Getenv("LOG_LEVEL"), Format: os.Getenv("LOG_FORMAT"), Service: "messora", Role: "fake-meta"})
	if env := os.Getenv("APP_ENV"); env == "" || env == config.EnvProduction {
		return reportFatal(log, errors.New("messora fake-meta runs only with APP_ENV=development or test"))
	}
	addr := os.Getenv("FAKE_META_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	appID := os.Getenv("FAKE_META_APP_ID")
	if appID == "" {
		appID = whatsapptest.DefaultAppID
	}
	_, h := whatsapptest.NewAutoSignupHandler(appID, os.Getenv("FAKE_META_CALLBACK_URL"))
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.Handle("/", h)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx }}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	log.Info("fake Meta Graph API listening", "addr", addr, "app_id", appID)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return reportFatal(log, err)
	}
	return nil
}
