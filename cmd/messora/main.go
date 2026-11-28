// Command messora is the single Messora binary. Each sub-command runs one
// process role so the same image serves every container.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/furkanmeclis/messora-app-go/internal/app"
)

const usage = `Messora — self-hosted WhatsApp Business Cloud API platform

Usage:
  messora <command>

Commands:
  api                 Serve the REST API and WhatsApp webhooks
  worker              Process background tasks and relay the outbox
  scheduler           Enqueue periodic tasks (leader-elected)
  migrate             Apply database migrations and exit
  init                Generate missing secrets into MESSORA_SECRETS_DIR and exit
  admin create        Make an account a platform admin (--email, --name, --password-stdin)
  tech-provider       Show, change or test Messora's Meta Tech Provider set-up
  fake-meta           Serve a fake Meta Graph API (E2E stack only; not in production)
  healthcheck <url>   Exit 0 if GET <url> returns 200 (for container healthchecks)
  version             Print the version
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "api":
		err = app.RunAPI(ctx)
	case "worker":
		err = app.RunWorker(ctx)
	case "scheduler":
		err = app.RunScheduler(ctx)
	case "migrate":
		err = app.RunMigrate(ctx)
	case "init":
		err = app.RunInit(ctx)
	case "admin":
		if len(os.Args) < 3 || os.Args[2] != "create" {
			fmt.Fprintln(os.Stderr, "usage: messora admin create --email <address> [--name <name>] [--password-stdin]")
			os.Exit(2)
		}
		err = app.RunAdminCreate(ctx, os.Args[3:])
	case "tech-provider":
		err = app.RunTechProvider(ctx, os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	case "fake-meta":
		err = app.RunFakeMeta(ctx)
	case "healthcheck":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: messora healthcheck <url>")
			os.Exit(2)
		}
		err = app.RunHealthcheck(ctx, os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	case "version", "--version", "-v":
		fmt.Println(app.Version)
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		os.Exit(1)
	}
}
