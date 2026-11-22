# Yol Haritası

> Phase 0 · 2026-11-22 · Her phase sonunda: testler çalıştırılır, dokümanlar güncellenir, `PHASE/COMPLETED/…` raporu verilir ve **sonraki phase için kullanıcı onayı beklenir.**

"Bitti" kriterleri her phase için ölçülebilir tanımlanmıştır. Kriterleri karşılamayan iş tamamlanmış sayılmaz.

## Phase 0 — Kaynak kod analizi ✅ (bu teslim)
ZeroMsg analizi, özellik matrisi, mimari, DB tasarımı, güvenlik, geçiş planı, yol haritası.

## Phase 1 — Temel altyapı ✅ (2026-11-22)
- Repo iskeleti, `go.mod` (Go 1.27.4), sabitlenmiş bağımlılıklar, Makefile, `.editorconfig`, `.gitignore`, `.dockerignore`, `LICENSE` (MIT), `NOTICE`, `SECURITY.md` (disclosure), `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `CHANGELOG.md`.
- `cmd/messora`: `api | worker | scheduler | migrate | init | version` alt komutları.
- Config yükleme ve doğrulama, secrets dosyası, slog + redaction, request ID, recover, Prometheus, `/healthz` ve `/readyz`.
- pgxpool, go-redis, goose (embed) + ilk migration (extension'lar, `settings`, `outbox_events`), sqlc yapılandırması, asynq server/client iskeleti, outbox relay, scheduler leader lock.
- Next.js 16 iskeleti: ZeroMsg UI bileşenleri, layout ve tema taşınır, boş bir auth ekranı olur.
- Docker: multi-stage distroless Go imajı, Next standalone imajı, Compose (caddy, web, api, worker, scheduler, init, migrate, postgres, redis, centrifugo) + healthcheck'ler.
- CI: lint (golangci-lint, eslint), test, `govulncheck`, `npm ci` lockfile kontrolü, gitleaks.

**Bitti kriteri:** Temiz bir makinede `cp .env.example .env && docker compose up -d --build` çalışır, bütün servisler `healthy` olur, `/readyz` 200 döner, migration sürümü doğru görünür, worker kuyruğa bağlıdır, Centrifugo health OK'dir. Integration testleri gerçek Postgres ve Redis'e karşı geçer.

## Phase 2 — Uygulama çekirdeği
Auth (ilk kurulum token'ı + CLI, login, logout, oturum yönetimi, şifre sıfırlama, TOTP 2FA, re-auth), kullanıcılar ve davet, RBAC (4 sistem rolü, izin seed'i), ayarlar, audit log, API key'ler (scope'lu), API standardı (zarf, hata, pagination, rate limit, Idempotency-Key), OpenAPI 3.1 + oapi-codegen + üretilmiş TS tipleri, Next BFF oturum akışı.

**Bitti kriteri:** E2E senaryosu "ilk admin oluşturma → login → kullanıcı davet → rol ile yetki reddi" geçer. Brute-force kilidi test edilmiştir. Audit kayıtları oluşur. İzin matrisi için tablo testleri vardır.

## Phase 3 — WhatsApp entegrasyonu
Meta Graph client (v26.0 varsayılan, yapılandırılabilir), hata sınıflandırması, tüm mesaj tipleri, medya upload/download, hesap ve numara yönetimi (şifreli token), webhook verify ve alım (önce kaydet), webhook işleme, template CRUD ve sync, Meta mock sunucusu, dry-run koruması.
**Başlangıçta doğrulanacaklar:** `biz_opaque_callback_data`'nın durum webhook'unda geri dönmesi, idempotency desteğinin olup olmaması, güncel hata kodu listesi, güncel throughput ve messaging-limit kuralları.

**Bitti kriteri:** Mock'a karşı her mesaj tipi gönderilir. İmzalı webhook replay'i duplicate üretmez. Sıra dışı durum event'leri monoton işlenir. Template sync E2E senaryosu geçer. Gerçek bir test numarasıyla manuel smoke checklist'i hazırdır (çalıştırma kullanıcı onayıyla yapılır).

## Phase 4 — Contact yönetimi
Contacts CRUD (E.164, unique), tag, grup, not, custom field (var1..5 uyumu), consent olayları, CSV/XLSX import (worker, kolon eşleme, doğrulama, duplicate, hata raporu, progress, iptal, streaming) ve export, segment AST + güvenli derleyici + önizleme sayısı.

**Bitti kriteri:** 100k satırlık CSV import'u bellek sınırı içinde tamamlanır ve iptal edilebilir. Segment derleyicisi için fuzz ve tablo testleri yazılmıştır (enjeksiyon girişimleri dahil). "Contact ekleme", "CSV import" ve "segment oluşturma" E2E senaryoları geçer.

## Phase 5 — Kampanya motoru
Kampanya CRUD ve planlama, materialize (resumable), pencereli dispatcher, `message:send` durum makinesi, numara başına GCRA rate limiter + AIMD, retry/backoff, unknown durumu ve uzlaştırma, pause/resume/cancel, held mesajlar, istatistikler, event log, opt-out ve 24h kuralları, lease reaper.

**Bitti kriteri:**
- **Yük testi:** Mock Meta'ya karşı 100k alıcı; 0 duplicate, 0 kayıp.
- **Kaos testi:** Gönderim sırasında worker `kill -9` + restart → duplicate yok, kalan mesajlar ya devam eder ya da `unknown` olarak işaretlenip uzlaşır.
- Pause/resume 10 kez tekrarlanır, sonunda toplamlar tutar.
- Mock'tan 130429/131056 enjekte edilince backoff görülür.
- "Kampanya oluşturma, başlatma, webhook ile durum güncelleme, pause/resume" E2E senaryoları geçer.

## Phase 6 — Real-time altyapı
Centrifugo config (init tarafından üretilir), connection ve subscription token endpoint'leri, kanal izinleri, outbox → publish, frontend Centrifuge client (yeniden bağlanma + recovery + kopma sonrası yeniden çekme).

**Bitti kriteri:** Yetkisiz kanala abonelik reddedilir (test edilmiş). Kampanya ilerlemesi ve yeni mesaj, polling olmadan UI'a yansır. Centrifugo restart sonrası client kendiliğinden toparlanır.

## Phase 7 — Inbox
Konuşma listesi (filtre: açık, kapalı, bana atanmış, atanmamış), geçmiş (cursor), tüm tiplerde gönderim, medya görüntüleme, okundu işaretleme, atama ve geçmişi, kapat/aç, müşteri paneli (tag, not, consent), 24h göstergesi, Agent kapsamı.

**Bitti kriteri:** Agent başka birine atanmış konuşmayı API'den bile okuyamaz. 10k konuşmalık liste < 200 ms (p95) döner. Realtime güncellemeler çalışır.

## Phase 8 — Otomasyon
Tetikleyici motoru (5 tetikleyici), aksiyon motoru (otomatik yanıt, template, tag, grup, alan kaydetme, koşul, HTTP/webhook [SSRF korumalı], e-posta, gecikme, atama, kapatma, susturma), worker'da yürütme, döngü ve duplicate koruması, execution log, ZeroMsg v1 import, giden webhook teslimat sistemi (event'ler, retry, log, yeniden gönder).

**Bitti kriteri:** Aynı webhook iki kez gelince otomasyon bir kez çalışır. İki otomasyon birbirini tetikleyemez. Gecikme aksiyonu worker restart'ından sağ çıkar. ZeroMsg'den export edilmiş örnek akışlar import edilip aynı sonucu verir.

## Phase 9 — Frontend migration (tamamlama)
Tüm ekranların yeni API'ye bağlanması, Inertia shim'lerinin kaldırılması, contact-groups sayfası (F8), ayar ekranları (users, roles, sessions, channels, api-keys, webhooks, audit), loading/empty/error state'leri, form validasyonları, erişilebilirlik kontrolü.

**Bitti kriteri:** `@inertiajs/*` bağımlılığı tamamen kalkmıştır. ZeroMsg'deki her ekranın Messora karşılığı vardır (ekran ekran checklist). Playwright smoke testi her route'u gezer.

## Phase 10 — Production readiness
Güvenlik denetimi (SECURITY.md checklist, bağımlılık taraması, CSP doğrulaması), integration, E2E ve performans paketlerinin tamamı, Compose finalizasyonu (digest pinleri, kaynak limitleri, log rotasyonu), README, deployment, backup/restore (pg_dump + secrets volume + medya, geri yükleme provası), upgrade prosedürü, `FEATURE_MATRIX` test eşlemesi.

**Bitti kriteri:** Talimat §23'teki teslim listesi ve temiz ortamda `git clone → cp .env.example .env → docker compose up -d --build` akışı panelin açıldığı noktaya kadar sorunsuz çalışır. Backup'tan geri yükleme provası belgelenmiştir.

## Sonraki sürümler (v1.x)
- Passkey (go-webauthn), Embedded Signup
- ZeroMsg veri taşıma aracı (`MIGRATION_PLAN.md` §5)
- Maliyet takibi (Meta pricing alanları)
- i18n (EN + TR)
- Centrifugo Redis engine (çoklu node), `messages` partition ve arşivleme, `campaign_stats` sharding
- S3 uyumlu medya depolama
- Özel rol editörü
- Google Sheets aksiyonu (ZeroMsg'de placeholder'dı)

## Riskler

| Risk | Etki | Azaltma |
|---|---|---|
| Meta `biz_opaque_callback_data`'yı beklenen şekilde döndürmezse | `unknown` mesajlar otomatik uzlaşmaz | P3'te doğrulanacak. B planı: `unknown` mesajlar operatör kararına kalır; kampanya bitişinde ayrı raporlanır. |
| Asynq'in 1.0 öncesi olması | API değişikliği | Sürüm sabit, kod `platform/queue` arkasında |
| Next.js BFF ek karmaşıklık getiriyor | Auth hataları | Yetki tamamen Go'da; Next ↔ Go sözleşme testleri |
| ZeroMsg UI'ının Inertia bağımlılığı tahminden derin çıkarsa | Phase 9 uzar | Shim yaklaşımı, sayfa sayfa ilerleme |
| Meta politika ve limit değişiklikleri | Gönderim reddi | Sürüm yapılandırılabilir, her ilgili phase başında doküman kontrolü |
| ~~Yerel makinede Docker kapalı / Go 1.26.4~~ | — | Çözüldü: Docker çalışıyor. Go 1.27.4 `GOTOOLCHAIN=auto` ile yalnızca bu projede kullanılıyor, sistem Go'su değişmedi. |
