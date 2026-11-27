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
| — | Abonelik, plan ve kredi | L | WP2, WP4 — sonra |

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
| Marka kimliği (kanvas) | taslak 1 — geri bildirim bekliyor |
| Abonelik tasarımı (kanvas) | taslak — öncelik sonra |
