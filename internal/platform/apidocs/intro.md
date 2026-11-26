The Messora API lets your systems send WhatsApp messages, manage contacts and
audiences, run campaigns, work the shared inbox and receive events as
webhooks. Every operation listed here can be called with an API key; the
panel itself uses the same API.

## Authentication

Create a key under **API keys** in the panel and send it as a bearer token:

```http
GET /api/v1/api-keys/whoami
Authorization: Bearer msk_live_…
```

The secret is shown once, when the key is created. Keys can expire and can be
revoked at any time; a revoked or expired key answers `401 UNAUTHORIZED`.
Call `GET /api-keys/whoami` to check a key and see its scopes.

## Scopes

A key is created with a list of **scopes**, the same permission keys the
panel's roles use (`messages.send`, `contacts.read`, `campaigns.start`, …).
Each operation states the scope it needs (also in `x-required-scopes`); when
several are listed, any one of them is enough. A key without the scope gets
`403 FORBIDDEN` with the missing permission in the message.

## Rate limits

Each API key may make **{{.APIKeyPerMinute}} requests per minute**, with bursts of up to
the same number (generic cell rate algorithm, shared by all API replicas).
Every response reports the budget in the headers of the IETF
[RateLimit header fields](https://datatracker.ietf.org/doc/draft-ietf-httpapi-ratelimit-headers/) draft:

| Header | Meaning |
|---|---|
| `RateLimit-Limit` | Requests allowed in a burst (the per-minute budget). |
| `RateLimit-Remaining` | Requests you can still make right now. |
| `RateLimit-Reset` | Seconds until the full budget is available again. |
| `RateLimit-Policy` | The policy, e.g. `{{.APIKeyPerMinute}};w=60` ({{.APIKeyPerMinute}} per 60 seconds). |

Over the limit the API answers `429 RATE_LIMITED` with `Retry-After`
(seconds until the next request is allowed). Wait at least that long before
retrying; spread bulk work out instead of retrying in a tight loop.

## Idempotency

`POST` operations accept an `Idempotency-Key` header (8–200 characters, e.g. a
UUID). A retry with the same key and the same body returns the stored
response of the first successful attempt for 24 hours, marked with
`Idempotent-Replayed: true`, instead of doing the work twice. The same key
with a different body is `409 IDEMPOTENCY_KEY_REUSED`; a retry while the first
attempt is still running is `409 IDEMPOTENCY_IN_PROGRESS`. Failed requests
(4xx/5xx) are not remembered, so a corrected request may reuse its key. Use a
new key for every new action, and always send one when sending messages or
starting campaigns from a job that may retry.

## Responses and errors

Successful responses wrap the result in an envelope:

```json
{ "data": { … }, "meta": { "request_id": "01a2…" } }
```

Errors use one format for every status:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request",
    "details": [{ "field": "to", "code": "invalid", "message": "must be in E.164 format" }]
  },
  "meta": { "request_id": "01a2…" }
}
```

Branch on `error.code` (stable, machine-readable), not on `message` (human
readable, may change). Common codes: `UNAUTHORIZED` (401), `FORBIDDEN` (403),
`NOT_FOUND` (404), `CONFLICT` and other conflict codes (409),
`VALIDATION_ERROR` (422, with field `details`), `RATE_LIMITED` (429),
`INTERNAL_ERROR` (500). Quote `meta.request_id` (also the `X-Request-Id`
header) when you contact support.

## Pagination

List operations return at most `limit` items (default 50, maximum 200). When
more exist, `meta.next_cursor` is set: pass it as `cursor` to get the next
page, until it is absent. Cursors are opaque; do not build them yourself.
Some lists (contacts) also accept `page` for numbered pages, and then report
`meta.total`.

## Webhooks

Instead of polling, register an HTTPS endpoint (`POST /webhook-endpoints`) and
Messora pushes events such as `message.received` or `campaign.completed`.
Each delivery is a signed `POST`: verify `Messora-Signature`
(`t=<unix seconds>,v1=<hex>`, HMAC-SHA256 with the endpoint secret over
`"<t>.<raw body>"`) against the raw body, in constant time, and reject old
timestamps. Delivery is at least once and not ordered: drop duplicates by
the event `id`. Failed deliveries are retried for about 24 hours and can be
redelivered from the delivery log.
