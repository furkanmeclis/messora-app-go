# Güvenlik Tasarımı ve Uyumluluk

> Bu doküman tehdit modelini ve kontrolleri tanımlar. Açık bildirme (disclosure) politikası kök dizindeki `SECURITY.md` dosyasında olacak (Phase 1).

## 1. Varlıklar ve tehditler

| Varlık | Tehdit | Kontrol |
|---|---|---|
| Meta access token, app secret | DB sızıntısı, tarayıcıya sızma, log'a düşme | AES-256-GCM ile şifreli `_enc` kolonlar. API yanıtında asla dönmez, yalnızca `token_last4` döner. Log redaction uygulanır. Rotasyon: anahtar sürüm byte'ı. |
| Müşteri verisi (telefon, mesaj içeriği) | Yetkisiz erişim, log sızıntısı | RBAC kullanılır, Agent yalnızca atanmış konuşmaları görür (ayar). Mesaj gövdesi ve telefon numarası log'lara yazılmaz. Contact silme hard delete'tir. |
| Panel hesapları | Brute force, credential stuffing, oturum çalınması | Argon2id hash. E-posta+IP ve IP bazında Redis rate limit uygulanır. 10 başarısız denemede hesap 15 dk kilitlenir. TOTP 2FA, Owner/Admin için zorunlu yapılabilir (ayar). Opak token HttpOnly `__Host-` cookie'de tutulur, Go tarafında yalnızca BFF'den kabul edilir. |
| Mesaj gönderim kapasitesi | Kötüye kullanım, spam, numaranın banlanması | Opt-out kişilere gönderim sistem seviyesinde engellenir. 24h pencere zorunludur. Kalite düşüşünde (RED) kampanyalar otomatik duraklatılır. API rate limit uygulanır. |
| Webhook endpoint'i | Sahte event enjeksiyonu, replay | `X-Hub-Signature-256` ham body üzerinde sabit zamanlı karşılaştırmayla doğrulanır. Verify'da `hub.verify_token` kontrol edilir. Body hash ile dedupe yapılır. Body boyutu sınırlıdır. |
| Sunucu iç ağı | SSRF (otomasyon HTTP aksiyonu, giden webhook) | `internal/platform/safehttp` client'ı ismi kendisi çözer ve IP kontrolünü DNS çözümlemesinden **sonra**, ayrıca bağlantı anında (dialer `Control` hook'u) tekrar yapar; DNS rebinding işe yaramaz. Loopback, link-local, RFC1918, CGNAT, metadata (169.254.169.254), IPv6 ULA, multicast, ayrılmış/dokümantasyon blokları, IPv4-mapped ve NAT64 biçimleri ile `2130706433` / `0177.0.0.1` gibi sayısal kısaltmalar reddedilir (Docker ağları RFC1918 içindedir). Varsayılan yalnızca https; redirect izlenmez (izlenirse her adım yeniden kontrol edilir); `HTTP(S)_PROXY` yok sayılır. Timeout 10 sn, yanıt boyutu sınırlıdır, hata mesajlarında URL kimlik bilgisi yer almaz. Özel adreslere izin veren `AllowPrivate` seçeneği test binary'si dışında panic eder, yapılandırmayla açılamaz. İç hedefler için açık allowlist (`Options.AllowedPrefixes`) kodda hazırdır; `OUTBOUND_HTTP_ALLOWLIST` ayarına henüz bağlanmadı. |
| API key'ler | Sızma | `msk_live_` + 32 byte rastgele değer kullanılır, SHA-256 hash olarak saklanır. Değer tek seferlik gösterilir. Scope'lar ve opsiyonel bitiş tarihi desteklenir. Yalnızca `Authorization` header'ı ile kabul edilir, query parametresi reddedilir. `last_used_at` takip edilir. |
| Dosya yükleme | Zararlı dosya, zip bomb, CSV injection | Boyut sınırı (import 50 MB, medya Meta limitleri), uzantı ve magic-byte kontrolü yapılır. XLSX streaming okunur, sıkıştırılmış boyut ve oran sınırı uygulanır. CSV export'ta `= + - @` ile başlayan hücreler `'` ile kaçırılır (formula injection). Medya, Content-Type'ı zorlanmış ve `Content-Disposition: attachment` başlıklı yetkili bir endpoint'ten servis edilir. |
| Yetki yükseltme | IDOR | Her ID girdisi (grup, tag, template, numara) servis katmanında var olma ve izin açısından doğrulanır. Agent kapsamı (atanmış konuşmalar) sorgu seviyesinde uygulanır. |
| Tedarik zinciri | Kötü niyetli bağımlılık | Bağımlılıklar sabitlenir ve lockfile CI'da doğrulanır (`npm ci` zorunlu). `govulncheck` ve `npm audit` CI'da çalışır. Docker imajları digest ile sabitlenir. Dependabot/Renovate kullanılır. |

## 2. Kimlik doğrulama

- **Parola:** Argon2id (`golang.org/x/crypto/argon2`). OWASP önerisi olarak m=19 MiB, t=2, p=1 alt sınır alınır, açılışta kalibre edilebilir. Parametreler PHC string içinde saklanır, giriş sırasında parametreler eskiyse hash yeniden üretilir. En az 12 karakter, yaygın parola listesi kontrolü yapılır. HIBP k-anonymity kontrolü opsiyoneldir ve varsayılan kapalıdır (dış servise veri gönderdiği için).
- **Oturum:** 32 byte token kullanılır, yalnızca `sha256` saklanır. İdle timeout 12 sa, mutlak süre 30 gün (ayarlanabilir). Parola değişince diğer tüm oturumlar iptal edilir. Kullanıcı aktif oturumlarını görüp sonlandırabilir.
- **2FA:** TOTP (RFC 6238, ±1 adım), 10 adet tek kullanımlık recovery kodu (hash'li). Hassas işlemlerde (API key oluşturma, Meta token değiştirme, rol değiştirme, kullanıcı silme) 10 dakika içinde yapılmış yeniden kimlik doğrulama gerekir.
- **İlk kurulum:**
  - Kullanıcı tablosu boşken API, tek seferlik bir **setup token** üretir ve log'a yazar (`docker compose logs messora-api` içinde `/setup?token=…`). Token'ın hash'i saklanır ve 24 saat geçerlidir. Owner hesabı oluşunca kalıcı olarak geçersizleşir. Alternatif yol: `docker compose exec messora-api messora setup create-owner`.
  - **Açık kayıt yoktur.** Yeni kullanıcılar davetle eklenir.
  - Production'da demo hesabı ve varsayılan parola **hiçbir koşulda** oluşturulmaz. Demo seed'i yalnızca `APP_ENV=development` iken ve açık bir komutla çalışır.
- **Parola sıfırlama:** Tek kullanımlık, 30 dakika geçerli token kullanılır. Yanıt, e-postanın kayıtlı olup olmadığını belli etmez. SMTP yapılandırılmamışsa özellik kapalıdır ve Owner CLI ile sıfırlama yapabilir.

## 3. Yetkilendirme

Modül bazında izinler:

```text
contacts.read  contacts.create  contacts.update  contacts.delete  contacts.import  contacts.export
segments.read  segments.manage
campaigns.read  campaigns.create  campaigns.start  campaigns.pause  campaigns.delete
conversations.read  conversations.read_all  conversations.reply  conversations.assign
templates.read  templates.manage
automations.read  automations.manage
channels.read  channels.manage           (WhatsApp hesap/numara, Meta token)
webhooks.manage  api_keys.manage
settings.read  settings.update  users.manage  audit.read
ai.read  ai.manage                       (AI sağlayıcıları, fiyatlar, AI profilleri; bkz. docs/AI.md)
messages.send                             (API key scope'u)
```

| Rol | Özet |
|---|---|
| Owner | Tüm izinler. Owner sayısı 0'a düşürülemez. |
| Admin | Owner dışındaki tüm izinler (Owner atayamaz ve silemez). |
| Manager | Contacts, segments, campaigns, templates (okuma), automations, conversations (tümü), audit (okuma), AI (okuma). Kullanıcı, kanal, AI ve API key yönetimi yok. |
| Agent | `conversations.read` (yalnızca atanmış ve atanmamış konuşmalar, ayarla değişir), `conversations.reply`, `contacts.read/update`, `templates.read`. |

- İzin kontrolü handler seviyesinde middleware (`RequirePermission`) ile, kaynak kapsamı (Agent ataması) servis seviyesinde yapılır.
- İzin seti oturum başına Redis'te 60 sn cache'lenir. Rol değişikliğinde cache invalidation yapılır.

## 3a. Kiracı yalıtımı (Phase 17 WP1)

Her firma bir **kiracıdır** (`tenants`). Firmanın verisi hiçbir yoldan başka bir firmaya görünmemelidir; kontrol katmanları dıştan içe:

1. **Alan adı → kiracı.** API, isteğin alan adını `tenant_domains` tablosunda arar (`internal/platform/tenant`). BFF, Go'ya iç ağ adresinden bağlandığı için tarayıcının açtığı alan adını `X-Messora-Tenant-Host` başlığıyla iletir; bu başlığa **yalnızca** BFF sırrı (`X-Messora-BFF`) da doğruysa güvenilir, aksi hâlde `Host` kullanılır (Caddy `Host`'u değiştirmeden iletir). `X-Forwarded-Host` kullanılmaz. Bilinmeyen alan adı `404 TENANT_NOT_FOUND`, askıdaki kiracı `403 TENANT_SUSPENDED` alır; doğrulanmamış özel alan adları bilinmeyen sayılır. Sonuç 15 sn önbelleklenir (bilinmeyenler 5 sn), değişiklik yapılan replikada hemen geçersizleşir.
2. **Kimlik bilgisi kiracıya bağlı.** Oturumlar, API anahtarları ve kullanıcılar bir kiracıya aittir (`tenant_id`). Arama zaten alan adının kiracısıyla yapılır (RLS); API guard ayrıca `principal.TenantID` ile alan adının kiracısını karşılaştırır ve farklıysa bilinmeyen token gibi `401` döner. A'nın anahtarı B'nin adresinde çalışmaz.
3. **Havuz kancası (fail-closed).** Uygulama `messora_app` rolüyle bağlanır. Havuzdan her bağlantı alınırken `set_config('app.tenant_id', <ctx kiracısı>, false)` çalışır, bırakılırken `RESET app.tenant_id`; sıfırlama başarısızsa bağlantı atılır. Bağlamda kiracı da `tenant.System` işareti de yoksa bağlantı **hiç alınmaz** (`tenant.ErrNoTenant`). `pool.Begin` aynı kancadan geçer.
4. **Row-level security.** Kiracıya ait 57 tablonun hepsinde RLS açık ve `FORCE` edilmiştir; `tenant_isolation` politikası okuma ve yazmada `tenant_id = app_tenant_id()` ister. `app_tenant_id()` ayar yoksa `NULL` döner (hiçbir satır görünmez, hiçbir satır yazılamaz); bozuk değer hata verir, erişimi genişletmez. `tenant_id` sütununun varsayılanı bağlantının kiracısıdır, `WITH CHECK` başka bir değeri reddeder.
5. **Bileşik yabancı anahtarlar.** Çocuk tablolar ebeveyne `(tenant_id, id)` ile bağlanır (23 ilişki: kişi ↔ mesaj/konuşma/alıcı, kampanya ↔ alıcı, otomasyon ↔ çalıştırma, numara ↔ hesap, etiket/grup üyelikleri, oturum ↔ kullanıcı …). RLS'i atlayan bir rol bile başka kiracının satırına işaret eden satır yazamaz.

**Roller.**

| Rol | Kim kullanır | Yetki |
|---|---|---|
| `messora` (sahip) | yalnızca `messora migrate` | tabloların sahibi; şemayı değiştirir, rol parolalarını ayarlar |
| `messora_app` | API, worker, scheduler | `NOBYPASSRLS`, hiçbir tablonun sahibi değil; kiracı tablolarında DML (RLS ile sınırlı), `tenants`/`tenant_domains`/`platform_settings`'te yalnızca `SELECT` |
| `messora_system` | outbox aktarıcısı ve temizliği (worker) | `BYPASSRLS`; havuzu 4 bağlantılık, istekten gelen hiçbir yol kullanmaz |

Parolalar `messora init` tarafından üretilir (`POSTGRES_APP_PASSWORD`, `POSTGRES_SYSTEM_PASSWORD`) ve `messora migrate` her çalıştığında role yazılır. API'nin sahip parolasına ihtiyacı yoktur.

**Önbellekler.** Ayarlar, SMTP göndericisi, onay ayarları ve kampanya analitiği ayarları kiracı başına önbelleklenir (`tenant.Cache`); kiracısız iş hiç önbelleğe yazılmaz. Numaraların gönderim tavanı (kiracı, numara) çiftiyle önbelleklenir. Paneldeki görünüm önbelleği alan adı başınadır. Alan adı çözümü ve adillik ayarları platform genelidir.

**Arka plan (WP3).** `outbox.Write` olayı yazanın kiracısıyla kaydeder; aktarıcı kiracıyı asynq görevinin başlığına (`messora-tenant`) koyar ve worker her işleyiciyi o kiracı olarak çalıştırır. Zamanlanmış işler kiracı başına dağıtılır: zamanlayıcı dilimi tek bir platform görevi olarak kuyruğa alır, worker aktif kiracıları sistem havuzundan okuyup işi her biri için başlıkla kuyruğa alır (askıdaki ve silinen kiracılar hariç). **Başlıksız görev reddedilir** (çalıştırılmadan ölü mektuba): yalnızca iki platform görevi (`platform:tenant_fanout`, `maintenance:outbox_cleanup`) başlıksız, `tenant.System` olarak çalışır. Tek kiracı bir worker'ın eşzamanlılığının en fazla yarısını tutar, kampanyaları birlikte en fazla 200 mesaj/sn gönderir (platform ayarı, ARCHITECTURE.md §10.1). Başarısız işler sayfası yalnızca isteğin kiracısının görevlerini gösterir; tekrar deneme ve silme de öyle.

**Realtime (WP3).** Her Centrifugo kanalı kiracı önekini taşır (`tenant:<tenant_id>:inbox:all`). Yayıncı öneki outbox satırının kiracısından alır; bağlantı ve abonelik token'ları yalnızca oturumun kiracısının kanallarını verir (istek alan adının kiracısı da aynı olmalı); kullanıcıya sınırlı kanal yoktur, kanal adı tek başına erişim açmaz. Centrifugo kullanıcısı `<tenant>:<user>` olduğundan bir kiracıda alınan abonelik token'ı başka kiracının bağlantısında geçmez. `/realtime/sync` aynı kuralla çalışır.

**Redis (WP3).** Kiracı işinin anahtarları `messora:t:{<tenant_id>}:…` önekindedir (API hız sınırı oturum/anahtar başına, gönderim zamanlayıcısı, AIMD). Kişi geneli anahtarlar bilerek globaldir: e-posta başına giriş kilidi, 2FA challenge'ı, TOTP tekrar koruması; anonim istek sınırı IP başınadır (ARCHITECTURE.md §10.2). Advisory lock anahtarları kiracıyı içerir.

**Medya (WP3).** Yerel sürücünün imzalı bağlantıları (`/media/s/`) nesnenin kiracısını imzaya katar ve yalnızca o kiracının alan adlarında açılır: `/media/s/` kiracı ara katmanının arkasındadır, isteğin kiracısı anahtarınkiyle uyuşmazsa `403`. Bağlantı onu üreten isteğin alan adına işaret eder. Kiracı bağlamı başka kiracının nesnesini imzalayamaz. `/media/p/` (logolar) alan adından bağımsızdır.

**CI kontrolü.** `TestEveryTableIsTenantScoped` (`internal/platform/database`) `pg_catalog`'u okur: platform listesi dışındaki her tabloda `NOT NULL tenant_id`, açık ve zorlanmış RLS ve iki yönlü `tenant_isolation` politikası yoksa test kırılır. `TestTenantIsolationMatrix` (`internal/app`) iki kiracıyla bütün liste/getir uçlarını, değiştirme denemelerini ve çapraz kimlik bilgilerini dener.

**Bilinen açıklar (sonraki paketler).** Platform tabloları (`meta_rates`, `ai_model_prices`) firma panelinden yazılabiliyor; platform paneline taşınacak (WP2). `APP_*` çalışma alanı ortam değişkenleri bütün kiracılara uygulanır (WP2'de platform ayarlarına taşınacak). Kullanıcıların e-posta adresi şimdilik bütün kiracılarda tekildir (WP2: global kullanıcı + üyelik).

## 4. Audit log

Yazılan event'ler:
- login başarılı/başarısız, logout, 2FA aç/kapat, parola değişikliği ve sıfırlama
- kullanıcı davet, rol değişikliği, devre dışı bırakma
- API key oluşturma ve iptal
- Meta hesap ve token değişikliği
- webhook endpoint değişiklikleri, ayar değişiklikleri
- kampanya oluşturma, başlatma, duraklatma, devam, iptal
- import ve export (kim, kaç kayıt), contact silme
- opt-in/opt-out değişikliği
- "belirsiz mesajı yine de tekrar gönder" kararı

Her kayıtta aktör, IP, user agent, request ID ve değişen alanların eski/yeni değerleri tutulur. Secret alanlar her zaman `"[redacted]"` olarak yazılır. Audit kayıtları uygulama üzerinden değiştirilemez ve silinemez. Saklama süresi ayarlanabilir.

## 5. HTTP güvenliği

- **Next.js yanıt başlıkları:**
  - `Content-Security-Policy`: nonce tabanlı. `connect-src` öz origin ve Centrifugo URL'i, `img-src` öz origin ve `blob:`/`data:` (Meta medyası proxy üzerinden servis edilir), `frame-ancestors 'none'` (panel hiçbir sayfada, kendi origin'inde bile çerçevelenemez; `X-Frame-Options: DENY`).
  - `Strict-Transport-Security` (HTTPS'te), `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy`.
- **XSS:** React varsayılan olarak escape eder. Mesaj gövdeleri asla `dangerouslySetInnerHTML` ile render edilmez. WhatsApp formatlama (`*kalın*` vb.) güvenli bir parser ile yapılır.
- **SQL injection:** Tüm sorgular sqlc ile üretilir veya parametrelidir. Dinamik kısımlar (sıralama kolonu, segment alanları) whitelist'ten gelir.
- **CSRF:** Ayrıntılar `ARCHITECTURE.md` §3.1'de. Server Action Origin kontrolü, Route Handler'larda `Origin` + `Sec-Fetch-Site` doğrulaması ve `SameSite=Lax` cookie birlikte kullanılır.
- **Boyut sınırları:** JSON body varsayılan 1 MiB, webhook 2 MiB, upload endpoint başına ayrı. `ReadHeaderTimeout` ve `ReadTimeout` set edilir (slowloris).
- **Go API dışa açıklığı:** Caddy yalnızca `/api/v1/*` ve `/webhooks/*` yollarını yönlendirir. `/metrics` ve health portları dışarı açılmaz.

## 6. WhatsApp politika uyumluluğu

| Gereksinim | Uygulama |
|---|---|
| Opt-in kaydı | `contact_consent_events`: kaynak, kanıt, zaman, aktör. Import sırasında "bu listenin izni var" beyanı alınır ve kayda geçer. |
| Opt-out / unsubscribe | Yapılandırılabilir anahtar kelimeler (varsayılan `STOP`, `DUR`, `IPTAL`, `UNSUBSCRIBE`, büyük/küçük harf ve aksan duyarsız) gelirse contact `opted_out` olur. Bu durum opsiyonel onay mesajıyla bildirilebilir. Opt-out butonlu marketing template yanıtları da işlenir. Manuel değişiklik audit'lenir. |
| Opt-out'a gönderim engeli | Dört katmanda uygulanır: (1) segment/kitle çözümünde varsayılan dışlama, (2) materialize'da `skipped_opted_out`, (3) `message:send` claim'i öncesi son kontrol, (4) API'de marketing template için `422 CONTACT_OPTED_OUT`. Kullanıcı bunu kapatamaz. Yalnızca UTILITY/AUTHENTICATION template'leri için ayarla izin verilebilir; bu ayar varsayılan olarak kapalıdır ve kullanılırsa audit'lenir. |
| Template zorunluluğu | Pencere dışında yalnızca `APPROVED` template gönderilebilir. Template durumu gönderim anında kontrol edilir (paused/disabled ise kampanya duraklar). |
| 24 saat penceresi | `conversations.customer_window_expires_at` tek kaynaktır. Serbest metin ve medya mesajları pencere dışında Meta'ya hiç gönderilmez. |
| Kalite ve limitler | Kalite RED'e düşünce yeni kampanya başlatma engellenir ve çalışan kampanyalar `paused_limit` olur. Bildirim gönderilir. Messaging limit tier'ı kampanya başlatma ekranında gösterilir. |
| Test ortamı | `APP_ENV=test` veya `MESSAGING_DRY_RUN=true` iken HTTP client gerçek Graph host'una bağlantıyı **kod seviyesinde reddeder**, yalnızca mock base URL'ine izin verilir. CI'da Meta kimlik bilgisi bulunmaz. |
| Politika güncelliği | Meta'nın güncel mesajlaşma, pricing ve messaging-limit kuralları Phase 3 ve Phase 5 başında resmî dokümandan yeniden doğrulanacak. Graph API sürümü yapılandırılabilir. |

## 7. Secret yönetimi

- Repository'de gerçek secret bulunmaz. `.env.example` yalnızca boş değerler ve açıklamalar içerir. `.gitignore` `.env*` dosyalarını (example hariç) dışlar. CI'da gitleaks çalışır.
- Eksik secret'ları ilk çalıştırmada `messora-init` üretir (`crypto/rand`) ve `messora-secrets` volume'una `0600` izinle yazar (`ARCHITECTURE.md` §14).
- Secret'lar process başlangıcında bir kez okunur. Log, panic çıktısı ve hata mesajlarına girmemeleri redaction katmanı ve tipli `Secret` sarmalayıcısıyla (`String()` → `[redacted]`) güvenceye alınır.
- **Anahtar rotasyonu:** `APP_ENCRYPTION_KEYS` birden fazla sürümlü anahtar alabilir. Yeni yazımlar en yeni anahtarla yapılır. `messora secrets reencrypt` komutu eski kayıtları yeni anahtarla yeniden şifreler.

## 8. Container güvenliği

- Go imajı `gcr.io/distroless/static` (nonroot, UID 65532), read-only root filesystem.
- Next imajı slim Node, non-root kullanıcı.
- Tüm servislerde `no-new-privileges`, gereksiz capability'ler düşürülür.
- Postgres, Redis ve Centrifugo admin portu host'a publish edilmez. Yalnızca Caddy 80/443 portlarını açar.
- Redis parola korumalıdır. Centrifugo admin UI varsayılan olarak kapalıdır.
