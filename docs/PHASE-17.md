# Phase 17 — Çok kiracılı SaaS (Messora Cloud)

Durum: **kapsam taslağı** (2026-11-27). Uygulamaya başlarken, 16C'de yaptığımız gibi önce teknik analiz yapılır: mevcut kod taranır, riskler ve iş paketleri bu belgeye eklenir, ürün sahibinin onayıyla başlanır. Tasarım kanvası: https://claude.ai/artifact/5gT44f3kyVA6ePXMGMidCX.

## 1. Kararlar

### 1.1 Ürün yönü
- **Messora yalnızca çok kiracılı SaaS olarak çalışır.** Lisanslı self-hosted mod kapatılır: lisans doğrulama, kısıtlı mod, lisans registry'si, `messora-license` CLI ve müşteri kurulum dosyası devre dışı kalır ya da kaldırılır. Hangisinin silinip hangisinin pasif bırakılacağı teknik analizde netleşir. `MESSORA_MODE` anahtarı şimdilik yok.
- **Veri izolasyonu:** tek PostgreSQL, her satırda `tenant_id`, satır bazlı güvenlik (RLS). İstek bağlamı, kuyruk işleri, Centrifugo kanalları, Redis anahtarları, medya depolaması ve API anahtarları kiracıya göre ayrılır.
- **Adresleme:** `firma.messora.tech` alt alan adları (wildcard DNS hazır) ve isteğe bağlı özel alan adı, otomatik TLS ile.
- **Firma oluşturma:** ilk etapta platform yönetici panelinden. Self sign-up kanvastaki akışla sonra açılır.

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
- **Kuyruk (WP3'ün bir kısmı öne alındı):** outbox satırı kiracıyı taşır, aktarıcı asynq görev başlığına (`messora-tenant`) yazar, worker ara katmanı bağlamı kurar. Başlıksız görevler (zamanlanmış işler) geçici kuralla 1 numaralı kiracı olarak çalışıyordu; WP3'te kaldırıldı (§5.7).
- **Önbellekler:** ayarlar, SMTP, onay ve kampanya analitiği ayarları kiracı başına (`tenant.Cache`); panelin görünüm önbelleği alan adı başına. `mailer.Sender.Enabled` bağlam alır.
- **WP2/WP3'e kalanlar:** realtime kanalları, Redis anahtarları (MFA, TOTP, hız sınırı), başarısız işler sayfası (asynq arşivi bütün kiracıları gösterir), zamanlanmış işlerin kiracı başına dağıtımı ve başlıksız görevin reddi (WP3); `meta_rates` / `ai_model_prices` yazma yetkisinin platforma taşınması, `APP_*` çalışma alanı ortam değişkenleri, global kullanıcılar, kiracı açma ve alan adı yönetimi, `messora setup token`'ın kiracı seçmesi (WP2); gelen Meta webhook'unun kiracıya özel adresi (WP5).

### 5.7 WP3 uygulama kararları (2026-11-28)
- **Zamanlanmış işler kiracı başına:** zamanlayıcı veritabanına bağlanmaz; her dilimi tek bir `platform:tenant_fanout` görevi olarak kuyruğa alır. Worker aktif kiracıları sistem havuzundan okur (`tenant.ListActive`, askıdaki/silinen hariç) ve işi her kiracı için başlıkla, `sched:<iş>:<dilim>:<kiracı>` kimliğiyle kuyruğa alır; tekrar eden dağıtım kimseyi iki kez kuyruğa almaz. Kiracı işleri RLS altında çalıştığı için eski "bütün veriyi tara" işleri değişmeden kiracı başına çalışır. Platform işi yalnızca `maintenance:outbox_cleanup` (sistem havuzu).
- **Başlıksız görev reddedilir:** `queue.TenantMiddleware` başlığı olmayan görevi çalıştırmadan `SkipRetry` ile ölü mektuba gönderir. İzin listesi iki platform görevidir (`platform:tenant_fanout`, `maintenance:outbox_cleanup`), `tenant.System` ile çalışırlar. `scheduler.New` işin platform bayrağıyla task tipini karşılaştırır.
- **Saklama:** dışa aktarma temizliği (7 gün) kiracı başına çalışan analitik uzlaştırmasında kaldı; içe aktarmalar için yeni `contacts:import_retention` (6 saatte bir, 30 gün: dosya, hata raporu ve satır).
- **Realtime:** tek Centrifugo namespace'i `tenant`; kanal adları `tenant:<tenant_id>:<mantıksal ad>`. Centrifugo namespace adı en az iki karakter istediği için `t:` yerine `tenant:` (Centrifugo 6.10 doğrulaması). Kullanıcıya sınırlı kanallar kaldırıldı (kişisel kanal yalnızca bağlantı token'ının sunucu tarafı aboneliği). Yayıncı öneki outbox satırının kiracısından alır; token'lar yalnızca oturumun kiracısının kanallarına; Centrifugo kullanıcısı `<kiracı>:<kullanıcı>` (WP2'de aynı kişinin başka kiracıda aldığı token geçmesin diye). Bağlantı token'ı `channel_prefix` döner; panel mantıksal adlarla çalışır, sağlayıcı öneki ekleyip siler. `messora init` Centrifugo yapılandırmasını yeniden yazar; yükseltmede Centrifugo yeniden başlatılır.
- **Redis:** kiracı anahtarları `messora:t:{<tenant_id>}:…` (hash tag kiracı). API hız sınırı oturum/anahtar başına kiracıda; anonim sınır IP başına global; gönderim zamanlayıcısı ve AIMD kiracıda. Kimlik anahtarları (giriş kilidi, MFA, TOTP) kişi geneli kaldı; kimlik koduna dokunulmadı, `tenant.GlobalRedisKey` ve `TODO(WP2)` bırakıldı.
- **Adillik:** (1) kuyruk ağırlıkları; (2) process başına kiracı payı (`queue.TenantLimiter`, varsayılan eşzamanlılığın %50'si; payı dolu kiracının görevi ~1 sn ertelenir, deneme sayılmaz); (3) kiracı başına kampanya kovası (varsayılan 200 mesaj/sn). Ayarlar `platform_settings.fairness` (`tenant_share`, `tenant_campaign_mps`), 30 sn önbellek. Redis semaforu yerine bellek içi sayaç: tur yok, ölen process slot sızdırmaz; sınır her replikada ayrı tutulur. Model: ARCHITECTURE.md §10.1.
- **Başarısız işler:** liste, sayılar, tekrar deneme ve silme isteğin kiracısıyla sınırlı (`system.TenantScope`); platform görünümü `system.PlatformScope` (WP2 yönetici paneli için). Kiracı sayısı kuyruk başına en yeni ~1000 arşiv kaydından hesaplanır.
- **Önbellekler:** WP1'deki kiracı önbelleklerine ek olarak gönderim tavanı önbelleği (kiracı, numara) anahtarlı. Fiyat kartı ve AI fiyatları için süreç içi önbellek yok (platform tabloları her seferinde okunuyor).
- **Kilitler ve idempotency:** `LockPortfolio` advisory lock anahtarı kiracıyı içeriyor (`messora.portfolio:<kiracı>:<hesap>`); idempotency birincil anahtarı WP1'den beri `(tenant_id, principal, key)`.
- **Medya:** `/media/s/` imzası nesnenin kiracısını kapsar, rota kiracı ara katmanının arkasında; isteğin kiracısı uyuşmazsa 403. Bağlantı onu üreten isteğin alan adına işaret eder. `/media/p/` alan adından bağımsız. Migrasyon gerekmedi.

### 5.8 WP5 uygulama kararları (2026-11-28)
- **İki yöntem, WP5 birinciyi tamamlar.** Firmanın kendi Meta uygulaması (App ID, App secret, System User token, WABA ID) tamamen çalışır. Tech Provider kartı panelde "yakında" olarak **devre dışı** görünür (ürün sahibinin WP5 talebi; §3.2'deki "kapalıyken görünmez" yerine) ve `platform_settings.tech_provider` (`enabled: false`, `app_id`/`configuration_id` yer tutucuları) anahtarına bağlıdır. WP6 bu anahtarı ve platform panelini kullanır; Tech Provider'ın app secret'ı `platform_settings`'te düz metin tutulmayacak.
- **Adres:** `https://hooks.<TENANT_BASE_DOMAIN>/meta/<anahtar>` (§3.1'deki `/whatsapp/<anahtar>` yerine). Taban `HOOKS_PUBLIC_URL`, varsayılan `https://hooks.<TENANT_BASE_DOMAIN>`; taban alan adı yoksa `<PUBLIC_APP_URL>/webhooks`. Aynı anahtar `/webhooks/meta/<anahtar>` altında da yanıt verir (Caddy'nin mevcut `/webhooks/*` rotası, alt alan adı TLS'i gelmeden de çalışır); Caddy `/meta/*`'ı da API'ye yönlendirir. Anahtarlı yol alan adından bağımsızdır: kiracıyı anahtar belirler.
- **Anahtar:** 32 rastgele bayt, küçük harf base32 (52 karakter). Panelde gösterildiği için `Sealer` ile şifreli saklanır; arama SHA-256 ile. Doğrulama belirteci de aynı biçimde, firma başına, şifreli.
- **Kiracının önceden çözümü:** platform yönlendirme tablosu `meta_webhook_keys` (hash → kiracı, `expires_at`). SECURITY DEFINER arama fonksiyonu yerine tablo, çünkü FORCE RLS sahibe de uygulanır ve API'de BYPASSRLS bağlantı yok (WP1); `tenant_domains` ile aynı desen. Yazma yalnızca `meta_webhook_key_rotate()` (SECURITY DEFINER, çağıranın `app.tenant_id`'si) ile; `messora_app`'in tabloya `INSERT/UPDATE/DELETE` yetkisi yok. Önbellek yok: birincil anahtar aramasının maliyeti olayı yazmaktan küçük, rotasyon anında etkili.
- **Alan adlı yol korunur:** firmanın kendi adresindeki `/webhooks/whatsapp` (ve yeni `/webhooks/meta`) aynı doğrulamayla çalışır; mevcut kurulumlar bozulmaz, panelde "alternatif adres" olarak gösterilir. Tercih edilen adres anahtarlıdır (alan adı değişse de değişmez).
- **İmza sırları:** firmanın aktif hesaplarının app secret'ları (RLS ile yalnızca o firmanınki). Ayrı bir "bağlantı düzeyi" app secret alanı açılmadı; hesap bağlarken **App ID ve App secret artık zorunlu** (API, panel, kurulum sihirbazı). Sırrı olmayan eski hesaplar panelde uyarıyla görünür.
- **WABA eşleşmesi:** imzası doğru gövdede firmada bağlı olmayan `entry[].id`'ler atılır, loglanır, sayılır (son WABA kimliği panelde); kalan girdiler saklanır. Yanıt `200` (red Meta'nın yeniden denemesini anlamlı kılmaz, karışık paketlerde firmanın kendi olayları kaybolmaz). Gövde tamamen firmanınsa olduğu gibi, değilse süzülmüş hâliyle saklanır; dedupe süzülmüş gövdenin hash'iyle `(tenant_id, payload_sha256)`.
- **Rotasyon:** yeni adreste eski anahtar 0–168 saat (panelde hemen / 24 sa / 3 gün / 7 gün) geçerli kalır, en fazla bir eski anahtar. Doğrulama belirteci hemen değişir (Meta onu yalnızca doğrulamada ister). Yeniden kimlik doğrulama istenmiyor (diğer kanal işlemleriyle tutarlı; `channels.manage` gerekir), işlemler denetim kaydına yazılır.
- **Bağlantı testi:** `POST /channels/accounts/{id}/test`: `debug_token` (uygulamanın app token'ıyla; token başka uygulamanınsa uyarı), `/<WABA>/phone_numbers`, `GET /<WABA>/subscribed_apps` (uygulama yoksa `POST` ile abone eder), son webhook ve imza hatası. Sonuç anahtar/durum/kod listesi; panel kodları çevirir. Token ve numara bilgisi `CheckToken` gibi kaydedilir.
- **Sağlık:** `meta_connections`'ta son webhook (dakikada en fazla bir yazma), imza hatası sayısı/zamanı, WABA uyuşmazlığı sayısı/zamanı/kimliği.
- **`META_*` ortam değişkenleri:** `META_APP_ID`, `META_APP_SECRET`, `META_VERIFY_TOKEN`, `META_ACCESS_TOKEN`, `META_WABA_ID` kaldırıldı, doğrulamada yok; ayarlıysa API açılışta uyarır. `DEV_` tohumu **yok** (sağlamlık: tek yol panel/API, testler mock Graph ile bağlanır). `messora init` artık doğrulama belirteci üretmiyor. Platform düzeyinde yalnızca `META_GRAPH_API_VERSION`, `META_GRAPH_BASE_URL`, `META_DRY_RUN`, `HOOKS_PUBLIC_URL`. Giden Graph çağrıları zaten hesabın token'ıyla yapılıyordu; global geri dönüş kalmadı (`envSecret` ve `EnsureFromEnv` silindi).
- **Panel:** Ayarlar → WhatsApp / Meta (yöntem seçici, 5 adımlı rehber, kopyalanabilir adres ve belirteç, rotasyon, hesap başına test, sağlık kartı). Kanallar sayfasındaki webhook kartı buraya bağlantı oldu; `GET /channels/webhook` kaldırıldı (`GET /channels/meta-connection`).
- **WP2'ye kancalar:** `channels.Service.Connection` ilk okumada satırı oluşturur; `TODO(WP2)`: firma açılırken çağrılsın. Askıdaki firmanın anahtarlı adresi `403`.
- **Migrasyon:** 00030 (WP2'nin migrasyonlarıyla birleşirken yeniden numaralanabilir).

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
| WP3 — arka plan, realtime, Redis, önbellekler | ✅ tamamlandı (2026-11-28); zamanlanmış işler kiracı başına, başlıksız görev reddi, kiracı önekli kanallar, Redis anahtar önekleri, adillik (worker payı + kiracı kampanya kovası), kiracıya özel başarısız işler, kiracıya bağlı medya bağlantıları; migrasyon yok (kararlar §5.7) |
| WP5 — firmanın kendi Meta uygulaması | ✅ tamamlandı (2026-11-28); migrasyon 00030, firmaya özel anahtarlı webhook adresi + doğrulama belirteci (rotasyon, geçiş süresi), firmanın sırlarıyla imza, WABA eşleşmesi, bağlantı testi, sağlık, Ayarlar → WhatsApp / Meta, `META_*` kimlik değişkenleri kaldırıldı, Tech Provider anahtarı kapalı (kararlar §5.8; rehber docs/META_SETUP.md) |
| Marka kimliği (kanvas) | taslak 1 — geri bildirim bekliyor |
| Abonelik tasarımı (kanvas) | taslak — öncelik sonra |
