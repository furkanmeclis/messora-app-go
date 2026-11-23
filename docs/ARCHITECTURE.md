# Messora Mimarisi

> Son güncelleme: 2026-11-22 (Phase 1 sonrası)
> İlgili: [`DATABASE_DESIGN.md`](DATABASE_DESIGN.md) · [`SECURITY.md`](SECURITY.md) · [`FEATURE_MATRIX.md`](FEATURE_MATRIX.md) · [`ROADMAP.md`](ROADMAP.md)

## 1. İlkeler

1. **Mesaj kaybı ve duplicate gönderim, hızdan önce gelir.** PostgreSQL tek doğruluk kaynağıdır. Redis yalnızca kuyruk, cache, lock ve rate limit için kullanılır.
2. **Modüler monolith.** Tek Go binary'si `api`, `worker`, `scheduler` ve `migrate` alt komutlarıyla ayrı process'ler olarak çalışır. Mikroservis yok.
3. **HTTP isteğinde ağır iş yapılmaz.** Kampanya, import, webhook işleme ve otomasyon worker'da çalışır.
4. **Tek işletme.** `tenant_id` kullanılmaz. Modül sınırları ileride tenant eklemeyi lokal bir değişiklik olarak bırakacak şekilde çizilir.
5. **Dağıtık transaction varmış gibi davranılmaz.** Meta API çağrısı ile DB yazımı atomik değildir. Bu durum açıkça bir durum makinesiyle modellenir (§6).

## 2. Bileşenler

```text
                         İnternet
                            │
                   ┌────────▼────────┐
                   │  caddy (edge)   │  otomatik HTTPS (domain varsa), tek giriş noktası
                   └─┬──────┬──────┬─┘
     /webhooks/*     │      │      │  /connection/*  (WebSocket)
     /api/v1/* (API key)    │      └──────────────┐
          ┌──────────▼┐  ┌──▼─────────────┐  ┌────▼───────────┐
          │ messora-  │  │ messora-web    │  │ centrifugo v6  │
          │ api (Go)  │◄─┤ Next.js 16 BFF │  │ (realtime)     │
          └─┬───┬─────┘  │ session cookie │  └────▲───────────┘
            │   │ iç ağ  └────────────────┘       │ HTTP API publish
            │   │                                 │
   ┌────────▼┐ ┌▼──────────────┐  ┌────────────────┴─┐
   │postgres │ │ redis (AOF)   │◄─┤ messora-worker    │── Meta Graph API
   │  18     │ │ asynq/lock/RL │  │ (asynq server)    │── giden webhook'lar
   └────▲────┘ └──────▲────────┘  └───────────────────┘
        │             │           ┌───────────────────┐
        └─────────────┴───────────┤ messora-scheduler │ (leader lock)
                                  └───────────────────┘
   messora-init (tek seferlik): secret üretimi → messora-migrate (tek seferlik): goose up
```

| Servis | İmaj | Görev |
|---|---|---|
| `caddy` | `caddy:2.11.9` | TLS, yönlendirme. Panel → web, `/api/v1` ve `/webhooks` → api, `/connection` → centrifugo |
| `messora-web` | Node 26 LTS + Next.js 16.4 standalone | Panel UI ve BFF. Browser oturum cookie'sinin sahibi |
| `messora-api` | distroless, non-root | REST API, Meta webhook alımı |
| `messora-worker` | aynı binary | Asynq kuyrukları, outbox relay |
| `messora-scheduler` | aynı binary | Periyodik işler (leader lock ile) |
| `messora-migrate` | aynı binary | `goose up`, ardından çıkar |
| `messora-init` | aynı binary | İlk çalıştırmada secret üretir (§11), ardından çıkar |
| `postgres` | `postgres:18` | Kalıcı veri |
| `redis` | `redis:8` | AOF açık, `noeviction` |
| `centrifugo` | `centrifugo/centrifugo:v6.10.3` | WebSocket |

> **caddy**, **init** ve **migrate** neden ayrı servis: Caddy olmadan Meta webhook'u (HTTPS zorunlu), Centrifugo WebSocket'i ve panel aynı origin altında toplanamaz. Ayrıca bir self-host kullanıcısına ayrı bir reverse proxy kurdurmak "tek komutla kurulum" hedefini bozar. Kullanıcının kendi proxy'si varsa `caddy` servisi Compose profiliyle kapatılabilir.

## 3. Frontend: Next.js BFF

Frontend **Next.js 16 (App Router, `output: 'standalone'`)** ile yazılır ve BFF (backend-for-frontend) görevi görür.

**Gerekçe:**
- Oturum token'ı ve Centrifugo secret'ları tarayıcı JS'ine hiç ulaşmaz.
- Sayfa verisi Server Components ile sunucuda çekilir, interaktif parçalar `'use client'` bileşenleridir.
- Go API yalnızca iç ağdan veya API key ile erişilebilir kalır.

**Yapı taşları:**

| Konu | Yaklaşım |
|---|---|
| UI bileşenleri | shadcn/ui (Radix tabanı, `radix-nova` stili), CLI ile üretilir (`components.json`) |
| Tema | Messora tasarım token'ları (`globals.css`: nötr yüzeyler, indigo marka rengi), `next-themes` ile light/dark/system |
| Veri okuma | Server Component → `lib/server/api.ts` (server-only, BFF secret + oturum token'ı ekler) |
| Veri yazma | Server Action + `useActionState`. Doğrulama hataları API'nin `details[]` alanından forma eşlenir |
| Bildirimler | Server Action sonucu ve realtime event'ler → `sonner` |
| Tipler | OpenAPI 3.1 spec'ten üretilir (`openapi-typescript`) |
| Gerçek zamanlı | Centrifugo JS SDK, tek merkezi bağlantı (Phase 6) |

### 3.1 Güven sınırı (Next ↔ Go)

- **Yetki merkezi Go'dur.** Next hiçbir yetki kararı vermez, yalnızca UI'da gizleme yapar. Her istek Go'da izin kontrolünden geçer.
- Oturum token'ı: 32 byte rastgele değer, opak. Go yalnızca `sha256(token)` saklar (`user_sessions`). Tarayıcıda `__Host-messora_session` cookie'sinde durur (`HttpOnly; Secure; SameSite=Lax; Path=/`).
- Next her Go çağrısında `Authorization: Bearer <session_token>`, `X-Request-ID`, `X-Forwarded-For` (gerçek istemci IP'si) ve `X-Messora-BFF: <BFF_SHARED_SECRET>` gönderir.
- Go, **session token'larını yalnızca geçerli `X-Messora-BFF` başlığı taşıyan isteklerde kabul eder.** Caddy üzerinden doğrudan `/api/v1`'e gelen dış istekler yalnızca API key ile doğrulanabilir. Böylece sızan bir session token'ı dışarıdan tek başına kullanılamaz. İstemci IP'si de yalnızca BFF'den gelirse güvenilir sayılır (brute-force koruması ve audit log için).
- **CSRF:** Tarayıcı yalnızca Next ile konuşur. Server Action'lar Next'in yerleşik Origin/Host kontrolüne sahip. Route Handler'larda (`/bff/*`: upload, inbox gönderimi) `Origin` ve `Sec-Fetch-Site` kontrol edilir, cookie `SameSite=Lax`. Go API cookie ile doğrulamadığı için CSRF yüzeyi yok.
- Centrifugo connection ve subscription token'larını **Go** üretir (HMAC secret yalnızca Go'da ve Centrifugo'da). Next bunları kendi oturumu üzerinden Go'dan alıp tarayıcıya verir.

## 4. Go uygulama yapısı

```text
messora-app-go/
├── cmd/messora/main.go          # alt komutlar: api | worker | scheduler | migrate | init | setup | version
├── internal/
│   ├── app/                     # bağımlılıkları kuran composition root (process başına)
│   ├── config/                  # env + secrets dosyası yükleme, doğrulama
│   ├── platform/
│   │   ├── database/            # pgxpool, tx helper, goose embed
│   │   ├── redis/
│   │   ├── queue/               # asynq client/server sarmalayıcı, task tipleri, retry politikası
│   │   ├── outbox/              # transactional outbox yazıcı + relay
│   │   ├── centrifugo/          # gocent HTTP API istemcisi (publish, info)
│   │   ├── crypto/              # AES-256-GCM (anahtar sürümlü), argon2id, token üretimi
│   │   ├── httpx/               # chi middleware: request id, recover, log, auth, rate limit, idempotency
│   │   ├── logger/              # slog JSON, hassas alan redaction
│   │   ├── metrics/             # prometheus registry
│   │   ├── ratelimit/           # Redis GCRA (Lua)
│   │   ├── lock/                # Redis lease lock (scheduler leader)
│   │   ├── mailer/              # SMTP
│   │   ├── netguard/            # SSRF korumalı HTTP client
│   │   └── storage/             # local disk (varsayılan) | S3 uyumlu
│   ├── api/                     # OpenAPI'den üretilen tipler + chi sunucu arayüzü
│   ├── db/                      # sqlc ile üretilen uygulama sorguları
│   ├── modules/
│   │   ├── identity/            # kurulum, login, oturum, 2FA, parola, kullanıcı, davet, rol, API key
│   │   ├── settings/
│   │   ├── audit/
│   │   ├── system/              # kurulum durumu
│   │   ├── whatsapp/            # Meta Graph client (DB'siz adaptör) + inbound webhook alımı + imza doğrulama
│   │   ├── channels/            # whatsapp_accounts / phone_numbers kayıtları, token şifreleme
│   │   ├── templates/
│   │   ├── media/
│   │   ├── contacts/            # contacts, tags, groups, notes, custom fields, import/export
│   │   ├── segments/            # kural AST + güvenli SQL derleyici
│   │   ├── messaging/           # giden mesaj durum makinesi, dispatcher, rate limit, uzlaştırma
│   │   ├── campaigns/
│   │   ├── conversations/       # inbox, atama, 24h pencere
│   │   ├── automation/
│   │   ├── webhooks/            # giden webhook abonelikleri + teslimat
│   │   ├── notifications/
│   │   └── realtime/            # kanal yetkilendirme, token endpoint'leri, event yayınlama
│   └── shared/                  # ortak tipler (ID, sayfalama, hata), phone (E.164)
├── api/openapi.yaml             # OpenAPI 3.1 — tek kaynak (spec-first)
├── migrations/                  # goose SQL migration'ları
├── queries/<module>/*.sql       # sqlc sorguları
├── frontend/                    # Next.js uygulaması
├── deployments/{caddy,centrifugo,docker}/
├── tests/{integration,e2e,load}/
└── docs/
```

**Notlar:**
- auth, users, roles ve api_keys tek bir `identity` modülünde birleştirildi. Ortak tablolara ve kurallara (ör. "son owner kaldırılamaz") bağlılar; ayrı modüller döngüsel bağımlılık doğururdu.
- Yetkilendirme spec'te tanımlıdır. `platform/apiguard`, her isteği gömülü spec'teki operasyonla eşleştirir ve operasyonun `security` gereksinimini uygular (`panel`, `session[izinler]`, `apiKey[scope'lar]`). Spec'te açık `security` alanı olmayan operasyon varsa API hiç başlamaz.
- İstek gövdeleri spec'e göre doğrulanır (kin-openapi). Integration testleri her yanıtı spec'e karşı doğrular (contract test).
- `channels` modülü eklendi. Böylece `whatsapp` saf bir Meta adaptörü kalır (DB'ye dokunmaz, mock'lanması kolay), hesap ve numara kayıtları ayrı bir modülde durur.
- `media`, `notifications`, `realtime` eklendi.
- `api/openapi.yaml` eklendi.

**Modül içi düzen:** `handler.go` (HTTP), `service.go` (iş kuralları), `store/` (sqlc üretimi), `tasks.go` (asynq handler'ları), `*_test.go`. Modüller birbirinin `store`'una dokunmaz, yalnızca servis arayüzlerini çağırır.

**Veri erişimi:** sqlc + pgx v5. Dinamik sorgular (segment, liste filtreleme ve sıralama) için küçük, whitelist tabanlı bir query builder kullanılır. Değerler her zaman `$n` parametresi olarak geçer. ORM yok.

**OpenAPI:** Spec-first. `oapi-codegen` chi için strict server arayüzü ve tipleri üretir. Böylece spec ile kod derleme zamanında uyuşmak zorunda kalır. Frontend tipleri aynı spec'ten `openapi-typescript` ile üretilir.

## 5. API standartları

- Base path `/api/v1`. Webhook'lar `/webhooks/whatsapp` altında (versiyonsuz, Meta'ya verilen sabit URL).
- Başarılı yanıt formatı: `{"data": …, "meta": {"request_id": "…", "next_cursor": "…"}}`
- Hata formatı: `{"error": {"code": "VALIDATION_ERROR", "message": "…", "details": [{"field": "phone", "code": "invalid_e164"}]}, "meta": {"request_id": "…"}}`
- **Pagination:** Büyük tablolarda (contacts, messages, recipients, audit) cursor tabanlı (keyset). Küçük listelerde `page`/`per_page`.
- **Filtreleme ve sıralama:** Whitelist edilmiş alanlar, `sort=-created_at`.
- **Idempotency-Key:** Tüm POST'larda opsiyonel, mesaj gönderen endpoint'lerde (`/messages`, `/campaigns/{id}/start`) önerilir. `(key, principal)` ikilisi ile 24 saat saklanır. Aynı key ve farklı body gelirse `409`.
- **Rate limit:** Principal başına (session veya API key) Redis GCRA. `429` + `Retry-After`.
- **Request ID:** `X-Request-ID` header'ı (gelmezse UUIDv7 üretilir) yanıta, loglara ve job payload'larına yazılır.
- **Auth:** Session (yalnızca BFF üzerinden) veya `Authorization: Bearer msk_live_…` API key. API key'ler scope taşır. Scope'lar izin adlarıyla aynıdır (`messages.send`, `contacts.read`, …).

## 6. Giden mesaj güvenilirliği (çekirdek tasarım)

Inbox yanıtı, API, otomasyon ve kampanya dahil **her giden mesaj** aynı boru hattından geçer: `messaging.Dispatcher`.

### 6.1 Durum makinesi

```text
             ┌──────────── cancel/pause ────────────┐
             ▼                                      │
 queued ──claim──► sending ──2xx+wamid──► accepted ──webhook──► sent ─► delivered ─► read
   ▲                 │                                  │
   │ retry_wait ◄────┤ kesin "gönderilmedi" (dial hatası, 429/130429, kalıcı olmayan 4xx-öncesi)
   │ (backoff)       │
   │                 ├──kalıcı hata (131026, 132xxx, 131047…)──► failed
   │                 │
   │                 └──belirsiz (timeout, EOF, bağlantı yanıt öncesi koptu)──► unknown
   │                                                                │
   └─────── operatör "yine de tekrar gönder" (bilinçli onay) ◄──────┤ webhook gelirse ► sent/delivered/read/failed
                                                                    └ süre aşımı ► unknown (kalıcı, UI'da işaretli)
 held: kampanya duraklatıldığında claim edilmiş ama gönderilmemiş mesajlar
```

### 6.2 Kurallar

1. **Tek kazanan claim:** `UPDATE messages SET state='sending', attempts=attempts+1, lease_until=now()+$lease WHERE id=$1 AND state IN ('queued','retry_wait') RETURNING …`. Aynı task iki worker'da çalışsa bile yalnızca biri gönderir. Asynq task ID'si `msg:<message_id>` olduğu için aynı mesaj kuyruğa iki kez giremez.
2. **Hata sınıflandırması** (`whatsapp/errors.go`), Meta hata kodu ve ağ hatası tipine göre yapılır:
   - *Kesin gönderilmedi:* DNS/dial/TLS handshake hatası, istek yazılmadan oluşan iptal, HTTP 429, Meta `130429` (throughput), `131056` (pair rate), `80007`. Sonuç: `retry_wait` ve exponential backoff (taban 2 sn, çarpan 2, ±%20 jitter, üst sınır 10 dk, en fazla 8 deneme). Sonrasında `failed` (dead letter).
   - *Kalıcı:* Geçersiz alıcı, template parametre hatası, 24h pencere dışı (`131047`), izin veya token hatası (`190`, `10`, `200` → ayrıca hesap `needs_attention` olur ve kampanya otomatik duraklatılır). Sonuç: `failed` + `error_code` + `error_detail`.
   - *Belirsiz:* İstek gönderildikten sonra timeout, yanıt okunurken bağlantı kopması, gövdesiz 5xx. Sonuç: **`unknown`. Otomatik olarak asla tekrar gönderilmez.**
3. **Uzlaştırma:** Her gönderimde `biz_opaque_callback_data = "m:<message_id>"` alanı geçilir. Meta bu değeri durum webhook'unda geri döndürür. `unknown` durumundaki bir mesaj için webhook gelirse `wamid` bağlanır ve durum ilerletilir. `MESSAGE_UNKNOWN_TIMEOUT` (varsayılan 30 dk) içinde webhook gelmezse mesaj `unknown` olarak kalır. UI'da "teslim durumu belirsiz" olarak işaretlenir, operatör bilinçli olarak yeniden gönderebilir veya `failed` olarak kapatabilir. *(Meta'nın bu alanı durum webhook'larında döndürdüğü Phase 3'te resmî dokümandan ve mock testlerinden doğrulanacak. Meta'nın `/messages` için idempotency anahtarı sunmadığı varsayılıyor; bu da aynı fazda doğrulanacak.)*
4. **Worker çökmesi:** Lease süresi dolmuş `sending` kayıtlarını scheduler'daki reaper **`unknown`** durumuna taşır, `queued`'ya değil. İsteğin Meta'ya gidip gitmediği bilinemez. Gerçek sonuç, 3. maddedeki callback data ile kendiliğinden uzlaşır.
5. **Graceful shutdown:** SIGTERM gelince asynq yeni task almayı bırakır, devam eden HTTP çağrılarının bitmesi için `SHUTDOWN_TIMEOUT` (varsayılan 30 sn, Meta client timeout'u 20 sn) beklenir. Bitmeyenler asynq tarafından kuyruğa iade edilir. Bu task'ler tekrar çalıştığında 1. maddedeki claim, durumu `sending` görüp çakışmayı engeller. Lease bitince 4. madde devreye girer.
6. **Durum webhook'ları monoton işlenir:** Sıralama `accepted(1) < sent(2) < delivered(3) < read(4)`. Güncelleme yalnızca `rank` artıyorsa yapılır. `failed`, ancak `delivered` öncesindeyse uygulanır. Her event `message_status_events` tablosuna `(message_id, status)` unique olarak yazılır. Sayaçlar yalnızca gerçek bir geçiş olduğunda artırılır. Bu, sıra dışı gelen durum güncellemelerini ve sayaç yarışlarını çözer.
7. **Opt-out kontrolü gönderimden hemen önce bir kez daha yapılır.** Kampanya oluşturma ile gönderim arasında opt-out olan alıcı `skipped_opted_out` olur.
8. **24h pencere:** Serbest metin ve medya mesajları (template olmayanlar) yalnızca konuşmanın `customer_window_expires_at > now()` olduğu durumda gönderilir. Aksi halde istek API'de `422 SESSION_WINDOW_CLOSED` ile reddedilir (Meta'ya hiç gitmez). Kampanyada bu alıcılar `skipped_no_session` olur.

### 6.3 Rate limiting

- **Numara başına:** Redis GCRA (Lua script, atomik). Limit `phone_numbers.throughput_mps` değerinden gelir (varsayılan 80, Meta'nın varsayılan throughput'u). Token alınamazsa task gecikmeli olarak yeniden planlanır. Bu, başarısız deneme sayılmaz (asynq `IsFailure` ile ayrıştırılır).
- **Meta 130429 / 131056:** Numara (veya alıcı) için Redis'te bir "soğuma" anahtarı set edilir ve kampanya dispatcher'ı penceresini daraltır (AIMD: hata gelince yarıya iner, başarıda yavaşça artar).
- **WABA messaging limit tier'ı** (24 saatte ulaşılabilecek benzersiz kullanıcı sayısı): Kampanya başlatılırken uyarı verilir. Limit aşımı hataları kampanyayı `paused_limit` durumuna alır.

## 7. Kampanya motoru

```text
POST /campaigns/{id}/start
  └─ TX: status draft|scheduled → preparing ; outbox(campaign.materialize)
worker campaign:materialize        (resumable: campaigns.materialize_cursor)
  └─ hedef kitle keyset sayfalarıyla (1000'er) çözülür: segment SQL | grup | yüklenen liste
     INSERT campaign_recipients … ON CONFLICT (campaign_id, phone_e164) DO NOTHING   ← dedupe
     opt-out / geçersiz numara → skipped_* olarak yazılır (görünür ve sayılır)
  └─ TX: status → running ; outbox(campaign.dispatch)
worker campaign:dispatch           (task id: camp:<id>:dispatch — kampanya başına tek)
  └─ döngü: status running mi? değilse çık
     uçuştaki (queued+sending) < pencere ise:
       SELECT … FROM campaign_recipients WHERE campaign_id=$1 AND state='pending'
         ORDER BY id LIMIT $n FOR UPDATE SKIP LOCKED
       TX: messages INSERT (queued) + recipient → queued + message_id bağla ; outbox(message.send ×n)
     kendini ProcessIn(1s) ile yeniden planla ; hepsi bitti → finalize
worker message:send  → §6
```

- **Redis şişmez:** Tüm alıcılar asla kuyruğa basılmaz. Kuyrukta en fazla `pencere` kadar (varsayılan `throughput_mps × 10`) task bulunur. 100k alıcılı kampanya Redis'te birkaç yüz küçük task'tir (payload yalnızca ID).
- **Pause:** `running → paused`. Dispatcher durur. Claim anında kampanyası duraklatılmış olan mesaj `held` olur. **Resume:** `paused → running` + dispatch. Dispatcher önce `held` mesajları yeniden kuyruğa alır. **Cancel:** `→ cancelled`. `pending` ve `held` kayıtlar `cancelled` olur.
- **Planlanmış kampanya:** Scheduler her dakika `UPDATE … SET status='preparing' WHERE status='scheduled' AND scheduled_at <= now() RETURNING id` çalıştırır ve outbox'a yazar.
- **İstatistik:** `campaign_stats` satırı (pending, queued, accepted, sent, delivered, read, failed, unknown, skipped) durum geçişleriyle aynı transaction'da artırılır. Webhook başına COUNT sorgusu yok. 80 mps'de tek satır güncellemesi PostgreSQL için sorun değildir. 1000 mps senaryosunda sayaç sharding'i (`campaign_stats_shards`) roadmap'tedir.
- **Event log:** `campaign_events` (started, paused, resumed, cancelled, completed, auto_paused + sebep, kullanıcı).

## 8. Webhook alımı (Meta → Messora)

```text
POST /webhooks/whatsapp
 1. Body'yi ham haliyle oku (üst sınır 2 MiB)
 2. X-Hub-Signature-256 doğrula (app secret: DB'de şifreli, bellekte 60 sn cache) — geçersizse 401
 3. INSERT whatsapp_webhook_events(payload jsonb, body_sha256 UNIQUE, received_at) ON CONFLICT DO NOTHING
 4. asynq enqueue webhook:process (task id = event id) — Redis down ise sorun değil (adım 6)
 5. 200 OK  (hedef < 50 ms; DB yazılamazsa 500 → Meta tekrar dener)
 6. reaper: processed_at IS NULL AND received_at < now()-1m olanları yeniden kuyruğa alır
worker webhook:process (idempotent):
  - mesaj: INSERT messages … ON CONFLICT (wamid) DO NOTHING RETURNING id
      → yalnızca yeni satır oluştuysa aynı TX'te: konuşma upsert, 24h pencere güncellemesi,
        outbox(automation.evaluate, webhook.deliver, realtime.publish, media.download)   ← yan etkiler yalnızca yeni mesajda
  - durum: §6.2 madde 6 ; biz_opaque_callback_data ile unknown uzlaştırma
  - template durum güncellemesi, numara kalite güncellemesi, hesap event'leri
  - STOP / opt-out anahtar kelimeleri → consent kaydı (yapılandırılabilir)
```

- `GET /webhooks/whatsapp`: `hub.mode=subscribe` **ve** `hub.verify_token` değeri `META_VERIFY_TOKEN` ile sabit zamanlı karşılaştırmada eşleşirse `hub.challenge` döner, aksi halde 403.
- Tek işletme olduğu için URL'de tenant token'ı yoktur. Birden fazla Meta App kullanılırsa (nadir), imza her kayıtlı app secret ile denenir.

## 9. Outbox

`outbox_events(id, topic, payload, available_at, published_at, attempts)` tablosu, iş durumu değişikliğiyle **aynı transaction'da** yazılır. Worker'daki relay:

- `LISTEN outbox` ile uyanır, ayrıca 1 sn'lik yedek polling yapar.
- `SELECT … WHERE published_at IS NULL AND available_at <= now() ORDER BY id LIMIT 200 FOR UPDATE SKIP LOCKED` ile kayıtları alır.
- Asynq'e `TaskID = outbox:<id>` ile basar (tekrar basılırsa `ErrTaskIDConflict` → zaten yayınlanmış sayılır) veya Centrifugo'ya publish eder, ardından `published_at` set edilir.
- Teslim garantisi en az bir kezdir (at-least-once), tüm tüketiciler idempotent. Yayınlanmış kayıtlar 7 gün sonra silinir.

*Değerlendirilen alternatif:* Centrifugo v6'nın yerleşik PostgreSQL outbox consumer'ı. Yalnızca realtime için işe yarıyor ve Centrifugo'ya DB erişimi gerektiriyor. Tek bir relay hem kuyruk hem realtime için yeterli olduğundan tercih edilmedi.

## 10. Kuyruklar ve worker

| Kuyruk | Öncelik | Task'ler |
|---|---|---|
| `critical` | 6 | `webhook:process`, `message:send` (inbox/API/otomasyon kaynaklı) |
| `campaigns` | 3 | `campaign:materialize`, `campaign:dispatch`, `message:send` (kampanya) |
| `default` | 3 | `automation:run`, `automation:resume`, `webhook:deliver`, `media:download`, `template:sync` |
| `bulk` | 1 | `contacts:import`, `contacts:export`, `segment:preview` |

- Inbox yanıtları kampanya trafiğinin arkasında beklemesin diye kampanya mesajları ayrı kuyruğa gider (aynı numara rate limiter'ını paylaşırlar).
- Tek worker process'i tüm kuyrukları dinler (`WORKER_QUEUES` ile seçilebilir). Ölçeklemek için aynı imaj farklı `WORKER_QUEUES` değerleriyle çoğaltılır.
- Retry: Task tipine göre `RetryDelayFunc`. Kalıcı hatalar `asynq.SkipRetry` ile işaretlenir. Asynq "archived" kuyruğu dead-letter görevi görür. Mesajlar için asıl kayıt DB'deki `failed` durumudur, UI'dan yeniden denenebilir.
- **Asynq notu:** v0.26 (1.0 öncesi), son commit 2026-11-11, aktif. Sürüm `go.mod`'da sabitlenecek ve asynq'e özgü kod `platform/queue` arkasında tutulacak. Böylece gerekirse değiştirilebilir. Asynq Redis'te `noeviction` gerektirir.

## 11. Scheduler

- Asynq Scheduler birden fazla instance'ta duplicate üretir. Bu yüzden scheduler process'i Redis lease lock (`SET NX PX` + yenileme) ile **leader** seçer, yalnızca leader tick atar.
- İkinci güvenlik katmanı: her tick task'inin ID'si zaman dilimini içerir (`sched:campaigns_due:202611221905`), bu da duplicate'i asynq'te engeller.
- Üçüncü katman: işler DB'de atomik claim ile yapılır.

| İş | Periyot |
|---|---|
| Vadesi gelmiş planlı kampanyalar | 1 dk |
| `sending` lease reaper, webhook event reaper, outbox reaper | 1 dk |
| `unknown` mesaj zaman aşımı | 5 dk |
| Template senkronizasyonu | 30 dk |
| Numara kalite / limit tier yenileme | 6 sa |
| Süresi dolmuş oturum, idempotency, outbox temizliği | 1 sa |

## 12. Gerçek zamanlı (Centrifugo v6)

- Centrifugo config'ini `messora init` üretir (`provision.CentrifugoConfig`): token ve subscription token HMAC secret'ı, `allowed_origins` = `PUBLIC_APP_URL`, kapalı admin UI, namespace'ler. Client publish yoktur; token'sız abonelik yoktur (namespace'lerde `allow_subscribe_for_client` kapalı).
- **Token'lar:** `POST /api/v1/realtime/connection-token` ve `POST /api/v1/realtime/subscription-token` (yalnızca `session` şeması, API key'ler reddedilir). Go, gocent'in `token` paketiyle HS256 JWT üretir, ömür 10 dakika. SDK süresi dolmadan yenisini ister; devre dışı bırakılan kullanıcı veya geri alınan yetki en geç bu sürede erişimini kaybeder.
- **Kanallar ve izinler** (`internal/modules/realtime/channels.go`, tek kaynak):
  - `personal:#<user_id>` ve `system:notifications`: connection token'ın `subs` claim'i ile sunucu tarafı abonelik.
  - `inbox:all`: `conversations.read` veya `conversations.read_all`.
  - `conversation:<id>`: şimdilik `conversations.read_all`. Agent'ın kendisine atanmış konuşmalara aboneliği Phase 7'de atamalarla gelir.
  - `campaign:all` ve `campaign:<id>`: `campaigns.read`.
  - Bilinmeyen veya bozuk her kanal `403 CHANNEL_FORBIDDEN`. Centrifugo token'daki `sub` ve `channel`'ı ayrıca doğrular: başka kullanıcının ya da başka kanalın token'ı reddedilir.
- **Yayınlama:** Üretici, iş değişikliğiyle aynı transaction'da `realtime.Publish(ctx, tx, kanal, event)` çağırır. Bu, `rt:<kanal>` topic'li bir outbox satırıdır. Worker'daki relay `rt:` prefix'ini Centrifugo publish API'sine yönlendirir, idempotency key `outbox:<id>`'dir (tekrar eden publish client'a iki kez ulaşmaz).
- **Event'ler** sürümlüdür (`{"type": …, "v": 1, …}`) ve yalnızca ID ve durum taşır, mesaj gövdesi taşımaz. Client ayrıntıyı API'den kendi yetkisiyle çeker.
  - `message.created`: gelen mesaj (webhook) veya kuyruğa alınan giden mesaj. `message_id`, `contact_id`, `phone_number_id`, `direction`, `message_type`, `status`, `at`.
  - `message.status`: gönderim sonucu (sent / failed / unknown) veya webhook'tan gelen gerçek bir durum değişikliği. Değişmeyen (eski sıradaki) status yayınlanmaz.
  - `campaign.updated`: kampanya motoru için tanımlı (`CampaignEvent`), Phase 5 kampanya kodu geldiğinde `campaign:all` ve `campaign:<id>`'ye yayınlanır.
- Event'ler kalıcı veri değildir. `campaign`, `conversation`, `inbox` ve `system` namespace'lerinde kısa history (50–100 kayıt, 2–5 dk) ve `force_recovery` açıktır.
- **Frontend:** `RealtimeProvider` (`frontend/src/components/realtime.tsx`) panel layout'unda tek bağlantı açar: `wss://<origin>/connection/websocket` (Caddy `/connection/*` → centrifugo). Token'lar Server Action'larla BFF üzerinden alınır, session token ve secret'lar tarayıcıya ulaşmaz. SDK kopmada backoff ile kendiliğinden yeniden bağlanır. Abonelik `wasRecovering && !recovered` ile gelirse (history dolmuş, Centrifugo yeniden başlamış) sayfa `router.refresh()` ile API'den yeniden çekilir. `<RealtimeRefresh channel types>` sayfalara polling'siz yenileme sağlar. CSP `connect-src` yalnızca aynı origin'in `ws(s)://` adresine izin verir.
- Centrifugo memory engine ile tek node çalışır. Çok node gerekirse Redis engine'e geçilir (roadmap).

## 13. Gözlemlenebilirlik

- **Log:** `log/slog` JSON. Alanlar: `request_id`, `user_id`, `op`, `duration_ms`, `err`, `job_id`, `task_type`. Redaction katmanı `token`, `secret`, `password`, `authorization`, mesaj `body` ve telefon numarasını (son 4 hane hariç) maskeler.
- **Metrikler:** `:9100/metrics` (iç port, Caddy'den dışarı açılmaz). İçerik: HTTP istek sayısı ve gecikmesi (route şablonu etiketiyle), asynq kuyruk boyutları (`asynq/x/metrics`), task başarı/başarısızlık sayıları, Meta API hataları (koda göre), kampanya ilerlemesi, webhook işleme süresi ve gecikmesi, pgxpool istatistikleri, Redis ping durumu, outbox birikimi.
- **Health:** `/healthz` (process ayakta mı), `/readyz` (Postgres, Redis, migration sürümü). Worker ve scheduler için `:9100/healthz` (asynq ve DB bağlantısı dahil).
- **Tracing:** OpenTelemetry SDK. `OTEL_EXPORTER_OTLP_ENDPOINT` set edilmemişse kapalıdır (no-op). Trace context, asynq task header'larında taşınır.

## 14. Yapılandırma ve secret'lar

- Kaynak sırası: ortam değişkenleri → `MESSORA_SECRETS_FILE` (init'in ürettiği dosya, yalnızca boş kalan değişkenleri doldurur). Başlangıçta doğrulanır, eksik zorunlu değer varsa process açık bir hata mesajıyla çıkar.
- **İlk çalıştırma** (`messora-init`): `messora-secrets` volume'unda `secrets.env` yoksa üretir. İçerik: `APP_ENCRYPTION_KEY` (32 byte), `BFF_SHARED_SECRET`, `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `CENTRIFUGO_API_KEY`, `CENTRIFUGO_HMAC_SECRET`, `META_VERIFY_TOKEN`. Bunlara ek olarak Centrifugo config dosyası üretilir. Postgres `POSTGRES_PASSWORD_FILE` ile okur. Kullanıcı `.env`'de bir değer verirse o değer kullanılır.
- `APP_ENCRYPTION_KEY` kaybedilirse şifreli Meta token'ları açılamaz. Yedekleme dokümanında bu açıkça belirtilecek.
- Meta kimlik bilgileri (`META_*`) `.env` üzerinden verilebilir veya ilk kurulumdan sonra panelden girilebilir. Her iki durumda da DB'de şifreli saklanır, env değerleri yalnızca ilk tohumlama için kullanılır.

## 15. Test stratejisi (özet)

- **Unit:** Servisler; dış bağımlılıklar arayüzlerle soyutlanır.
- **Integration:** testcontainers-go ile gerçek Postgres 18 ve Redis 8 kullanılır.
- **Meta mock:** `internal/modules/whatsapp/whatsapptest` paketi. Programlanabilir yanıtlar (başarı, hata kodları, gecikme ve timeout, imzalı webhook üretimi) içerir. `META_GRAPH_BASE_URL` ile yönlendirilir. `APP_ENV=test` veya `MESSAGING_DRY_RUN=true` iken gerçek Graph host'una istek **kod seviyesinde engellenir**.
- **E2E:** Compose stack + Playwright (ilk kurulum, login, contact ekleme, CSV import, segment, template sync, kampanya oluşturma/başlatma, webhook ile durum, pause/resume).
- **Yük testi:** 100k alıcılı kampanya mock Meta'ya karşı çalıştırılır. Ölçülenler: duplicate sayısı (hedef 0), kayıp sayısı (hedef 0), worker kill/restart sırasında belirsiz kalan mesaj sayısı.

## 16. Sabitlenmiş sürümler (2026-11-22 itibarıyla doğrulandı; Phase 1'de güncellendi)

| Bileşen | Sürüm | Kaynak / not |
|---|---|---|
| Go | **1.27.4** | go.dev/dl (yerel makinede 1.26.4 var, güncellenmeli; build Docker'da 1.27.4 ile yapılır) |
| chi | v5.5.0 | GitHub release 2026-10-14 |
| pgx | **v5.13.0** | 2026-11-11. Güvenlik düzeltmesi içeriyor (GHSA-w9rp-vfrh-q8vv). Phase 0'da GitHub "latest release" v5.11.0 gösteriyordu; Go modülleri için kaynak artık modül proxy'si (`go list -m -versions`). |
| sqlc | v1.31.1 | 2026-04-22 |
| goose | v3.30.1 | 2026-11-12. Seçim gerekçesi: SQL dosyaları, embed, sqlc'nin goose formatını doğrudan okuması. golang-migrate v4.20.1 da değerlendirildi. |
| asynq | v0.26.0 | 2026-02-03, son commit 2026-11-11 |
| go-redis | v9.23.0 | 2026-10-05 |
| gocent | v4.0.0 | 2026-11-06 |
| Centrifugo | v6.10.3 | 2026-11-08 |
| centrifuge-js | 5.8.0 | 2026-10-20 |
| oapi-codegen | v2.9.0 | 2026-11-21 (bir günlük; P1'de bir önceki patch ile karşılaştırılacak) |
| prometheus/client_golang | v1.25.0 | |
| OpenTelemetry Go | v1.47.0 | |
| nyaruka/phonenumbers | v1.8.1 | libphonenumber portu |
| excelize | v2.11.1 | streaming reader/writer |
| pquerna/otp | v1.5.0 | Son push 2025-08. Küçük ve stabil bir kütüphane, P2'de güvenlik bildirimleri kontrol edilecek. |
| go-webauthn | v0.18.2 | passkey (P10) |
| testcontainers-go | v0.45.0 | |
| PostgreSQL | **18** (18.7) | 19.0 yeni çıktı; olgunlaşması için 18 seçildi (EOL 2030-11). Yerleşik `uuidv7()` desteği var. |
| Redis | **8.10.3** | 8.12.0 bugün çıktı ama Docker Hub'da henüz imajı yok; Hub'daki en yeni kararlı sürüm sabitlendi. Lisans: AGPLv3 seçeneği OSI onaylı. Valkey 9.2 (BSD) uyumlu alternatif. |
| Next.js | **16.4.6** | 2026-11-09 |
| React | 19.3.1 | |
| TypeScript | **6.0.3** | 7.0.2 mevcut, ama Go ile yazılmış yeni derleyicinin paketi klasik JS derleyici API'sini sunmuyor (yalnızca `unstable/*` export'ları var). Next.js'in build-time tip kontrolü ve typescript-eslint bu API'ye ihtiyaç duyuyor. 6.x ile kalındı. |
| Tailwind CSS | 4.3.5 | |
| ESLint | 10.15.0 + eslint-config-next 16.4.6 | |
| lucide-react | 1.77.0 | |
| shadcn (CLI) | 4.23.0 | `radix-ui` 1.8.1, `cn` 0.5.2, `next-themes` 0.4.6 |
| Node.js | **26 LTS** (Lithium, 26.13.1) | |
| golangci-lint | v2.15.0 | Docker'da çalışır |
| govulncheck | v1.10.0 | |
| gitleaks | v8.30.1 | Docker'da çalışır |
| GitHub Actions | checkout/setup-go/setup-node v7 | |
| Caddy | 2.11.9 | |
| Meta Graph API | **v26.0** (varsayılan, `META_GRAPH_API_VERSION` ile değiştirilebilir) | Resmî sürüm tablosu: v26.0 yayın tarihi 2026-07-29, bitiş TBD |

Tüm sürümler `go.mod`, `package.json`, lockfile'lar ve Dockerfile/Compose imaj etiketlerinde sabitlenir. Docker imajları P1'de digest ile sabitlenir.

## 17. Karar kaydı

| # | Karar | Durum |
|---|---|---|
| D1 | Modüler monolith, tek binary + alt komutlar | Kabul |
| D2 | Next.js 16 BFF + shadcn/ui | Kabul |
| D3 | Opak session token + `X-Messora-BFF` güven sınırı | Önerildi |
| D4 | Tek `/api/v1`. Session yalnızca BFF'den, API key dışarıdan | Önerildi |
| D5 | Mesaj durum makinesi + `unknown` durumu + `biz_opaque_callback_data` uzlaştırması | Önerildi (P3'te Meta davranışı doğrulanacak) |
| D6 | Transactional outbox (kendi relay'imiz) | Önerildi |
| D7 | Pencereli kampanya dispatcher'ı, tüm alıcıları kuyruğa basmamak | Önerildi |
| D8 | Scheduler leader lock + zaman dilimli task ID + DB claim | Önerildi |
| D9 | goose + sqlc + pgx, ORM yok | Önerildi |
| D10 | Spec-first OpenAPI (oapi-codegen) | Önerildi |
| D11 | Caddy edge + init + migrate servisleri | Önerildi |
| D12 | PostgreSQL 18, UUIDv7 birincil anahtarlar | Önerildi |
| D13 | Açık kayıt kaldırılır, ilk kurulum token'ı + davet sistemi | Önerildi |
| D14 | Embedded Signup ve passkey sonraya ertelenir | Önerildi |
| D15 | Outbox ve zaman karşılaştırmalarında tek saat kaynağı PostgreSQL `now()` | **Uygulandı (P1).** Integration testinde host ile container arasındaki milisaniyelik saat farkı event'leri gizledi. Gecikmeler artık DB tarafında `now() + interval` olarak yazılıyor. |
| D16 | Secret'lar servis başına ayrı dosyada (Compose `volume.subpath`), her dosya yalnızca tüketicisinin UID'si tarafından okunabilir | **Uygulandı (P1)** |
| D17 | Ağlar ayrıldı: `internal` (internet yok; postgres, redis) ve `edge` | **Uygulandı (P1)** |
| D18 | Phase 1'deki kurulum durumu sayfası kimlik doğrulama olmadan açık | **Geçici.** Yalnızca bileşen sağlığını gösteriyor, secret göstermiyor. Phase 2'de ilk kurulum ve login akışının arkasına alınacak. |
