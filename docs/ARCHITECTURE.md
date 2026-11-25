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
   │                 ├──kalıcı hata (131026, 131050, 132xxx, 131047…)──► failed (failure_class)
   │                 ├──131049 (ekosistem limiti)──► deferred ──next_attempt_at (≥24 sa)──► sending
   │                 ├──izin engeli (gönderim anında)──► skipped
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
7. **İzin kontrolü gönderimden hemen önce bir kez daha yapılır.** İzin kategori bazlıdır (pazarlama / servis) ve kuralları tek yerdedir (`consent.Decide`, ayrıntı [`CONSENT.md`](CONSENT.md)): kuyruğa alırken (API, gelen kutusu, otomasyon), kampanya alıcıları oluşturulurken ve Meta çağrılmadan hemen önce uygulanır. Kuyruğa alındıktan sonra izni kalkan alıcının mesajı `skipped` olur (`failure_class = blocked_by_consent`, sebep `error_code`'da). Kesin engeller (çıkış, WhatsApp'ta pazarlamayı durdurma, ulaşılamayan numara) hiçbir ayarla aşılamaz.
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

- **Ertelenen mesajlar pencerede yer tutmaz:** Meta'nın 131049 ile geri çevirdiği mesaj `deferred` olur ve `next_attempt_at`'e (en az 24 saat) kadar uçuştaki sayıma girmez; dispatcher diğer alıcılarla pencereyi doldurmaya devam eder. Kampanya, ertelenen mesajlar gönderilene ya da vazgeçilene kadar `running` kalır (bu sürede dakikada bir tık).
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
  - STOP / opt-out anahtar kelimeleri → kategori bazlı izin değişikliği + consent_events + `subscription_request` tetikleyicisi (listeler firma ayarı; CONSENT.md)
  - `user_preferences` (WhatsApp'ta pazarlamayı Durdur/Devam) → kişinin `meta_marketing_stopped_at` bayrağı
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

| Kuyruk | Öncelik (varsayılan) | Task'ler |
|---|---|---|
| `critical` | 6 | `whatsapp:webhook`, `whatsapp:send` (inbox/API/otomasyon kaynaklı), `automation:evaluate` |
| `campaigns` | 3 | `campaign:materialize`, `campaign:dispatch`, `whatsapp:send` (kampanya mesajı) |
| `default` | 3 | `automation:run`, `automation:resume`, `webhook:fanout`, `webhook:deliver`, `whatsapp:media_download`, zamanlanmış işler (§11) |
| `ai` | 2 | Henüz task yok; Phase 16C'deki AI işleri için ayrıldı |
| `bulk` | 1 | `contacts:import` |

- **Yönlendirme** `internal/platform/queue` içindeki tablodadır (`PolicyFor`). `whatsapp:send` payload'ı `campaign_id` taşıyorsa (`messaging.SendPayload`) `campaigns` kuyruğuna, taşımıyorsa `critical`'a gider (`RouteTask`). Böylece inbox yanıtları kampanya yükünün arkasında beklemez. İki kuyruk da aynı numara başı hız sınırlayıcısını paylaşır.
- **Ertelemeler kuyruğu değiştirmez:** hız sınırı (`RetryAfterError`) asynq'in aynı task'i aynı kuyrukta yeniden planlamasıyla yapılır ve deneme hakkından düşmez (`IsFailure`).
- Tek worker process'i tüm kuyrukları dinler. Öncelikler `WORKER_QUEUES` ile değiştirilir (varsayılan `critical=6,campaigns=3,default=3,ai=2,bulk=1`, eşzamanlılık `WORKER_CONCURRENCY=20`). Ölçeklemek için aynı imaj farklı `WORKER_QUEUES` değerleriyle çoğaltılır. `WORKER_QUEUES`'tan çıkarılan kuyruk hiç işlenmez.
- **Retry:** exponential backoff (2 sn taban, ×2, ±%20 jitter, en fazla 10 dk). `RetryAfterError` istenen gecikmeyi kullanır. Kalıcı hatalar `asynq.SkipRetry` ile hemen arşivlenir. Deneme bütçesi task tipine göre aşağıdaki tablodadır (`queue.PolicyFor`; tabloda olmayan tipler `default` kuyruğu ve 10 deneme alır). Değer, ilk denemeden sonraki *başarısız* tekrar sayısıdır.

| Task | Kuyruk | MaxRetry | Neden |
|---|---|---|---|
| `whatsapp:send` | `critical` / `campaigns` | 10 | Meta'ya yapılan denemeler mesajda sayılır ve 8'de `failed` olur (`maxSendAttempts`); geçici Meta hataları erteleme olarak tekrar edilir. Bu bütçe yalnızca altyapı hatalarını (DB/Redis) kapsar. |
| `whatsapp:webhook` | `critical` | 20 | Son denemede olay `failed` işaretlenir (handler bütçeyi `GetMaxRetry` ile okur). Eşleşmeyen durumlar erteleme olarak 15 dk beklenir. |
| `whatsapp:media_download` | `default` | 8 | |
| `automation:evaluate`, `automation:run`, `automation:resume` | `critical` / `default` | 10 | Ayrıca `automation_reaper` takılan koşuları yeniden başlatır. |
| `campaign:materialize`, `campaign:dispatch` | `campaigns` | 10 | Kaldığı yerden devam eder (cursor); `campaign_reaper` boşta kalan kampanyayı yeniden tetikler. |
| `webhook:fanout`, `webhook:deliver` | `default` | 10 | Teslim denemeleri ve backoff `webhook_deliveries` tablosundadır; bu bütçe yalnızca kayıt yazılamadığında harcanır. |
| `contacts:import` | `bulk` | 3 | |
| Zamanlanmış işler (`maintenance:*`, `campaign:start_due`, `campaign:reaper`, `offers:tick`, `automation:reaper`, `webhook:cleanup`, `channels:token_check`, `license:check_in`) | `default` | 2 | Bir sonraki dilimde zaten yeniden çalışır. |

- **Ölü mektup (dead letter):** Bütçesi biten veya `SkipRetry` dönen task asynq "archived" kümesine geçer (asynq kuyruk başına en fazla 10.000 kaydı 90 gün tutar). Panelde **Sistem → Başarısız işler** (`GET /system/failed-jobs`, `settings.read`) bu kayıtları en yeniden eskiye listeler: tip, kuyruk, son hata (kimlik bilgileri maskelenir), başarısızlık zamanı, deneme sayısı ve payload'dan yalnızca `_id` ile biten alanlar. Payload'ın kendisi asla döndürülmez. `POST /system/failed-jobs/{queue}/{id}/retry` task'i tekrar kuyruğa alır (bütçesi bittiği için yeniden başarısız olursa hemen arşive döner), `DELETE …/{queue}/{id}` siler. İkisi `settings.update` ister ve audit log'a yazılır (`failed_job.retry`, `failed_job.delete`). Toplam sayı `/system/status` yanıtındaki `failed_jobs` alanında ve genel bakış sayfasında uyarı olarak görünür. Mesajlar için asıl kayıt yine DB'deki `failed` durumudur.
- **Asynq notu:** v0.26 (1.0 öncesi). Sürüm `go.mod`'da sabit, asynq'e özgü kod `internal/platform/queue` arkasında. Böylece gerekirse değiştirilebilir. Asynq Redis'te `noeviction` gerektirir.

## 11. Scheduler

- Asynq Scheduler birden fazla instance'ta duplicate üretir. Bu yüzden scheduler process'i Redis lease lock (`SET NX PX` + yenileme) ile **leader** seçer, yalnızca leader tick atar.
- İkinci güvenlik katmanı: her tick task'inin ID'si zaman dilimini içerir (`sched:<iş>:<dilimin unix zamanı>`, ör. `sched:campaigns_due:1795640700`), bu da duplicate'i asynq'te engeller.
- Üçüncü katman: işler DB'de atomik claim ile yapılır.

| İş (`internal/app/scheduler.go`) | Task | Periyot |
|---|---|---|
| Vadesi gelmiş planlı kampanyaları başlat | `campaign:start_due` | 1 dk |
| Kampanya/gönderim reaper'ı: `sending`'de kalan mesajlar `unknown` olur, kuyruğu kaybolmuş `pending` mesajlar yeniden kuyruğa alınır, boşta kalan kampanya dispatcher'ı yeniden tetiklenir | `campaign:reaper` | 1 dk |
| Süreli teklifler: `scheduled` → `active` → `expired`; biten teklifin kampanyaları `offer_expired` ile duraklar, `offer.updated` yayınlanır | `offers:tick` | 1 dk |
| Otomasyon reaper'ı: süresi dolan beklemeler ve takılan koşular | `automation:reaper` | 1 dk |
| Yayınlanmış outbox kayıtlarının temizliği (7 gün) | `maintenance:outbox_cleanup` | 1 sa |
| Süresi dolmuş oturum ve idempotency anahtarı temizliği | `maintenance:identity_cleanup` | 1 sa |
| Giden webhook olay/teslim kayıtlarının temizliği | `webhook:cleanup` | 6 sa |
| WhatsApp erişim token'ı kontrolü | `channels:token_check` | 6 sa |
| Lisans check-in | `license:check_in` | 6 sa |
| Numara bilgisi senkronu (`throughput`, `quality_rating`, portföy mesajlaşma limiti) | — | *Planlandı: Phase 16C P0* |

- Meta webhook olayları alındıkları transaction'da outbox'a yazıldığı için ayrı bir webhook event reaper'ı yoktur. Outbox relay'i yayınlanamayan kaydı kendisi tekrar dener.
- Template'ler hesap bağlanırken ve panelden elle yenilenince senkronlanır, durum değişiklikleri `message_template_status_update` webhook'uyla gelir. Periyodik template senkron işi yoktur.
- `unknown` mesajlar için zaman aşımı işi yoktur: mesaj, Meta'dan durum webhook'u gelene kadar `unknown` kalır (§6.2).

## 12. Gerçek zamanlı (Centrifugo v6)

- Centrifugo config'ini `messora init` üretir (`provision.CentrifugoConfig`): token ve subscription token HMAC secret'ı, `allowed_origins` = `PUBLIC_APP_URL`, kapalı admin UI, namespace'ler. Client publish yoktur; token'sız abonelik yoktur (namespace'lerde `allow_subscribe_for_client` kapalı).
- **Token'lar:** `POST /api/v1/realtime/connection-token` ve `POST /api/v1/realtime/subscription-token` (yalnızca `session` şeması, API key'ler reddedilir). Go, gocent'in `token` paketiyle HS256 JWT üretir, ömür 10 dakika. SDK süresi dolmadan yenisini ister; devre dışı bırakılan kullanıcı veya geri alınan yetki en geç bu sürede erişimini kaybeder.
- **Kanallar ve izinler** (`internal/modules/realtime/channels.go`, tek kaynak):
  - `personal:#<user_id>` ve `system:notifications`: connection token'ın `subs` claim'i ile sunucu tarafı abonelik.
  - `inbox:all`: yalnızca `conversations.read_all` (tüm konuşmaların kimliklerini taşır).
  - `inbox:unassigned`: `conversations.read` veya `conversations.read_all`; atanmamış konuşmaların olayları.
  - `conversation:<id>`: `conversations.read_all` veya, `conversations.read` ile, kendisine atanmış ya da atanmamış konuşmalar (kontrol `conversations.Service.CanRead`, token verilirken).
  - `campaign:all` ve `campaign:<id>`: `campaigns.read`.
  - Bilinmeyen veya bozuk her kanal `403 CHANNEL_FORBIDDEN`. Centrifugo token'daki `sub` ve `channel`'ı ayrıca doğrular: başka kullanıcının ya da başka kanalın token'ı reddedilir.
- **Yönlendirme (görünürlüğe göre):** Konuşma ve mesaj olayları `realtime.ConversationChannels` ile dağıtılır: her zaman `inbox:all` ve `conversation:<id>`; ayrıca konuşma atanmamışsa `inbox:unassigned`, atanmışsa atanan kişinin `personal:#<id>` kanalı. Böylece yalnızca `conversations.read` yetkisi olan temsilci, göremediği konuşmaların kimliğini ya da durumunu hiçbir kanalda almaz. Atama değişince (`PublishConversationChange`) önceki kitle de (önceki atanan ya da atanmamış havuzu) yeni durumu bir kez alır, istemci konuşmayı listeden düşürür. Konuşması olmayan mesaj olayları yalnızca `inbox:all`'a gider.
- **Yayınlama:** Üretici, iş değişikliğiyle aynı transaction'da `realtime.Publish(ctx, tx, kanal, event)` (birden çok kanal için `PublishAll`) çağırır. Bu, `rt:<kanal>` topic'li bir outbox satırıdır. Worker'daki relay `rt:` prefix'ini Centrifugo publish API'sine yönlendirir, idempotency key `outbox:<id>`'dir (tekrar eden publish client'a iki kez ulaşmaz).
- **Event zarfı:** Her event `realtime.Envelope` gömer: `type`, `v` (sürüm, şu an 1), `id` (event başına benzersiz UUIDv7; aynı event birden çok kanala yayınlansa da tek `id` taşır, client tekrarı eler), `ts` (üretim anı, UTC). Alanlar eklemek sürüm artırmaz; anlam değişirse `v` artar. Event'ler yalnızca ID ve durum taşır, mesaj gövdesi taşımaz. Client ayrıntıyı API'den kendi yetkisiyle çeker.
- **Adlandırma:** `<alan>.<olay>`; alan iş nesnesidir (`message`, `conversation`, `campaign`, `automation.run`, `ai`, `notification`), olay geçmiş zaman fiili (`created`, `updated`) ya da durum akışı için isim (`status`, `progress`, `activity`). Client bilmediği türleri yok sayar. 16C için ayrılmış (tanımlı, henüz yayınlanmayan) türler: `conversation.handoff`, `conversation.activity`, `automation.run.updated`, `campaign.progress`, `ai.status`, `notification.created`.
  - `message.created`: gelen mesaj (webhook) veya kuyruğa alınan giden mesaj. `message_id`, `contact_id`, `phone_number_id`, `direction`, `message_type`, `status`, `at`.
  - `message.status`: gönderim sonucu (sent / failed / unknown) veya webhook'tan gelen gerçek bir durum değişikliği. Değişmeyen (eski sıradaki) status yayınlanmaz.
  - `conversation.updated`: konuşmaya mesaj geldi/gitti, atama, durum veya okunmamış sayısı değişti. Yukarıdaki yönlendirmeyle yayınlanır. Mesaj event'leri de aynı kanallara gider ve `conversation_id` taşır.
  - `campaign.updated`: kampanya durumu veya sayaçları değişti; `campaign:all` ve `campaign:<id>`'ye yayınlanır.
- Event'ler kalıcı veri değildir. `personal`, `campaign`, `conversation` ve `inbox` namespace'lerinde history 200 kayıt / 10 dk, `system`'de 20 kayıt / 5 dk; hepsinde `force_recovery` açıktır.
- **Yeniden senkron:** `GET /api/v1/realtime/sync?channel=<kanal>&since=<RFC3339>` (yalnızca `session`, kanal yetkisi abonelik token'ıyla aynı) kanalın `since`'ten beri duyuracağı şeylerin güncel halini, o kanalın event'leri biçiminde döner: inbox/personal/conversation kanalları için `since`'ten beri değişen ve çağıranın görebildiği konuşmalar (`conversation.updated`, en çok 200), kampanya kanalları için değişen ve aktif kampanyalar (`campaign.updated`, en çok 50). Fazlası varsa `has_more: true`.
- **Frontend:** `RealtimeProvider` (`frontend/src/components/realtime.tsx`) panel layout'unda tek bağlantı açar: `wss://<origin>/connection/websocket` (Caddy `/connection/*` → centrifugo). Token'lar Server Action'larla BFF üzerinden alınır, session token ve secret'lar tarayıcıya ulaşmaz. SDK kopmada backoff ile kendiliğinden yeniden bağlanır. Client her event'i `id` ile tekrar eler (sınırlı LRU, `lib/realtime-sync.ts`), kanal başına son offset/epoch'u ve kanalın en son güncel olduğu anı tutar (bağlıyken 30 sn'de bir işaretlenir; cihaz uyurken zamanlayıcı durduğundan boşluğu doğru sınırlar). Abonelik `wasRecovering && !recovered` ile gelirse (history dolmuş, Centrifugo yeniden başlamış) önce `/realtime/sync` çağrılır; dönen anlık görüntüler normal event olarak (`synced: true`) dinleyicilere verilir, ardından client tarafı `realtime.resynced` event'i gelir. Boşluk 1 saatten uzunsa, an bilinmiyorsa ya da `has_more` ise `router.refresh()` yapılır. Gelen kutusu sayfası (`inbox-realtime.tsx`) konuşma ve yeni mesaj olaylarında yenilenir; `message.status` olaylarını yalnızca açık konuşmaya aitse dikkate alır. `<RealtimeRefresh channel types>` sayfalara polling'siz yenileme sağlar. CSP `connect-src` yalnızca aynı origin'in `ws(s)://` adresine izin verir.
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
