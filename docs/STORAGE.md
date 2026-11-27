# File storage

Messora keeps every file it stores — inbound WhatsApp media, contact import
files and their error reports, campaign export files and the organisation
logo — in an object store (`internal/platform/storage`). Two drivers exist:

| Driver | Use | Where files live |
|---|---|---|
| `local` (default) | self-hosted single server | the `messora-media` volume (`STORAGE_LOCAL_DIR`) |
| `s3` | SaaS, several servers, staging | two S3 buckets — AWS S3, Cloudflare R2 or MinIO |

Code never touches the file system for stored files; it calls
`storage.Store` (`Put`, `Get`, `Stat`, `Delete`, `PresignGet`, `PublicURL`,
`Ping`).

## Two classes of objects

| Class | Contents | How it is read |
|---|---|---|
| **private** | inbound media, imports, import error reports, exports | through an authenticated API route (visibility checked first), or a short-lived presigned URL |
| **public** | logos, template and campaign header images | `PUBLIC_ASSET_BASE_URL` (a CDN) or, without one, the app's `/media/p/<key>` route |

Private objects are never reachable without a check or a signature. Public
objects are tenant-scoped (`t/<tenant>/…`); nothing else is served from the
public route.

## Keys

New keys are tenant-scoped:

```
t/<tenant_id>/<kind>/<yyyy>/<mm>/<uuid>[.<ext>]
```

| kind | Written by |
|---|---|
| `media` | inbound WhatsApp media (worker, `messaging.DownloadMedia`) |
| `imports` | uploaded contact files, stored as CSV (workbooks are converted first) |
| `import-errors` | the error report of an import |
| `exports` | background campaign exports (CSV, XLSX, PDF) |
| `branding/logo-<version>` | the logo (public; a new key per upload so a CDN never serves a stale one) |

`storage.Key(tenantID, kind, ext)` builds keys. `storage.ValidateKey` refuses
`..`, `.` and empty segments, absolute paths, backslashes, control
characters, and segments starting with `_` or `.` or ending in `._meta`
(reserved by the local driver). `storage.CheckTenantKey(tenant, key)` refuses
keys outside `t/<tenant>/`; every read of a key taken from the database goes
through it.

**Tenant id.** `storage.TenantKey(ctx)` returns the tenant the context
carries (`storage.WithTenant`) or `default` when it carries none. Until the
tenancy work package (WP1) puts the tenant into request and job contexts,
everything is stored under `t/default/`. A malformed tenant id fails closed
(no key is built, no key is accepted) instead of falling back to `default`.

**Older files.** Keys written before the object store (`aa/bb/<uuid>-<hex>`
for media, `imports/uploads/<hex>.csv`, `exports/<id>.<format>`,
`branding/logo`) map one-to-one onto the same paths under
`STORAGE_LOCAL_DIR`, so an existing `MEDIA_DIR` keeps working with the local
driver without a migration. They belong to the `default` tenant only.

## Drivers

### local

- Objects are files under `STORAGE_LOCAL_DIR` (key `a/b/c` is
  `<dir>/a/b/c`); public objects under `<dir>/_public/`. A `<file>._meta`
  sidecar keeps the content type.
- Writes go to `<dir>/_staging/` and are renamed into place: a reader never
  sees a partial file, and a failed or cancelled write leaves nothing.
- **Presigned URLs** are app URLs signed with HMAC-SHA256:
  `<PUBLIC_APP_URL>/media/s/<key>?exp=<unix>&ct=<type>&cd=<disposition>&sig=<mac>`.
  The MAC covers the key, the expiry, the content type and the disposition,
  so none can be changed. The `/media/s/` handler answers with Range
  support, the signed `Content-Type` and `Content-Disposition`,
  `X-Content-Type-Options: nosniff`, a sandboxing CSP,
  `Referrer-Policy: no-referrer` and `Cache-Control: private, max-age=60`;
  an expired or tampered link gets 403.
- The key is `MEDIA_SIGNING_KEY`, or — when unset — derived from
  `APP_ENCRYPTION_KEY` with HKDF-SHA256 (`messora media signing v1`).
  Changing it invalidates outstanding links only.
- Caddy routes `/media/*` to the Go API (`deployments/caddy/Caddyfile`).

### s3

- AWS SDK for Go v2 (`service/s3`, `feature/s3/transfermanager`).
  Uploads of unknown or large size go up as multipart uploads in 16 MiB
  parts; a failed upload is aborted, so no orphaned parts are billed.
- Every object is written with server-side encryption `AES256` (SSE-S3).
  `S3_SSE=none` disables the header for services that refuse it.
- Credentials: the default chain (environment, shared files, **EC2 instance
  role**, ECS task role) unless `S3_ACCESS_KEY_ID` and
  `S3_SECRET_ACCESS_KEY` are both set.
- R2 and MinIO: set `S3_ENDPOINT`; MinIO also needs
  `S3_FORCE_PATH_STYLE=true`. With a custom endpoint, request checksums are
  only sent when an operation requires them (compatibility).
- Presigned URLs are S3's own (SigV4), with `response-content-type` and
  `response-content-disposition`.

## Serving

`storage.Serve` answers a request with an object:

- local objects: `http.ServeContent` (Content-Length, Range, conditional
  requests);
- S3 objects: streamed through the API, a single `Range` passed on to the
  bucket (audio and video seeking keeps working);
- with `ServeOptions.Redirect`: a 302 to a presigned URL instead. Only for
  routes browsers call directly — the panel's BFF fetches API routes
  server-side and would follow a redirect with its own headers. The
  `/media/p/` fallback route redirects; the authenticated media, export and
  logo routes stream.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `STORAGE_DRIVER` | `local` | `local` or `s3` |
| `STORAGE_LOCAL_DIR` | `MEDIA_DIR`, else `/var/lib/messora/media` | local objects, and scratch space (`_tmp/`) for every driver — the containers' root file system is read-only |
| `MEDIA_SIGNING_KEY` | derived from `APP_ENCRYPTION_KEY` | signs local download links (≥ 32 characters) |
| `S3_REGION` | `AWS_REGION` | bucket region (required for `s3`) |
| `S3_BUCKET_PRIVATE` | — | private bucket (required for `s3`) |
| `S3_BUCKET_PUBLIC` | — | public bucket (required for `s3`; must differ from the private one) |
| `S3_ENDPOINT` | AWS | `https://<account>.r2.cloudflarestorage.com`, `http://minio:9000` … |
| `S3_FORCE_PATH_STYLE` | `false` | `true` for MinIO |
| `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` | default chain | static credentials, both or neither |
| `S3_SSE` | `AES256` | `AES256` or `none` |
| `PUBLIC_ASSET_BASE_URL` | app `/media/p/` | CDN in front of the public bucket; `https` in production |

The API and the worker validate these at start-up and refuse to start with a
clear message (unknown driver, missing region or buckets, the same bucket
twice, half a key pair, a malformed endpoint or CDN URL, a short signing
key).

**Readiness.** `/readyz` of the API and the worker includes a `storage`
check: the local directory must be writable, or `HeadBucket` must succeed on
both buckets.

## AWS setup (staging)

Region `eu-central-1`:

| Resource | Name | Notes |
|---|---|---|
| private bucket | `messora-media-575500997783` | Block Public Access on, SSE-S3 default encryption |
| public bucket | `messora-public-575500997783` | Block Public Access on; read only by CloudFront through Origin Access Control |
| CDN | CloudFront distribution → `PUBLIC_ASSET_BASE_URL` (e.g. `https://cdn.messora.tech`) | **pending**: CloudFront is waiting for AWS account verification |
| instance role | `messora-staging-ec2` | read/write on both buckets; the SDK picks it up from the instance metadata service, so no keys are configured |

Environment of the staging stack (Dokploy *Environment* tab):

```sh
STORAGE_DRIVER=s3
S3_REGION=eu-central-1
S3_BUCKET_PRIVATE=messora-media-575500997783
S3_BUCKET_PUBLIC=messora-public-575500997783
# Once the CloudFront distribution exists:
# PUBLIC_ASSET_BASE_URL=https://cdn.messora.tech
```

Until `PUBLIC_ASSET_BASE_URL` is set, public assets are served by the app
under `/media/p/` (redirected to a presigned URL of the public bucket).

The instance role needs, on both buckets:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    { "Effect": "Allow", "Action": ["s3:ListBucket", "s3:ListBucketMultipartUploads"],
      "Resource": ["arn:aws:s3:::messora-media-575500997783", "arn:aws:s3:::messora-public-575500997783"] },
    { "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:AbortMultipartUpload", "s3:ListMultipartUploadParts"],
      "Resource": ["arn:aws:s3:::messora-media-575500997783/*", "arn:aws:s3:::messora-public-575500997783/*"] }
  ]
}
```

(`HeadBucket` is authorised by `s3:ListBucket`.) The public bucket's policy
grants `s3:GetObject` to the CloudFront service principal with the
distribution's ARN as `AWS:SourceArn` (OAC); nothing is public on the
bucket itself. Add a lifecycle rule that aborts incomplete multipart uploads
after a day as a safety net.

On EC2 with IMDSv2 inside Docker, the instance's metadata hop limit must be
at least 2, or the containers cannot reach the role's credentials.

## Tests

- Unit (`go test ./internal/platform/storage/`): keys and traversal, the
  local backend (put/get/stat/delete, failed writes, legacy layout), signed
  URLs (expiry, tampering, foreign keys), the `/media` handler, ranges,
  `Serve` for local and remote stores.
- Integration (`-tags integration`): the S3 backend against MinIO in a
  container (testcontainers) — SSE, multipart of unknown size and abort,
  presigned URLs with ranges and tampering, streaming and redirects, public
  URLs and the public fallback route.

## Still to do

- **WP1 (tenancy):** put the tenant id into request and job contexts with
  `storage.WithTenant`; every key then lands under `t/<tenant>/` and reads
  are checked against it. Drop the legacy-key allowance once no
  installation has pre-tenancy files.
- **WP3/WP8 (plans):** per-plan storage quotas (bytes per tenant, upload
  sizes); `Put` callers already know the size or count it.
- Lifecycle: import files and error reports are kept until deleted;
  exports are removed after `ExportRetention` by the scheduler.
