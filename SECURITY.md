# Security Policy

Messora sends messages to real customers and stores their personal data, so we
take security reports seriously.

## Reporting a vulnerability

**Please do not open a public issue for security problems.**

Report privately through GitHub's
[private vulnerability reporting](https://github.com/furkanmeclis/messora-app-go/security/advisories/new)
for this repository. Include:

- the affected version or commit,
- steps to reproduce or a proof of concept,
- the impact you observed or expect.

You will get an acknowledgement within 5 working days. We aim to ship a fix
for confirmed high-severity issues within 30 days and will credit you in the
advisory unless you prefer otherwise.

## Supported versions

Messora is pre-1.0. Only the latest commit on `main` receives security fixes.

## Scope

In scope: this repository's Go backend, Next.js panel, Docker Compose
deployment and documentation. Out of scope: vulnerabilities in Meta's
WhatsApp Cloud API, PostgreSQL, Redis, Centrifugo or Caddy themselves (report
those upstream), and issues requiring an already compromised host.

## Hardening guidance

See [`docs/SECURITY.md`](docs/SECURITY.md) for the threat model, the controls
Messora implements, and operator recommendations (HTTPS, backups of the
`messora-secrets` volume, key rotation).
