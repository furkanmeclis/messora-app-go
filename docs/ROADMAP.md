# Yol Haritası

> Her phase sonunda testler çalıştırılır ve dokümanlar güncellenir. "Bitti" kriterini karşılamayan iş tamamlanmış sayılmaz.

"Bitti" kriterleri her phase için ölçülebilir tanımlanmıştır. Kriterleri karşılamayan iş tamamlanmış sayılmaz.

## Phase 0 — Analiz ve tasarım ✅
Özellik kapsamı, mimari, DB tasarımı, güvenlik modeli, yol haritası.

## Phase 1 — Temel altyapı ✅ (2026-11-22)
- Repo iskeleti, `go.mod` (Go 1.27.4), sabitlenmiş bağımlılıklar, Makefile, `.editorconfig`, `.gitignore`, `.dockerignore`, `LICENSE` (MIT), `NOTICE`, `SECURITY.md` (disclosure), `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `CHANGELOG.md`.
- `cmd/messora`: `api | worker | scheduler | migrate | init | version` alt komutları.
- Config yükleme ve doğrulama, secrets dosyası, slog + redaction, request ID, recover, Prometheus, `/healthz` ve `/readyz`.
- pgxpool, go-redis, goose (embed) + ilk migration (extension'lar, `settings`, `outbox_events`), sqlc yapılandırması, asynq server/client iskeleti, outbox relay, scheduler leader lock.
- Next.js 16 iskeleti: shadcn/ui bileşenleri, Messora teması, kurulum durumu sayfası.
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
Tetikleyici motoru (5 tetikleyici), aksiyon motoru (otomatik yanıt, template, tag, grup, alan kaydetme, koşul, HTTP/webhook [SSRF korumalı], e-posta, gecikme, atama, kapatma, susturma), worker'da yürütme, döngü ve duplicate koruması, execution log, giden webhook teslimat sistemi (event'ler, retry, log, yeniden gönder).

**Bitti kriteri:** Aynı webhook iki kez gelince otomasyon bir kez çalışır. İki otomasyon birbirini tetikleyemez. Gecikme aksiyonu worker restart'ından sağ çıkar.

## Phase 9 — Frontend tamamlama
Tüm ekranların tamamlanması: kişi grupları, ayar ekranları (users, roles, sessions, channels, api-keys, webhooks, audit), loading/empty/error state'leri, form validasyonları, erişilebilirlik kontrolü.

**Bitti kriteri:** FEATURE_MATRIX'teki her ekran özelliğinin bir sayfası vardır. Playwright smoke testi her route'u gezer.

## Phase 10 — Production readiness
Güvenlik denetimi (SECURITY.md checklist, bağımlılık taraması, CSP doğrulaması), integration, E2E ve performans paketlerinin tamamı, Compose finalizasyonu (digest pinleri, kaynak limitleri, log rotasyonu), README, deployment, backup/restore (pg_dump + secrets volume + medya, geri yükleme provası), upgrade prosedürü, `FEATURE_MATRIX` test eşlemesi.

**Bitti kriteri:** FEATURE_MATRIX'teki tüm v1 özellikleri `Implemented` durumundadır ve temiz ortamda `git clone → cp .env.example .env → docker compose up -d --build` akışı panelin açıldığı noktaya kadar sorunsuz çalışır. Backup'tan geri yükleme provası belgelenmiştir.

## Phase 11 — Ayar katmanı ve kurulum sihirbazı
- Kurulumla yönetilen ayarlar veritabanında tutulur, `.env` her zaman önceliklidir: `.env`'de tanımlı bir değer panelde "ortam değişkeniyle yönetiliyor" olarak kilitli görünür.
- SMTP ayarları (şifre AES-GCM ile şifreli) + test e-postası; marka (organizasyon adı, logo); varsayılan dil, saat dilimi, tarih ve saat biçimi; tema.
- 2FA zorunluluğu organizasyon ayarıdır; varsayılan olarak hiçbir rol için zorunlu değildir, sahip ve adminler açar.
- Web kurulum sihirbazı (sahip hesabı oluşturulduktan sonra): organizasyon ve logo → dil → saat dilimi ve tarih biçimi → tema → SMTP → Meta hesabı → 2FA politikası → ekip daveti → bitiş. Her adım atlanabilir ve sonradan Settings'ten değiştirilebilir; sihirbaz kaldığı adımdan devam eder.

**Bitti kriteri:** Temiz kurulumda sihirbaz baştan sona tarayıcıda tamamlanır (E2E). `.env`'de tanımlı SMTP değeri panelden değiştirilemez ve kilitli görünür. SMTP şifresi veritabanında düz metin bulunmaz.

## Phase 12 — Tasarım sistemi ve tema motoru
- Messora'ya özgü görsel dil; tarayıcının yerel (native) kontrolleri kullanılmaz: select, checkbox, radio, tarih/saat seçici, dosya seçici, renk seçici, onay pencereleri özel bileşenlerdir.
- Tema motoru: hazır temalar (açık/koyu) + tam özelleştirme (renkler, yüzeyler, köşe yuvarlaklığı, yazı tipi, yoğunluk), canlı önizleme, okunabilirlik (kontrast) uyarısı. Sadece sahip değiştirir.
- Yükleme, boş ve hata durumlarında tutarlı hareket ve geçişler.

**Bitti kriteri:** Kaynakta `<select>`, `type="date|time|color|file|checkbox|radio"`, `window.confirm/alert` kalmaz (lint kuralı). Tema değişikliği yeniden yüklemeden uygulanır ve sunucu tarafında ilk boyamada doğru gelir (titreme yok).

## Phase 13 — Türkçe ve İngilizce
- Bütün arayüz metinleri sözlükten gelir (TR + EN). Kullanıcı kendi dilini seçer; varsayılan organizasyon ayarıdır. Sunucu hata mesajları koddan çevrilir.

**Bitti kriteri:** Sözlükte eksik anahtar kalmaz (test). Playwright smoke testi her route'u iki dilde gezer.

## Phase 14 — İş akışı tuvali (n8n tarzı) ✅ (2026-11-23)
- Otomasyon editörü sürükle-bırak düğüm tuvaline dönüşür: tetikleyici, koşul (evet/hayır dalları), bekleme, mesaj/şablon, HTTP, etiket, grup, alan, atama, e-posta düğümleri oklarla bağlanır; yakınlaştırma, mini harita, otomatik yerleşim, geri al/yinele.
- Graf mevcut motorun programına derlenir (döngü yasak, tek tetikleyici). Mevcut otomasyonlar tuvale otomatik çevrilir. Çalıştırma geçmişi tuvalde hangi düğümden geçtiğini gösterir.

**Bitti kriteri:** Graf → program derleyicisi için tablo ve fuzz testleri; eski otomasyonların dönüşümü kayıpsızdır; E2E: tuvalde dallı akış kurulur, kaydedilir, tetiklenir.

**Durum:** `@xyflow/react` tuvali; adımların kalıcı `id`'si, `layout` (düğüm konumları) ve çalıştırma izi (`visited`) migration 00011 ile. Koşulun üç çıkışı var: evet, hayır ve iki daldan sonra çalışan "sonra"; graf her zaman ağaçtır, adım ağacına kayıpsız çevrilir. Testler: id/derleme tablo testleri, `FuzzStepIDs`, eski otomasyonların konumdan türetilen id'leri ve izleri, dallı akışın API üzerinden tetiklenip izinin doğrulandığı entegrasyon testi, `e2e/automation-canvas.spec.ts` (kur, kaydet, yeniden yükle). Ayrıntılar: [AUTOMATIONS.md](AUTOMATIONS.md).

## Phase 15 — Masaüstü modu
- F11 (tarayıcı tam ekranı) veya menüden: masaüstü, görev çubuğu, uygulama ikonları; bölümler taşınabilir, boyutlandırılabilir, küçültülüp büyütülebilir pencerelerde açılır; birden çok pencere aynı anda açık kalır. Pencere düzeni kullanıcı başına hatırlanır. Tam ekrandan çıkınca normal panele dönülür.

**Bitti kriteri:** Her bölüm bir pencerede çalışır (E2E); klavye ile pencere değiştirme; mobilde masaüstü modu kapalıdır.

## Phase 16 — Lisans ve dağıtım
- **Lisans:** Ed25519 ile imzalı belge; özel anahtar yalnızca sahibin bilgisayarında (`messora-license` CLI), uygulamada yalnızca açık anahtar. Alanlar: lisans ID, müşteri, paket ve limitler, `issued_at`, `expires_at` (boş = süresiz), `updates_until` (boş = süresiz), `check_in` (`online` varsayılan / `offline`).
- **Durumlar:** aktif → uyarı (son 30 gün) → ek süre (14 gün) → kısıtlı mod (salt okunur: gönderim, kampanya ve otomasyon durur; veriler görünür ve dışa aktarılabilir). Veri hiçbir durumda silinmez; yeni anahtar anında geri açar. Saat geri alınmasına karşı görülen en ileri zaman saklanır.
- **Erken kapatma:** iptal listesi (`revoke` / `reinstate`, sebebiyle). İptal edilen lisans imaj ve güncelleme alamaz; çevrimiçi lisanslı kurulum günlük yoklamada öğrenir ve 7 gün sonra kısıtlı moda geçer. 30 gün yoklama yapamayan çevrimiçi lisans kısıtlı moda geçer. Yeni anahtar verilince eskisi "yerine geçti" olarak geçersizleşir. Çevrimdışı lisanslar uzaktan kapatılamaz, bu yüzden en fazla 1 yıllık verilir.
- **Dağıtım:** kod private repoda; GitHub Actions imajları derler (`-trimpath -ldflags='-s -w'`, kaynak haritası yok), cosign ile imzalar ve Cloudflare R2'deki private registry'ye (Worker + R2, herkese açık adres yok) gönderir; push anahtarı yalnızca GitHub Secrets'ta. Worker kapısı: `docker login` şifresi lisans anahtarıdır; imza, iptal listesi ve güncelleme hakkı (sürüm çıkış tarihi ≤ `updates_until`) kontrol edilir; her çekme lisans ID'siyle kaydedilir (paylaşım tespiti). Ağsız müşteriler için R2'de `docker load` arşivi, Worker'ın ürettiği 15 dakikalık indirme bağlantısıyla. Müşteri yalnızca `get.<alan-adı>` adresini bilir; depolama arkada değiştirilebilir (gerekirse GHCR yedek seçenek). Doğrulanacak: Worker istek gövdesi sınırı (büyük katmanlar parçalı yükleme veya doğrudan R2'ye yazma).
- **Güncelleme kanalı:** Worker'da imzalı sürüm listesi (`stable` / `beta`); panel günde bir kontrol eder; `messora update` önce yedek alır, sonra yeni imajları çekip yeniden başlatır.
- **Kurulum:** `install` betiği lisansı sorar; sihirbazın ilk adımı lisanstır; Settings → License ekranından değiştirilir. Sihirbazda "hazır içerikle başla" adımı (şablon, örnek otomasyon, etiket setleri).
- **Defter:** CLI verilen lisansları şifreli yerel defterde tutar (`issue`, `revoke`, `reinstate`, `list`).

**Bitti kriteri:** Süresiz, süreli, süresi dolmuş, iptal edilmiş, yerine geçmiş ve sahte imzalı lisanslar için tablo testleri; kısıtlı modda gönderim API'leri reddeder; Worker iptal edilmiş lisansla imaj çekmeyi reddeder (test); `messora update` yedek alıp güncellemeyi tamamlar (E2E).

## Sonraki sürümler (v1.x)
- Passkey (go-webauthn), Embedded Signup
- Maliyet takibi (Meta pricing alanları)
- Centrifugo Redis engine (çoklu node), `messages` partition ve arşivleme, `campaign_stats` sharding
- S3 uyumlu medya depolama
- Özel rol editörü
- Google Sheets aksiyonu

## Riskler

| Risk | Etki | Azaltma |
|---|---|---|
| Meta `biz_opaque_callback_data`'yı beklenen şekilde döndürmezse | `unknown` mesajlar otomatik uzlaşmaz | P3'te doğrulanacak. B planı: `unknown` mesajlar operatör kararına kalır; kampanya bitişinde ayrı raporlanır. |
| Asynq'in 1.0 öncesi olması | API değişikliği | Sürüm sabit, kod `platform/queue` arkasında |
| Next.js BFF ek karmaşıklık getiriyor | Auth hataları | Yetki tamamen Go'da; Next ↔ Go sözleşme testleri |
| Meta politika ve limit değişiklikleri | Gönderim reddi | Sürüm yapılandırılabilir, her ilgili phase başında doküman kontrolü |
