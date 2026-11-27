# Phase 17 — Çok kiracılı SaaS (Messora Cloud)

Durum: **kapsam taslağı** (2026-11-27). Uygulamaya başlarken, 16C'de yaptığımız gibi önce teknik analiz yapılır: mevcut kod taranır, riskler ve iş paketleri bu belgeye eklenir, ürün sahibinin onayıyla başlanır. Tasarım kanvası: https://claude.ai/artifact/5gT44f3kyVA6ePXMGMidCX.

## 1. Kararlar

### 1.1 Ürün yönü
- **Messora yalnızca çok kiracılı SaaS olarak çalışır.** Lisanslı self-hosted mod kapatılır: lisans doğrulama, kısıtlı mod, lisans registry'si, `messora-license` CLI ve müşteri kurulum dosyası devre dışı kalır ya da kaldırılır. Hangisinin silinip hangisinin pasif bırakılacağı teknik analizde netleşir. `MESSORA_MODE` anahtarı şimdilik yok.
- **Veri izolasyonu:** tek PostgreSQL, her satırda `tenant_id`, satır bazlı güvenlik (RLS). İstek bağlamı, kuyruk işleri, Centrifugo kanalları, Redis anahtarları, medya depolaması ve API anahtarları kiracıya göre ayrılır.
- **Adresleme:** `firma.messora.tech` alt alan adları (wildcard DNS hazır) ve isteğe bağlı özel alan adı, otomatik TLS ile.
- **Firma oluşturma:** platform yönetici panelinden ya da self sign-up ile (`signup_enabled` platform ayarı; WP2, §5.6).

### 1.2 Kaldırılanlar
- **Firma bazında tema motoru kaldırılır.** Tüm firmalar Messora'nın tek, sabit görünümünü kullanır; **firma logosu kalır** (giriş ekranı, sol üst, e-postalar).
- **PWA (uygulama olarak yükleme) kaldırılır:** manifest, service worker ve ayarlardaki PWA sekmesi.
- **Masaüstü modu tamamen kaldırılır.**
- **Kullanıcılar global:** bir kişi tek hesapla birden çok firmada farklı rollerle bulunabilir (firma değiştirici).

### 1.3 Marka
- Messora'nın kendi marka kimliği (logo, renk, tipografi, dil) kanvasın "Marka kimliği" sayfasında. Seçilen konsept panelin tek temasına uygulanır.

### 1.4 Abonelik ve ödeme (öncelik: sonra)
- Kapsamda ama şimdilik gündemin sonunda.
- Akış: plan + mesaj kredisi + AI kredisi; ödeme havale/EFT (IBAN + tekil referans kodu, dekont, platform onayı); ödeme yöntemi soyutlaması (kart sonra).
- Ayrıntılar: `reports/WhatsApp SaaS rakip fiyat analizi.md` ve kanvastaki "Faz 17 · Abonelik" sayfası.
- **Kredi modelinin ön koşulu:** Meta'ya Messora'nın ödeme yapabilmesi (Solution Partner kredi hattı / BSP).

### 1.5 Planlar ve yetkiler (entitlements)
- Her firma bir **plana** bağlıdır. Plan, limitleri (numara, kullanıcı, AI ajanı, API kotası, mesaj ve AI kredisi) ve **özellik yetkilerini** belirler. Yetkiler hem backend'de hem panelde kontrol edilir.
- **E-posta:** platformun e-postaları ve firma bildirimleri **Resend** üzerinden gider (platform ayarı; SMTP `smtp.resend.com` ya da Resend API).
  - **Kendi SMTP sunucusunu kullanma yetkisi (`custom_smtp`) yalnızca Enterprise** planda vardır.
  - Diğer planlarda firma ayarlarında SMTP alanı görünmez; e-postalar platform göndericisinden, firmanın adıyla gider.
- **AI:** platformun kendi sağlayıcı anahtarları vardır (platform ayarı, şifreli). Firmalar varsayılan olarak bunları kullanır, maliyet **AI kredisinden** düşer.
  - **Kendi AI anahtarını ekleme yetkisi (`byo_ai_keys`) yalnızca Enterprise** planda vardır. Bu firmalarda platform anahtarı ve kredi kullanılmaz.
- **Meta fiyat kartı:** platform genelinde tek bir kart vardır; platform panelinden yönetilir, tüm firmalar okur. Mesaj maliyeti ve tahminler bu karttan hesaplanır.
- **Diğer yetkiler:** `custom_domain`, `public_api`, `ai_agents` ve benzerleri; plan tablosunda tutulur.

## 2. Firma başına API ve webhook

- **Her firmanın kendi API adresi:**
  - `https://<firma>.messora.tech/api/v1`; özel alan adı varsa `https://<alan-adı>/api/v1`.
  - API anahtarı yalnızca kendi firmasında geçerlidir; başka firmanın adresine giden anahtar 401 alır.
  - Kiracı, isteğin geldiği alan adından ve anahtarın sahibinden belirlenir; ikisi uyuşmazsa istek reddedilir.
- **Rate limit ve kota:** firma bazında tutulur; plan limitlerine bağlanır.
- **API dokümantasyonu:** `/docs/api` firmanın kendi adresinde açılır ve örneklerde o firmanın taban adresi görünür.
- **Giden webhook'lar** (Messora → firmanın sistemi):
  - Her firmanın kendi uç noktaları, imza sırları ve teslim kayıtları vardır.
  - Kuyruk ve yeniden deneme firma bazında adil paylaştırılır; bir firmanın bozuk uç noktası diğerlerini yavaşlatmaz.

## 3. Meta (WhatsApp) bağlantısı: iki yöntem

### 3.1 Firmanın kendi Meta uygulaması (bugünkü yöntem)
- Firma kendi Meta App ID, App Secret ve System User token'ını girer (16b'deki token sağlığı ve rehber korunur).
- **Gelen webhook adresi firmaya özeldir:** `https://<firma>.messora.tech/webhooks/whatsapp`, ya da firma anahtarlı yol `https://hooks.messora.tech/whatsapp/<firma-anahtarı>`.
- Doğrulama token'ı ve imza sırrı firmanın kendi değerleridir.
- Panel, Meta'ya girilecek callback URL'yi ve verify token'ı kopyalanabilir şekilde gösterir.

### 3.2 Messora Tech Provider (Embedded Signup)
- Firma "Meta ile bağla" ile Messora'nın Meta uygulaması üzerinden WABA'sını ve numarasını bağlar; App ID ya da token girmez.
- **Tek webhook uç noktası.** Gelen olay, WABA ve telefon numarası kimliğinden firmaya yönlendirilir. Eşleşmeyen olay reddedilir ve kaydedilir.
- **Şu an aktif değil:** Messora'nın Tech Provider başvurusu ve onayı tamamlanmadı. Bu yüzden:
  - Özellik platform ayarlarında bir anahtarla kapalı gelir.
  - Kapalıyken firma panelinde görünmez; yalnızca 3.1 yöntemi sunulur.
  - Kod ve testler sahte (mock) Meta ile yazılır. Gerçek uçtan uca test, onay sonrası ayrı bir kontrol listesiyle yapılır.

### 3.3 Ayarların yeri
- Platform yöneticisi (admin) panelinden yönetilir:
  - hangi yöntemlerin açık olduğu (Tech Provider anahtarı);
  - Messora'nın Meta App ID/Secret ve Embedded Signup yapılandırması;
  - varsayılan Graph API sürümü;
  - firma başına izin verilen yöntem (plan bazında da bağlanabilir).
- Firma tarafında yalnızca kendi bağlantısının ayarları görünür.

## 4. Platform yönetici paneli (ilk kapsam)
- **Firmalar:** oluştur, alt alan adı ve özel alan adı (DNS doğrulama, TLS durumu), askıya al / aç, firmaya geçici destek erişimi (denetim kaydına yazılır).
- **Planlar ve limitler:** numara, kullanıcı, mesaj, AI ajanı ve API kotası; abonelik akışı gelene kadar elle atanır.
- **Meta bağlantı yöntemleri** (bölüm 3.3) ve platform genel ayarları: e-posta, varsayılan AI fiyat tablosu, Meta fiyat kartı.
- **İzleme:** firma bazında kullanım (mesaj, AI, depolama), hatalı işler, Meta hesap uyarıları.

## 5. Teknik analiz (2026-11-27)

### 5.1 Mevcut durum
- **Tablolar:** 63 tablo var.
  - ~50'si kiracıya ait.
  - Platform genelinde kalacaklar: `permissions`, `roles`, `role_permissions`, `meta_rates`, `meta_rate_markets`, `ai_model_prices`, `users`.
  - Yeni tablolar: `tenants`, `tenant_domains`, `tenant_memberships`, `platform_settings`, `platform_admins`, `platform_audit_logs`, `meta_routes`.
- **Kiracı bazına çevrilecek global tekil kısıtlar:**
  - `settings.key` (birincil anahtar)
  - `contacts.phone_e164`
  - etiket, grup ve segment adları
  - `phone_numbers` varsayılan numara
  - `ai_providers.name`
  - `messages.wamid`
  - `webhook_events.payload_sha256`
  - `meta_user_preferences`
  - açık davet e-postası
- **Global kalacak tekiller (yönlendirme için):** `waba_id`, `phone_number_id`, anahtar ve token hash'leri.
- **Veritabanı erişimi:**
  - 18 servis, ~257 havuz çağrısı, 443 sqlc sorgusu.
  - Uygulama tablo sahibi rolüyle bağlanıyor, yani RLS bugün kendiliğinden atlanır.
- **Arka plan:**
  - 30 asynq task tipi ve 15 zamanlanmış iş var; payload'larda kiracı bilgisi yok.
  - Zamanlanmış işlerin çoğu bütün veriyi tarıyor.
- **Realtime:** `inbox:all`, `campaign:all`, `system:ops` gibi global kanallar kiracılar arasında sızar.
- **Bellek önbellekleri kiracıya göre ayrılmalı:** settings, consent, kampanya analitiği, SMTP; frontend'de appearance önbelleği.
- **Gelen Meta webhook'u:** tek `META_VERIFY_TOKEN` kullanılıyor. İmza, bütün hesapların sırlarıyla sırayla deneniyor; bu, çok kiracılıda **firmalar arası sahte olay** riski demek.
- **Silinecek kod:**
  - Lisans: ~2,5k satır Go, Cloudflare Worker, R2 yayın işi, kurulum dosyası araçları, ilgili dokümanlar.
  - Tema: ~1,5k satır.
  - PWA: ~0,6k satır.
  - Masaüstü modu, kaldırılırsa: ~1,3k satır.
- **Dağıtım:**
  - Caddy tek adresli.
  - Gerekecekler: `*.messora.tech` için DNS-01 wildcard sertifika, özel alan adları için on-demand TLS (`ask` uç noktası) ve `X-Forwarded-Host`.
  - Centrifugo `allowed_origins` çoklu origin'e açılmalı.
- **Testler:** `newHarness` 27 dosyada, 127 kez kullanılıyor ve tek kiracı varsayıyor. E2E alt alan adlarına (`*.localhost`) taşınmalı.

### 5.2 Mimari kararlar (öneri)
- **RLS yöntemi:**
  - Uygulama ayrı, sahibi olmayan bir rolle bağlanır.
  - Tablolarda `FORCE ROW LEVEL SECURITY` açılır.
  - Havuzdan her bağlantı alınırken `set_config('app.tenant_id', …)` çalışır, bırakılırken sıfırlanır.
  - Kiracı bağlamı yoksa erişim reddedilir (fail-closed).
  - Ek savunma olarak sorgularda açık `tenant_id` filtresi bulunur.
  - Outbox aktarıcısı, zamanlanmış işlerin dağıtıcısı ve talep eden sorgular ayrı bir `messora_system` (BYPASSRLS) rolüyle çalışır.
- **Kullanıcılar global:** üyelikler `tenant_memberships` (rol + durum) üzerinden tutulur; oturum tek bir kiracıya bağlıdır. Bir kişi birden çok firmada olabilir.
- **Kiracı çözümü:** istek alan adından çözülür. BFF, alan adını güvenli bir başlıkla Go'ya iletir. Oturum ya da API anahtarı başka bir kiracıya aitse istek 401 alır.
- **Kuyruk işleri:** kiracı bilgisi outbox üzerinden task başlığına taşınır ve bir ara katman bağlamı kurar. Zamanlanmış işler kiracı başına dağıtılır.
- **Adillik:** kiracı başına eşzamanlılık sınırı uygulanır (webhook teslimi, kampanya dağıtımı).
- **Meta webhook'u:** imza yalnızca o kiracının sırrıyla doğrulanır. Tech Provider olayları platform kutusuna düşer, `meta_routes` üzerinden kiracıya aktarılır.
- **Trigger'lar:** outbox'a ve diğer tablolara yazan trigger'lar `NEW.tenant_id`'yi kopyalar.
- **Mevcut veri:** 1 numaralı kiracıya taşınır. `tenant_id` önce boş eklenir, toplu doldurulur, sonra `NOT NULL` yapılır. Bunun bir runbook'u ve geri dönüş testi olur.

### 5.3 Riskler
| Risk | Azaltma |
|---|---|
| Unutulan kiracı filtresi / RLS'in atlanması | Sahip olmayan rol + FORCE RLS; bağlamsız erişimin reddi; CI'da "her tablo RLS'li" kontrolü; firmalar arası test matrisi |
| Havuzdaki bağlantıda eski kiracının kalması | Her alımda ayarlama, bırakılırken sıfırlama |
| Bağlamsız worker ve zamanlanmış işler | Task başlığı + ara katman; eksikse işi durdur; sistem rolü yalnızca dar yollarda |
| Firmalar arası sahte webhook | Kiracıya özel doğrulama; Tech Provider'da yalnızca `meta_routes` |
| Global önbellekler ve kanallar | Kiracı anahtarlı önbellekler; kanal adlarında kiracı öneki; Authorize'da kiracı kontrolü |
| Gürültülü komşu | Kiracı başına hız ve eşzamanlılık sınırları |
| On-demand TLS kötüye kullanımı | `ask` yalnızca doğrulanmış alan adlarına evet der |
| Büyük tablolarda migrasyon kilidi | Boş sütun + toplu doldurma + `NOT VALID`/`VALIDATE` |

### 5.4 İş paketleri
| # | Paket | Boyut | Bağımlılık |
|---|---|---|---|
| WP0 | Temizlik: lisans, tema, PWA (masaüstü modu kararına göre) | M | — |
| WP1 | Kiracı temeli: tablolar, `tenant_id`, kiracı bazlı tekiller, RLS, roller, havuz kancası, alan adı çözümü, trigger düzeltmeleri, mevcut verinin 1 numaralı kiracıya taşınması | L | WP0 |
| WP2 | Kimlik ve platform yönetici paneli: global kullanıcı + üyelik, alan adına bağlı oturum ve anahtar, kiracı adresli e-posta bağlantıları, kurulum sihirbazı yerine platform yöneticisi açılışı, firmalar, alan adları, askıya alma, destek erişimi | L | WP1 |
| WP3 | Arka plan, realtime, Redis ve önbellekler: kiracı bilgisinin taşınması, kanallar, adillik, medya yolları | M–L | WP1 (WP2 ile paralel) |
| WP4 | Firma başına API, webhook ve doküman; plan bazlı kotalar | M | WP2, WP3 |
| WP5 | Firmanın kendi Meta uygulaması: kiracıya özel webhook adresi, doğrulama ve sırlar | M | WP1, WP3 |
| WP6 | Tech Provider / Embedded Signup (anahtarla kapalı, sahte Meta ile) | M–L | WP5 |
| WP7 | Dağıtım: wildcard + on-demand TLS, alan adı iletimi, Centrifugo origin'leri, alt alan adında E2E | M | WP2 (erken başlayabilir) |
| WP8 | Planlar ve yetkiler: plan tablosu, limitler ve yetki kontrolü; Resend platform göndericisi + Enterprise'a özel SMTP; platform AI anahtarları + Enterprise'a özel kendi anahtarı; platform geneli Meta fiyat kartı | M | WP2, WP3 |
| WP2b | Kimlik akışları ve e-posta tasarımları: giriş, kayıt (self sign-up, firma + alt alan adı), e-posta doğrulama, şifremi unuttum/sıfırlama, davet kabulü, 2FA, firma değiştirici. Şık ve markaya uygun UI/UX; markalı, TR/EN, mobil ve koyu mod uyumlu HTML e-posta şablonları (doğrulama, şifre sıfırlama, davet, hoş geldin, güvenlik uyarısı, fatura ve ödeme hatırlatması); Resend ile gönderim, önizleme ve e-posta testleri | M–L | WP2 |
| WP9 | Abonelik ve faturalama: havale/EFT, fatura, mesaj ve AI kredisi bakiyeleri, platform onayı, ek süre ve kısıtlı mod (docs/design/faz17-kanvas) | L | WP8 |

### 5.5 WP1 uygulama kararları (2026-11-27)
- **Şema:** migrasyon 00024–00028. 57 tabloya `tenant_id` eklendi (`outbox_events` boş olabilir), 13 ebeveyne `(tenant_id, id)` anahtarı, 23 ilişkiye bileşik yabancı anahtar. Platform tabloları: `tenants`, `tenant_domains`, `platform_settings`, `permissions`, `roles`, `role_permissions`, `meta_rates`, `meta_rate_markets`, `ai_model_prices`, `goose_db_version`. Ayrıntı: DATABASE_DESIGN.md §1.1, SECURITY.md §3a.
- **`users` WP1'de kiracıya ait:** e-posta şimdilik bütün kiracılarda tekil kalır. WP2'de global kullanıcı + `tenant_memberships` gelince `users.tenant_id` üyeliğe taşınır; oturum ve API anahtarı kiracıya bağlı kalır.
- **`tenant_id` sorgularda geçirilmiyor:** sütunun varsayılanı bağlantının kiracısı (`app_tenant_id()`), RLS'in `WITH CHECK`'i başka değeri reddeder. Böylece tek kaynak bağlamdaki kiracıdır; sqlc parametre yapılarında unutulan bir alanın `uuid.Nil` yazması gibi bir hata olamaz. Kiracısız bağlantıda ekleme `NOT NULL` ile başarısız olur. Trigger'lar `NEW.tenant_id`'yi açıkça kopyalar.
- **Doldurma tek transaction'da:** gruplu doldurma yerine her tablo tek `UPDATE` ile, migrasyon transaction'ı içinde taşınır; yarım kalan migrasyon geri alınır. Mevcut veri küçük (demo kapatılıyor). `NOT VALID` kontrolü ve doğrulaması ayrı migrasyonlarda.
- **`tenant.System`:** uygulama havuzunda "kiracısız" demektir (kiracı satırı görünmez, yazılamaz; yalnızca platform tabloları). Alan adı çözümü ve sağlık kontrolleri bunu kullanır. Sistem havuzu (`messora_system`) yalnızca worker'da, outbox aktarıcısı ve temizliği için açılır; API'de BYPASSRLS bağlantısı yoktur.
- **Alan adı:** `X-Forwarded-Host` kullanılmıyor (Caddy `Host`'u koruyor); BFF başlığı yalnızca BFF sırrıyla. 1 numaralı kiracının alan adlarını `messora migrate` kaydeder (`PUBLIC_APP_URL` host'u ve `<slug>.TENANT_BASE_DOMAIN`); WP2'de platform paneline geçer.
- **Platform verisini sahip rolü yazar:** AI fiyat tablosunun tohumu API açılışından `messora migrate`'e taşındı; "bir kez tohumlandı" işareti `platform_settings`'te (migrasyon 00029 eski işareti taşır). `messora_app` platform tablolarına yazamadığı için başka bir platform tohumu da aynı yoldan gider.
- **Doğrudan SQL yazan araçlar kiracıyı belirtir:** E2E yardımcıları `PGOPTIONS=-c app.tenant_id=…` ile 1 numaralı kiracı olarak yazar; yedek provası `tenant_id`'yi açıkça verir. Kiracısız ekleme `NOT NULL`'a takılır, bu bilinçli.
- **Kuyruk (WP3'ün bir kısmı öne alındı):** outbox satırı kiracıyı taşır, aktarıcı asynq görev başlığına (`messora-tenant`) yazar, worker ara katmanı bağlamı kurar. Başlıksız görevler (zamanlanmış işler) `TODO(WP3)` işaretli geçici kuralla 1 numaralı kiracı olarak çalışır.
- **Önbellekler:** ayarlar, SMTP, onay ve kampanya analitiği ayarları kiracı başına (`tenant.Cache`); panelin görünüm önbelleği alan adı başına. `mailer.Sender.Enabled` bağlam alır.
- **WP2/WP3'e kalanlar:** realtime kanalları, Redis anahtarları (MFA, TOTP, hız sınırı), başarısız işler sayfası (asynq arşivi bütün kiracıları gösterir), zamanlanmış işlerin kiracı başına dağıtımı ve başlıksız görevin reddi (WP3); `meta_rates` / `ai_model_prices` yazma yetkisinin platforma taşınması, `APP_*` çalışma alanı ortam değişkenleri, global kullanıcılar, kiracı açma ve alan adı yönetimi, `messora setup token`'ın kiracı seçmesi (WP2); gelen Meta webhook'unun kiracıya özel adresi (WP5).

### 5.6 WP2 uygulama kararları (2026-11-28)
- **Şema:** migrasyon 00030. `users` global (RLS yok, e-posta platform genelinde tekil, `citext`); her kullanıcı rolü ve durumuyla `tenant_memberships`'e (birincil anahtar `(tenant_id, user_id)`, RLS'li) 1 numaralı kiracının üyesi olarak taşındı, `user_roles` kaldırıldı. Eski "devre dışı kullanıcı" artık devre dışı **üyeliktir**; hesap aktif kalır. Kurtarma kodları kişiye ait (global). `user_sessions` kiracıya bağlı kalır, destek oturumu için `support_grant_id` ve `read_only` aldı. Yeni platform tabloları: `platform_sessions`, `email_verification_tokens`, `platform_audit_logs`; `support_access_grants` kiracıya bağlı ve RLS'li (firma kendi destek kayıtlarını görür). `tenants`'a `subscription_status` (`trial | active | past_due | canceled`), `trial_ends_at`, `suspended_reason`; `tenant_domains`'e TXT doğrulama alanları. Down migrasyonu, birden çok firmada üyeliği olan kişi varsa bilerek reddeder.
- **Yaşam döngüsü ile abonelik ayrı:** `tenants.status` (`active | suspended | deleted`) erişimi belirler; deneme `subscription_status = 'trial'` + `trial_ends_at` (14 gün). Deneme bitince ne olacağı WP8/WP9'un işi; WP2 yalnızca saklar ve platform panelinden düzenletir.
- **Platform rolü — `messora_platform`:** `NOLOGIN`, `NOBYPASSRLS`, hiçbir şeyin sahibi değil. `messora_app`'e `WITH INHERIT FALSE, SET TRUE` ile verildi: API'nin normal havuzu bu yetkileri **miras almaz**; yalnızca `tenant.Platform` bağlamıyla alınan bağlantı `SET ROLE messora_platform` yapar, bırakılırken `RESET ROLE; RESET app.tenant_id` çalışır (başarısızsa bağlantı atılır). Neden `messora_system` ya da SECURITY DEFINER değil: (1) API'de BYPASSRLS bağlantısı olmaması WP1'in temel kuralı; `messora_system` yalnızca worker'da kalır. (2) SECURITY DEFINER fonksiyonları kayıt, firma açma, destek, askıya alma, alan adı ve fiyat kartı yazımı gibi her işlem için ayrı SQL API'si demek; mevcut sqlc sorgularını ve servisleri olduğu gibi kullanmak (fiyat kartı ve AI fiyat servisleri değişmeden platform panelinde çalışıyor) daha az hata yüzeyi. (3) Platform rolünün yetkisi dar ve açık: kayıt tabloları + katalog + platform tabloları üzerinde DML; kiracılar arası yalnızca `tenant_memberships`, `user_sessions` ve `support_access_grants`'ta açık `platform_access` politikası; diğer her kiracı tablosu yine `app.tenant_id`'ye bağlı. Firmaya ait satır yazan platform işlemi (ilk ayarlar, sahip daveti, firma denetim kaydı, destek oturumu) transaction'ı `set_config('app.tenant_id', …, true)` ile o firmaya daraltır. Risk: SQL enjeksiyonu `SET ROLE` çalıştırabilir — `app.tenant_id` değiştirmekle aynı güven seviyesi; bütün sorgular sqlc ile parametreli.
- **Alan adları:** `TENANT_BASE_DOMAIN` (ör. `messora.tech`) platformun adresidir. `admin.<base>` platform paneli (istekler platform işi olarak çalışır), `<base>`, `www.<base>`, `app.<base>` herkese açık platform adresi (kayıt ve e-posta doğrulama; kiracısız). Diğer her host `tenant_domains`'ten çözülür. Bu adresler hiçbir zaman bir kiracıya verilmez (`messora migrate` `PUBLIC_APP_URL` host'u platform adresiyse 1 numaralı kiracıya kaydetmez). Bağlantılar `PUBLIC_APP_URL`'in şemasını ve portunu alır (geliştirmede `http://acme.messora.localhost:8090`).
- **Operasyonların host kapsamı:** OpenAPI uzantısı `x-messora-host`: varsayılan `tenant` (firma adresi), `platform` (yalnızca admin host'u; `/platform/*` ve yalnızca onlar), `any` (kayıt, doğrulama, yeniden gönderme, adres kontrolü, yetenekler, görünüm). Yanlış host'ta işlem yokmuş gibi 404 döner. Guard açılışta spec'i denetler: `/platform/*` dışındaki hiçbir işlem `platform` şemasını kabul edemez, platform işlemleri oturum ya da API anahtarı kabul edemez.
- **Oturumlar kiracıya bağlı:** firma oturumu (`mss_`) yalnızca açıldığı firmanın adresinde, etkin üyelikle geçerli; platform oturumu (`msp_`, ayrı tablo, ayrı cookie) yalnızca admin host'unda. Hesabı üyesi olmadığı firmada giriş yapmak yanlış parola gibi `401 INVALID_CREDENTIALS` döner (adres başka firmalar hakkında bilgi vermez). Parola doğruysa ama adres doğrulanmamışsa `403 EMAIL_NOT_VERIFIED`, platform yöneticisi sıfırlama istediyse `403 PASSWORD_RESET_REQUIRED`.
- **Firma değiştirme (hand-over):** `POST /auth/switch-workspace` hedef firmadaki üyeliği platform rolüyle doğrular ve Redis'e tek kullanımlık, 60 saniyelik bir token yazar (`messora:handoff:<sha256>`); bağlantı `https://<hedef>/auth/handoff?token=`. Token ilk denemede (`GETDEL`) tükenir ve yalnızca hedef firmanın host'unda oturuma dönüşür; başka host'ta denenirse yine tükenir (`401 HANDOFF_INVALID`). 2FA kaynak oturumda geçilmişti; hedefte tekrar sorulmaz.
- **Redis anahtarları:** MFA giriş denemesi nerede başladıysa oraya bağlı (`messora:mfa:tenant:<id>:<hash>` / `messora:mfa:platform:<hash>`); TOTP tekrar kullanım koruması (`messora:totp:used:<user>`), giriş/parola/yeniden doğrulama hız sınırları ve kilitlenme kişiye ait (global). API bütçesi oturum ve anahtar için firma başına (`api:<tenant>:user:<id>`), platform için `api:platform:…`.
- **Kayıt (self sign-up):** `POST /auth/signup` platform adresinde (ya da `any` olduğu için firma adresinde) çalışır; tek platform transaction'ında global hesap (doğrulanmamış), kiracı (`trial`, 14 gün), `<slug>.<base>` alt alan adı (doğrulanmış, birincil), sahip üyeliği, firmanın ilk ayarları (ad, dil, saat dilimi `tr` → Europe/Istanbul), doğrulama token'ı ve iki denetim kaydı (firma + platform) yazılır; e-posta commit'ten sonra gider. Slug kuralları formla aynı: `a–z0–9-`, 3–30, baş/son/çift tire yok; ayrılmış liste `tenant.ReservedSlugs` (www, admin, api, app, hooks, dokploy, cdn, mail, docs, status, help, support) — Go, `tenant_reserved_slugs()` ve `frontend/src/lib/auth/slug.ts` testlerle karşılaştırılır. Silinmiş kiracının slug'ı kimseye verilmez. `platform_settings.signup_enabled` (varsayılan açık) kapalıyken `403 SIGNUP_DISABLED`; `TENANT_BASE_DOMAIN` yoksa da kayıt kapalıdır. IP başına 10 dakikada 5 kayıt.
- **E-posta doğrulama:** token 24 saat, tek kullanımlık; bağlantı yeni firmanın adresinde (`/verify-email?token=`). Hatalar sözleşmedeki gibi: `400 VERIFICATION_TOKEN_INVALID`, `410 VERIFICATION_TOKEN_EXPIRED`, `409 EMAIL_ALREADY_VERIFIED`. Yeniden gönderme adres başına dakikada bir (`429` + `Retry-After`), eski bağlantıları geçersiz kılar ve bilinmeyen adreste de `202` döner. Davetle gelen hesap, bağlantıyı izlediği için doğrulanmış sayılır; parola sıfırlama bağlantısını izlemek de adresi doğrular. Şablondaki kod isteğe bağlı oldu (sözleşmede kodla doğrulama yok).
- **Var olan hesabın daveti:** ikinci firmaya davet edilen kişi yeni hesap açmaz: davet önizlemesi `account_exists` döner, kabul hesabın kendi parolasıyla yapılır (giriş hız sınırları ve kilitlenme geçerli). Üyesi olduğu firmaya tekrar davet `409 USER_EXISTS`.
- **Parola değişince her yerde çıkış:** parola değişikliği, sıfırlama, e-posta değişikliği, hesabın devre dışı bırakılması ve zorunlu sıfırlama bütün firmalardaki ve platform panelindeki oturumları platform rolüyle kapatır (değişikliği yapan oturum hariç).
- **E-postalar:** kimlik e-postalarının hepsi markalı şablonlarla, kişinin dilinde, firmanın adı ve logosuyla, firmanın birincil adresine mutlak bağlantıyla gider: `verify_email`, `welcome` (doğrulamadan sonra), `password_reset`, `password_changed`, `invitation` (platformun sahip davetinde gönderen "Messora"), `login_alert` (kişinin o firmada daha önce görülmemiş IP'den girişi; ilk giriş hariç), `email_change_confirm` (yeni adrese). `two_factor_code` kullanılmıyor (2FA TOTP). Gönderici platformun SMTP ayarı (üretimde Resend: `smtp.resend.com:465`, `SMTP_TLS=tls`, kullanıcı `resend`); firma SMTP'si kod yolu duruyor, `settings.Mail.customSMTP` `TODO(WP8)` kancası `custom_smtp` yetkisini bekliyor. `PUBLIC_APP_URL` http ise (geliştirme/E2E) şablonlar http bağlantılara izin verir; üretimde yalnızca https.
- **`APP_*` ortam ayarları** yalnızca 1 numaralı kiracıya uygulanır; diğer firmaların ayarları kendilerinindir.
- **Kurulum sihirbazı kaldırıldı:** `/setup` uç noktaları, `messora setup token` ve sihirbazın sahip oluşturma adımı yok. Yerine: platform yöneticisi yokken `messora migrate` tek kullanımlık, 24 saatlik bir açılış bağlantısı yazar (`https://admin.<base>/bootstrap?token=msb_…`; her çalıştırmada yenilenir, hash'i `platform_settings`'te) ve `messora admin create --email … [--name …] [--password-stdin]` hesabı platform yöneticisi yapar (yoksa doğrulanmış olarak açar). Firma sahipleri platform panelinden davetle gelir; 1 numaralı kiracının mevcut kullanıcıları olduğu gibi kalır.
- **Platform paneli API'si (`/platform/*`):** giriş + TOTP (platform yöneticisi için 2FA zorunlu; kurulmadan önce yalnızca kayıt işlemleri açık, tenant panelinden kapatılamaz), genel bakış sayaçları, firmalar (liste/arama/durum, ayrıntı, oluşturma + isteğe bağlı sahip daveti, ad, deneme bitişi, askıya alma gerekçesiyle / açma), alan adları (özel alan adı, `_messora.<alan>` TXT = token doğrulaması, birincil yapma, silme; alt alan adı silinemez), hesaplar (global arama, üyelikler, devre dışı/etkin, zorunlu parola sıfırlama — bağlantı ilk etkin firmasından gider), destek erişimi, platform ayarları (`signup_enabled`, yanıt adresi; taban alan adı ve gönderici salt okunur), platform denetim kaydı, Meta fiyat kartı ve AI fiyat tablosu. Bütün platform işlemleri `platform_audit_logs`'a, firmayı ilgilendirenler ayrıca firmanın `audit_logs`'una yazılır. Public API dokümanı `/platform/*` içermez (`apidocs.IsPlatformPath`, test).
- **Destek erişimi:** platform yöneticisi gerekçe ve süreyle (5–240 dk, varsayılan 30) bir destek izni açar; varsayılan salt okunur, yazma açıkça seçilir. İzin firmanın satırıdır (firma `GET /support-access` ile görür) ve firmanın denetim kaydına `support.grant` + `support.session_open` olarak yazılır; oturum hand-over bağlantısıyla firmanın adresinde açılır. Salt okunur oturumda GET dışındaki her istek `403 SUPPORT_READ_ONLY`; destek oturumu yeniden doğrulama yapamaz (API anahtarı, 2FA, rol değişikliği gibi hassas işlemler kapalı), parola/profil/e-posta değiştiremez, firma değiştiremez, üye listesinde görünmez. `/me` `support` alanıyla döner, panel banner gösterir. İzin bitince ya da sonlandırılınca oturum hemen geçersiz.
- **Meta fiyat kartı ve AI fiyatları:** yazma işlemleri (`importMetaRates`, `createAIPrice`, `updateAIPrice`, `deleteAIPrice`, `importAIPrices`) `/platform/meta-rates/*` ve `/platform/ai-prices/*`'a taşındı; `messora_app` bu tablolara artık yazamaz (migrasyon). Firmalar listeyi okumaya devam eder.
- **Testler:** `identity_wp2_integration_test.go` (kayıt, doğrulama hataları, yeniden gönderme sınırı, üyelikler, hand-over, host'a bağlı oturum, var olan hesabın ikinci firmaya daveti, platform girişi ve kapsam ayrımı, askıya alma, alan adı doğrulama, destek erişimi denetimi, fiyat kartının taşınması, hesap kapatma/zorunlu sıfırlama), `TestTenantIsolationMatrix`'e yeni uçlar, `TestMigrationMovesUsersIntoMemberships`, `TestPlatformRole`, slug listesi karşılaştırmaları.
- **WP3+'e kalanlar:** zamanlanmış kimlik temizliği (`Cleanup`) hâlâ başlıksız görev olarak 1 numaralı kiracıyı temizliyor (platform oturumları aynı işte temizleniyor); kiracı başına dağıtım WP3. Realtime kanallarında destek oturumu salt okunur olarak inbox olaylarını görür (kanal yetkisi WP3'te kiracıya ayrılırken korunmalı). Özel alan adı TLS'i (on-demand `ask`) WP7'de doğrulanmış alan adlarına bağlanacak. Plan/yetki (`custom_smtp`, kullanıcı limiti, deneme bitişinde kısıt) WP8/WP9.

## 6. Kabul kriterleri (taslak)
1. İki firmanın verisi hiçbir API, olay, dışa aktarma, kuyruk işi ya da realtime kanalında birbirine görünmez. RLS ve uygulama katmanı testlerle doğrulanır; bir firmanın anahtarıyla diğerinin adresine yapılan istek reddedilir.
2. Her firma kendi adresinden API'ye erişir, kendi webhook'larını alır; firmaya özel gelen Meta webhook'u doğru firmaya işlenir.
3. Tech Provider kapalıyken hiçbir arayüzde görünmez. Açılınca Embedded Signup akışı mock Meta ile uçtan uca çalışır.
4. Tema ve PWA kodu ile ayarları kaldırılmıştır; panel tek temayla çalışır.
5. Lisans modu kapalıdır; lisans olmadan da hiçbir özellik kısıtlı moda düşmez.
6. Firma oluşturma, askıya alma ve alan adı ekleme platform panelinden yapılır ve denetim kaydına yazılır.

## 7. Durum

| İş | Durum |
|---|---|
| Teknik analiz ve iş paketleri | ✅ tamamlandı (2026-11-27) — onay bekliyor |
| WP0 — temizlik (lisans, tema, PWA, masaüstü modu) | ✅ tamamlandı (2026-11-27); migrasyon 00023, eski service worker bir sürüm boyunca kendini kaldırır |
| WP1 — kiracı temeli | ✅ tamamlandı (2026-11-27); migrasyon 00024–00028, RLS + `messora_app`/`messora_system`, alan adından kiracı, kiracılar arası test matrisi (kararlar §5.5) |
| WP2 — kimlik ve platform yönetici paneli | ✅ tamamlandı (2026-11-28); migrasyon 00030, global hesap + üyelik, `messora_platform` rolü, kayıt + doğrulama, firma değiştirme, platform paneli (`admin.<base>`), destek erişimi, markalı kimlik e-postaları (kararlar §5.6) |
| Marka kimliği (kanvas) | taslak 1 — geri bildirim bekliyor |
| Abonelik tasarımı (kanvas) | taslak — öncelik sonra |
