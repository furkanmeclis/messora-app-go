// Command apidocs derives the public API documentation from api/openapi.yaml:
// the public OpenAPI document (openapi.public.json) and the endpoint
// inventory (docs/API.md). With -check it fails when the inventory is stale.
//
//	go run ./cmd/apidocs -md docs/API.md -json bin/openapi.public.json -version v0.12.0
//	go run ./cmd/apidocs -md docs/API.md -check
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/furkanmeclis/messora-app-go/internal/platform/apidocs"
	"github.com/furkanmeclis/messora-app-go/internal/platform/apiguard"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "apidocs:", err)
		os.Exit(1)
	}
}

func run() error {
	spec := flag.String("spec", "api/openapi.yaml", "full OpenAPI spec")
	md := flag.String("md", "", "write (or with -check, compare) the inventory markdown here")
	out := flag.String("json", "", "write the public OpenAPI document here")
	version := flag.String("version", "dev", "application version for info.version")
	check := flag.Bool("check", false, "fail if -md is out of date instead of writing it")
	flag.Parse()

	raw, err := os.ReadFile(*spec)
	if err != nil {
		return err
	}
	full, err := apidocs.Load(raw)
	if err != nil {
		return err
	}
	if problems := apidocs.Lint(full); len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, p)
		}
		return fmt.Errorf("%d public operations are not fully documented", len(problems))
	}
	opts := apidocs.Options{Version: *version, APIKeyPerMinute: apiguard.APIKeyPerMinute}
	if *out != "" {
		b, err := apidocs.PublicJSON(raw, opts)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(*out), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(*out, b, 0o644); err != nil { //nolint:gosec // a public document
			return err
		}
	}
	if *md != "" {
		pub, err := apidocs.Public(full, opts)
		if err != nil {
			return err
		}
		inv := apidocs.Inventory(pub)
		if *check {
			cur, _ := os.ReadFile(*md)
			if !bytes.Equal(cur, inv) {
				return fmt.Errorf("%s is stale: run make api-docs", *md)
			}
			return nil
		}
		if err := os.WriteFile(*md, inv, 0o644); err != nil { //nolint:gosec // documentation
			return err
		}
	}
	return nil
}
