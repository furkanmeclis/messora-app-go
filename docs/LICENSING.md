# Licensing

Messora installations need a licence key. Without one they run a 14-day
trial; after that, and after an expired or revoked licence's grace period,
they enter **restricted mode**: sending, campaigns and automations stop,
everything else (reading, exporting, settings) keeps working, and entering
a valid key lifts it at once.

| Situation | Grace | Then |
| --- | --- | --- |
| No key | 14-day trial | restricted |
| Licence expires (`exp`) | 14 days | restricted |
| Licence revoked or replaced | 7 days from the next check-in | restricted |
| Online licence cannot reach the gate | warning after 14 days | restricted after 30 |

A clock set back cannot extend any of these: the latest time seen is kept.

## Keys

A key is `msl1.<payload>.<signature>`: base64url JSON signed with Ed25519.

| Field | Meaning |
| --- | --- |
| `customer`, `plan` | shown under Settings → License |
| `exp` | expiry; absent means never |
| `upd` | releases published after this date are not included; absent means all |
| `chk` | `online` (checks in every 6 hours) or `offline` (never) |
| `lim` | user and phone number limits; 0 means unlimited |

Messora trusts only the public keys compiled into
`internal/platform/license/keys.go`. They are never read from the
environment, since that would let anyone sign their own licence.

## The licence gate

`deployments/license-gate` is a Cloudflare Worker:

- `POST /v1/check-in` verifies the key, looks up revocations in KV and
  answers with an `msg1` envelope signed by the gate key (a Worker secret).
  The answer also carries the newest release on the stable channel.
- `/v2/...` is a read-only Docker registry over the private R2 bucket. Use
  any username and the licence key as the password. Tags released after the
  licence's update window are refused.

```sh
docker login messora-license-gate.messora-license-gate.workers.dev -u customer
# Password: the licence key
MESSORA_IMAGE=messora-license-gate.messora-license-gate.workers.dev/messora
MESSORA_WEB_IMAGE=messora-license-gate.messora-license-gate.workers.dev/messora-web
```

Installations override the gate with `LICENSE_GATE_URL`; leave it empty to
disable check-in (online licences then lapse after 30 days).

## Issuing (the issuer's Mac)

`go install ./cmd/messora-license`. Signing keys live in the login Keychain
(`Messora License Signing Key`, `Messora License Gate Key`). The ledger and
settings live in `~/Library/Application Support/Messora License`.

```sh
messora-license issue --customer "Acme Ltd" --expires 2027-12-31 \
    --updates-until 2027-12-31 --check-in online --users 20
messora-license list
messora-license show lic_…             # send a key again
messora-license issue --customer "Acme Ltd" --replaces lic_…   # renew: the old key is superseded
messora-license revoke lic_… --note "contract ended"
messora-license publish 1.0.0 --notes "…"   # copy GHCR images into R2, announce the release
```

**Back up after every change**: `messora-license backup` writes an encrypted
bundle (scrypt + AES-256-GCM) with the keys, ledger and Cloudflare settings
to iCloud Drive/Messora. Keep the passphrase in your password manager. On a
new Mac: `messora-license restore <file>`.

The iCloud Keychain itself cannot be written from a command-line tool,
because macOS allows that only for signed apps. The bundle in iCloud Drive
takes its place.

### Rotating a key

Run `messora-license keygen` and add the new public key to `keys.go`, keeping
the old one so issued keys stay valid. Then run `messora-license gate-deploy`
and release. For the gate key, run `keygen --gate` and add the key to
`GateKeys`. Release first, then run `gate-secret`, since installations must
know the new key before the gate signs with it.
