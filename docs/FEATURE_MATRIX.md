# Özellik Matrisi

> Messora'nın hedef kapsamı ve her özelliğin güncel durumu. Her phase sonunda güncellenir.

**Durumlar:**
- `Implemented`: Çalışıyor ve otomatik testle doğrulandı.
- `Partial`: Bir kısmı tamamlandı.
- `Missing`: Henüz yok.
- `Broken`: Mevcut ama çalışmıyor.
- `Needs verification`: Dış bir sisteme (ör. Meta) karşı doğrulama bekliyor.

## Platform (Phase 1)

| Özellik | Phase | Durum | Doğrulama |
|---|---|---|---|
| Tek binary: api / worker / scheduler / migrate / init | 1 | Implemented | Compose smoke testi |
| Config + otomatik secret üretimi (servis başına dosya) | 1 | Implemented | `internal/config`, `internal/provision` testleri |
| Yapılandırılmış log + redaction + request ID | 1 | Implemented | `logger`, `httpx` testleri |
| PostgreSQL bağlantısı, migration (up/down/eşzamanlı) | 1 | Implemented | `database` integration testi |
| Redis (AOF, noeviction, parola) | 1 | Implemented | Compose doğrulaması |
| Asynq kuyrukları, retry/backoff, task metrikleri | 1 | Implemented | `queue` testleri |
| Transactional outbox + LISTEN/NOTIFY relay | 1 | Implemented | `outbox` integration testi |
| Leader seçimli scheduler, duplicate'siz periyodik işler | 1 | Implemented | `scheduler` integration testi |
| Health / readiness / Prometheus metrikleri | 1 | Implemented | Compose healthcheck'leri |
| Docker Compose tek komut kurulum | 1 | Implemented | CI smoke testi |

## Kimlik doğrulama ve yönetim (Phase 2)

| Özellik | Phase | Durum |
|---|---|---|
| İlk kurulum (setup token + CLI) | 2 | Implemented |
| Login / logout, oturum yönetimi | 2 | Implemented |
| Argon2id parola, brute-force kilidi | 2 | Implemented |
| Şifre sıfırlama (SMTP) | 2 | Implemented |
| TOTP 2FA + recovery kodları, yeniden doğrulama | 2 | Implemented |
| Kullanıcı daveti, devre dışı bırakma | 2 | Implemented |
| Roller (Owner / Admin / Manager / Agent) + izinler | 2 | Implemented |
| Ayarlar | 2 | Implemented |
| Audit log | 2 | Implemented |
| API key'ler (scope'lu) | 2 | Implemented |
| API standardı: zarf, hata, pagination, rate limit, Idempotency-Key | 2 | Implemented |
| OpenAPI 3.1 + üretilmiş tipler (Go ve panel) | 2 | Implemented |
| CSP (nonce) ve güvenlik başlıkları | 2 | Implemented |
| Panel: kurulum, giriş, 2FA, davet, şifre sıfırlama, profil, güvenlik, kullanıcılar, API key'ler, audit, ayarlar | 2 | Implemented (Playwright E2E) |
| Passkey (WebAuthn) | v1.x | Missing |

## WhatsApp Cloud API (Phase 3)

| Özellik | Phase | Durum |
|---|---|---|
| Hesap / numara bağlama (şifreli token) | 3 | Implemented (API; panel ekranı Phase 9) |
| Graph API sürümü yapılandırılabilir (varsayılan v26.0) | 3 | Implemented |
| Text, template, image, video, document, audio, location, interactive | 3 | Implemented (+ sticker, reaction) |
| Medya upload / download | 3 | Partial (gelen medya indirilir ve inbox'tan yetkili indirilir; giden medya ID/link ile; panelden dosya yükleme henüz yok) |
| Webhook verify + imza doğrulama + önce kaydet | 3 | Implemented |
| Durum takibi (monoton) + durum geçmişi | 3 | Implemented (zaman damgaları; ayrıntılı event log Phase 5) |
| Template oluşturma / silme / senkronizasyon | 3 | Implemented (+ durum webhook'u) |
| Meta mock sunucusu + dry-run koruması | 3 | Implemented |
| `biz_opaque_callback_data` ile uzlaştırma | 3–5 | Implemented; gerçek Meta ile doğrulanmalı (gönderim tarafı dokümante değil) |
| Embedded Signup | v1.x | Missing |

## Contacts ve segmentasyon (Phase 4)

| Özellik | Phase | Durum |
|---|---|---|
| Contact CRUD, E.164 normalizasyonu, unique telefon | 4 | Implemented (API; panel ekranları Phase 9) |
| Etiketler, gruplar, notlar, özel alanlar | 4 | Implemented (var1–var5 + serbest attributes) |
| Opt-in / opt-out kayıtları | 4 | Implemented (append-only geçmiş; opt-out kişilere template gönderilmez) |
| CSV / XLSX import (worker, kolon eşleme, hata raporu, progress, iptal) | 4 | Implemented (CSV + XLSX; 100k satır ~20 sn, <10 MiB heap) |
| CSV / XLSX export (streaming) | 4 | Implemented (CSV streaming, XLSX; formül nötralizasyonu) |
| Dinamik segmentler (tipli kural ağacı, güvenli derleyici) | 4 | Implemented (tablo + enjeksiyon + fuzz testleri) |

## Kampanyalar (Phase 5)

| Özellik | Phase | Durum |
|---|---|---|
| Draft, planlanmış (timezone), başlat, duraklat, devam, iptal | 5 | Implemented (API; panel ekranları Phase 9) |
| Kitle: segment / grup / yüklenen liste, dedupe | 5 | Implemented (segment, grup, etiket, tüm kişiler; yüklenen liste = import + grup) |
| Pencereli dispatcher, numara başına rate limit, AIMD | 5 | Implemented |
| Retry, dead letter, belirsiz (`unknown`) durum | 5 | Implemented (lease reaper; kill -9 kaos betiği bekliyor) |
| Opt-out ve 24h kuralları sistem seviyesinde | 5 | Implemented |
| İstatistikler ve event log | 5 | Implemented (canlı sayılar + realtime yayın) |

## Gerçek zamanlı, inbox, otomasyon, giden webhook'lar

| Özellik | Phase | Durum |
|---|---|---|
| Centrifugo config'i (`messora init`), connection / subscription token endpoint'leri | 6 | Implemented |
| Kanal izinleri (yetkisiz kanal reddi, gerçek Centrifugo ile test edildi) | 6 | Implemented |
| Outbox → Centrifugo publish (idempotent), yeni mesaj ve durum event'leri | 6 | Implemented |
| Frontend Centrifuge client: yeniden bağlanma, recovery, kurtarılamayan kopmada yeniden çekme | 6 | Needs verification (Centrifugo restart E2E senaryosu yazıldı: `frontend/e2e/realtime.spec.ts`; CI'da çalışacak) |
| Kampanya ilerlemesinin realtime yayını | 6 | Partial (kanal, izin ve `campaign.updated` event'i hazır; kampanya motoru Phase 5 ile bağlanır) |
| Konuşma kanalı (`conversation:<id>`) agent ataması | 7 | Implemented (agent kendisine atanmış veya atanmamış konuşmalara abone olabilir; tablo + integration testleri) |
| Inbox: liste, geçmiş, gönderim, medya, okundu | 7 | Implemented (API + panel; görünürlük, 24h ve opt-out kuralları integration testlerinde; 10k konuşma listesi p95 < 200 ms testi; Playwright `frontend/e2e/inbox.spec.ts`, Meta mock'u olmadığı için gelen mesaj ve teslim bilgisi DB'ye yazılarak simüle edilir) |
| Konuşma atama, kapatma, müşteri paneli | 7 | Implemented (atama ve durum geçmişi, audit; müşteri paneli: consent, etiketler, notlar. Atanacak kullanıcı listesi yalnızca `users.manage` ile gelir, diğerleri kendine/atanmamışa atar) |
| Otomasyon: tetikleyiciler, aksiyonlar, görsel editör | 8 | Implemented (5 tetikleyici, 12 adım; editör paneli Phase 9) |
| Döngü / duplicate koruması, çalıştırma geçmişi | 8 | Missing |
| Giden webhook'lar (imzalı, retry, teslimat logu, SSRF korumalı) | 8 | Implemented (API + panel sayfası; event'ler değişiklikle aynı transaction'da outbox üzerinden; HMAC-SHA256 imza, ~24 saatte 10 deneme, 410'da devre dışı, yeniden gönder ve test event'i; `internal/platform/safehttp` hem kayıtta hem bağlantı anında IP denetler. Unit + integration testleri; entegrasyon rehberi `docs/WEBHOOKS.md`) |

## Operasyon (Phase 10)

| Özellik | Phase | Durum |
|---|---|---|
| E2E (Playwright) | 2+ | Partial (kimlik akışları kapsandı; her phase kendi akışlarını ekler) |
| Yük testleri | 10 | Missing |
| Backup / restore dokümanı ve provası | 10 | Missing |
| OpenTelemetry tracing | 10 | Missing |
| i18n (EN + TR) | v1.x | Missing |
