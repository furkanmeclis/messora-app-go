# Contributing to Messora

Thank you for helping! Messora is a self-hosted WhatsApp Business Cloud API
platform built as a Go modular monolith with a Next.js panel.

## Ground rules

- **Reliability first.** Message loss and duplicate sends are the worst bugs
  this project can have. Read [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
  §6 before touching anything that sends messages.
- **Official APIs only.** No WhatsApp Web, QR-code sessions or unofficial
  libraries.
- **No placeholders.** Do not merge stub endpoints, fake success responses or
  TODOs that stand in for required behaviour.
- **Tests are part of the change.** A feature without tests is not done.
- **Never commit secrets.** `make secrets-scan` must pass.

## Development setup

Requirements: Docker, Go (any version — `GOTOOLCHAIN=auto` fetches the one in
`go.mod`), Node.js 26.

```bash
cp .env.example .env
make up                 # full stack at http://localhost (see HTTP_PORT)
make test               # unit tests
make test-integration   # + real PostgreSQL/Redis via testcontainers
make lint web-lint      # Go and frontend linters
make help               # everything else
```

After changing SQL in `queries/` or `migrations/`, run `make sqlc` and commit
the generated code.

### Database migrations

- Add a new file `migrations/NNNNN_description.sql`; never edit a migration
  that has been released.
- Provide a working `-- +goose Down` section; the integration tests run
  up → down → up.
- Large tables: create indexes `CONCURRENTLY` in a migration marked
  `-- +goose NO TRANSACTION`.

## Commits and pull requests

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/):

```text
feat(campaigns): add campaign scheduling
fix(worker): prevent duplicate message dispatch
docs(deployment): document docker compose setup
```

Before opening a PR run `make check`. Describe what changed, why, and how you
tested it. Keep PRs focused; open an issue first for large changes.

## Code derived from ZeroMsg

Parts of the UI are derived from [ZeroMsg](https://github.com/megoxv/zeromsg)
(MIT). When you copy more code from it, keep the `Derived from ZeroMsg (MIT)`
header and list the path in [`NOTICE`](NOTICE).

## Code of conduct

Participation is governed by our [Code of Conduct](CODE_OF_CONDUCT.md).
