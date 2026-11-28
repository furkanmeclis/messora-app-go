# Güvenlik Tasarımı ve Uyumluluk

> Bu doküman tehdit modelini ve kontrolleri tanımlar. Açık bildirme (disclosure) politikası kök dizindeki `SECURITY.md` dosyasında olacak (Phase 1).

## 1. Varlıklar ve tehditler

| Varlık | Tehdit | Kontrol |
|---|---|---|
| Meta access token, app secret | DB sızıntısı, tarayıcıya sızma, log'a düşme | AES-256-GCM ile şifreli `_enc` kolonlar. API yanıtında asla dönmez, yalnızca `token_last4` döner. Log redaction uygulanır. Rotasyon: anahtar sürüm byte'ı. |
| Müşteri verisi (telefon, mesaj içeriği) | Yetkisiz erişim, log sızıntısı | RBAC kullanılır, Agent yalnızca atanmış konuşmaları görür (ayar). Mesaj gövdesi ve telefon numarası log'lara yazılmaz. Contact silme hard delete'tir. |
| Panel hesapları | Brute force, credential stuffing, oturum çalınması | Argon2id hash. E-posta+IP ve IP bazında Redis rate limit uygulanır. 10 başarısız denemede hesap 15 dk kilitlenir. TOTP 2FA, Owner/Admin için zorunlu yapılabilir (ayar). Opak token HttpOnly `__Host-` cookie'de tutulur, Go tarafında yalnızca BFF'den kabul edilir. |
| Mesaj gönderim kapasitesi | Kötüye kullanım, spam, numaranın banlanması | Opt-out kişilere gönderim sistem seviyesinde engellenir. 24h pencere zorunludur. Kalite düşüşünde (RED) kampanyalar otomatik duraklatılır. API rate limit uygulanır. |
| Webhook endpoint'i | Sahte event enjeksiyonu, firmalar arası enjeksiyon, replay | Her firmanın tahmin edilemez kendi adresi vardır. `X-Hub-Signature-256` ham body üzerinde, **yalnızca o firmanın** app secret'larıyla sabit zamanlı karşılaştırmayla doğrulanır. Verify'da o firmanın `hub.verify_token`'ı kontrol edilir. Firmada bağlı olmayan WABA'ya ait girdiler atılır. Body hash ile firma başına dedupe yapılır. Body boyutu sınırlıdır. Ayrıntı: §3a "Meta webhook'ları". |
| Sunucu iç ağı | SSRF (otomasyon HTTP aksiyonu, giden webhook) | `internal/platform/safehttp` client'ı ismi kendisi çözer ve IP kontrolünü DNS çözümlemesinden **sonra**, ayrıca bağlantı anında (dialer `Control` hook'u) tekrar yapar; DNS rebinding işe yaramaz. Loopback, link-local, RFC1918, CGNAT, metadata (169.254.169.254), IPv6 ULA, multicast, ayrılmış/dokümantasyon blokları, IPv4-mapped ve NAT64 biçimleri ile `2130706433` / `0177.0.0.1` gibi sayısal kısaltmalar reddedilir (Docker ağları RFC1918 içindedir). Varsayılan yalnızca https; redirect izlenmez (izlenirse her adım yeniden kontrol edilir); `HTTP(S)_PROXY` yok sayılır. Timeout 10 sn, yanıt boyutu sınırlıdır, hata mesajlarında URL kimlik bilgisi yer almaz. Özel adreslere izin veren `AllowPrivate` seçeneği test binary'si dışında panic eder, yapılandırmayla açılamaz. İç hedefler için açık allowlist (`Options.AllowedPrefixes`) kodda hazırdır; `OUTBOUND_HTTP_ALLOWLIST` ayarına henüz bağlanmadı. |
| API key'ler | Sızma | `msk_live_` + 32 byte rastgele değer kullanılır, SHA-256 hash olarak saklanır. Değer tek seferlik gösterilir. Scope'lar ve opsiyonel bitiş tarihi desteklenir. Yalnızca `Authorization` header'ı ile kabul edilir, query parametresi reddedilir. `last_used_at` takip edilir. |
| Dosya yükleme | Zararlı dosya, zip bomb, CSV injection | Boyut sınırı (import 50 MB, medya Meta limitleri), uzantı ve magic-byte kontrolü yapılır. XLSX streaming okunur, sıkıştırılmış boyut ve oran sınırı uygulanır. CSV export'ta `= + - @` ile başlayan hücreler `'` ile kaçırılır (formula injection). Medya, Content-Type'ı zorlanmış ve `Content-Disposition: attachment` başlıklı yetkili bir endpoint'ten servis edilir. |
| Yetki yükseltme | IDOR | Her ID girdisi (grup, tag, template, numara) servis katmanında var olma ve izin açısından doğrulanır. Agent kapsamı (atanmış konuşmalar) sorgu seviyesinde uygulanır. |
| Tedarik zinciri | Kötü niyetli bağımlılık | Bağımlılıklar sabitlenir ve lockfile CI'da doğrulanır (`npm ci` zorunlu). `govulncheck` ve `npm audit` CI'da çalışır. Docker imajları digest ile sabitlenir. Dependabot/Renovate kullanılır. |

## 2. Kimlik doğrulama

- **Parola:** Argon2id (`golang.org/x/crypto/argon2`). OWASP önerisi olarak m=19 MiB, t=2, p=1 alt sınır alınır, açılışta kalibre edilebilir. Parametreler PHC string içinde saklanır, giriş sırasında parametreler eskiyse hash yeniden üretilir. En az 12 karakter, yaygın parola listesi kontrolü yapılır. HIBP k-anonymity kontrolü opsiyoneldir ve varsayılan kapalıdır (dış servise veri gönderdiği için).
- **Oturum:** 32 byte token kullanılır, yalnızca `sha256` saklanır. İdle timeout 12 sa, mutlak süre 30 gün (ayarlanabilir). Parola değişince diğer tüm oturumlar iptal edilir. Kullanıcı aktif oturumlarını görüp sonlandırabilir.
- **2FA:** TOTP (RFC 6238, ±1 adım), 10 adet tek kullanımlık recovery kodu (hash'li). Hassas işlemlerde (API key oluşturma, Meta token değiştirme, rol değiştirme, kullanıcı silme) 10 dakika içinde yapılmış yeniden kimlik doğrulama gerekir.
- **Hesap ve üyelik (Phase 17 WP2):** hesap kişiye aittir (e-posta platform genelinde tekil, parola, TOTP, kurtarma kodları, kilitlenme). Firmadaki rol ve durum üyeliktedir (`tenant_memberships`). Giriş, isteğin geldiği firmadaki etkin üyeliği ister; üyesi olmayan hesap yanlış parola gibi `401` alır. Parola değişince, sıfırlanınca, e-posta değişince ya da hesap devre dışı kalınca **bütün firmalardaki** oturumlar kapanır.
- **İlk kurulum (platform):**
  - Platform yöneticisi yokken `messora migrate` tek seferlik, 24 saat geçerli bir açılış bağlantısı yazar (`https://admin.<base>/bootstrap?token=msb_…`). Token'ın yalnızca hash'i `platform_settings`'te saklanır, her `migrate` çalışmasında yenilenir, ilk yönetici oluşunca silinir. Alternatif: `docker compose exec messora-api messora admin create --email … --password-stdin`.
  - Firma sahipleri platform panelinden **davetle** ya da **self sign-up** ile gelir (`signup_enabled`). Kayıt olan hesap e-posta adresini doğrulamadan giriş yapamaz (`403 EMAIL_NOT_VERIFIED`); doğrulama bağlantısı 24 saat, tek kullanımlıktır; yeniden gönderme adres başına dakikada bir. Kayıt IP başına 10 dakikada 5 kez.
  - Production'da demo hesabı ve varsayılan parola **hiçbir koşulda** oluşturulmaz. Demo seed'i yalnızca `APP_ENV=development` iken ve açık bir komutla çalışır.
- **Parola sıfırlama:** Tek kullanımlık, 30 dakika geçerli token; bağlantı isteğin geldiği firmanın adresindedir ve yalnızca o firmanın üyesine gider. Yanıt, e-postanın kayıtlı olup olmadığını belli etmez. SMTP yapılandırılmamışsa özellik kapalıdır. Platform yöneticisi bir hesap için "parolayı sıfırlamaya zorla" diyebilir: oturumlar kapanır, giriş `403 PASSWORD_RESET_REQUIRED` alır, sıfırlama bağlantısı e-postayla gider.
- **Firma değiştirme:** oturum tek firmaya bağlıdır. Başka bir firmaya geçiş, hedef firmadaki üyelik doğrulanınca verilen tek kullanımlık, 60 saniyelik bir token'la (`/auth/handoff?token=`) hedef adreste yeni oturum açar; token başka adreste denenirse de tükenir.
- **Yeni cihaz bildirimi:** kişinin o firmada daha önce görülmemiş bir IP'den girişi `login_alert` e-postası gönderir (ilk giriş hariç).

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

Her firma bir **kiracıdır** (`tenants`). Firmanın verisi hiçbir yoldan başka bir firmaya görünmemelidir; kontrol katmanları dıştan içe (WP2 eklemeleri aşağıda, "Platform paneli"):

1. **Alan adı → kiracı.** API, isteğin alan adını `tenant_domains` tablosunda arar (`internal/platform/tenant`). BFF, Go'ya iç ağ adresinden bağlandığı için tarayıcının açtığı alan adını `X-Messora-Tenant-Host` başlığıyla iletir; bu başlığa **yalnızca** BFF sırrı (`X-Messora-BFF`) da doğruysa güvenilir, aksi hâlde `Host` kullanılır (Caddy `Host`'u değiştirmeden iletir). `X-Forwarded-Host` kullanılmaz. Bilinmeyen alan adı `404 TENANT_NOT_FOUND`, askıdaki kiracı `403 TENANT_SUSPENDED` alır; doğrulanmamış özel alan adları bilinmeyen sayılır. Sonuç 15 sn önbelleklenir (bilinmeyenler 5 sn), değişiklik yapılan replikada hemen geçersizleşir.
2. **Kimlik bilgisi kiracıya bağlı.** Oturumlar ve API anahtarları bir kiracıya aittir (`tenant_id`); hesaplar global, roller üyeliktedir (WP2). Arama zaten alan adının kiracısıyla yapılır (RLS); API guard ayrıca `principal.TenantID` ile alan adının kiracısını karşılaştırır ve farklıysa bilinmeyen token gibi `401` döner. A'nın anahtarı B'nin adresinde çalışmaz.
3. **Havuz kancası (fail-closed).** Uygulama `messora_app` rolüyle bağlanır. Havuzdan her bağlantı alınırken `set_config('app.tenant_id', <ctx kiracısı>, false)` çalışır, bırakılırken `RESET app.tenant_id`; sıfırlama başarısızsa bağlantı atılır. Bağlamda kiracı da `tenant.System` işareti de yoksa bağlantı **hiç alınmaz** (`tenant.ErrNoTenant`). `pool.Begin` aynı kancadan geçer.
4. **Row-level security.** Kiracıya ait 57 tablonun hepsinde RLS açık ve `FORCE` edilmiştir; `tenant_isolation` politikası okuma ve yazmada `tenant_id = app_tenant_id()` ister. `app_tenant_id()` ayar yoksa `NULL` döner (hiçbir satır görünmez, hiçbir satır yazılamaz); bozuk değer hata verir, erişimi genişletmez. `tenant_id` sütununun varsayılanı bağlantının kiracısıdır, `WITH CHECK` başka bir değeri reddeder.
5. **Bileşik yabancı anahtarlar.** Çocuk tablolar ebeveyne `(tenant_id, id)` ile bağlanır (23 ilişki: kişi ↔ mesaj/konuşma/alıcı, kampanya ↔ alıcı, otomasyon ↔ çalıştırma, numara ↔ hesap, etiket/grup üyelikleri, oturum ↔ kullanıcı …). RLS'i atlayan bir rol bile başka kiracının satırına işaret eden satır yazamaz.

**Roller.**

| Rol | Kim kullanır | Yetki |
|---|---|---|
| `messora` (sahip) | yalnızca `messora migrate` | tabloların sahibi; şemayı değiştirir, rol parolalarını ayarlar |
| `messora_app` | API, worker, scheduler | `NOBYPASSRLS`, hiçbir tablonun sahibi değil; kiracı tablolarında DML (RLS ile sınırlı), `tenants`/`tenant_domains`/`platform_settings`'te yalnızca `SELECT` |
| `messora_system` | outbox aktarıcısı ve temizliği (worker) | `BYPASSRLS`; havuzu 4 bağlantılık, istekten gelen hiçbir yol kullanmaz |
| `messora_platform` | platform işi: kayıt, platform paneli (API) | `NOLOGIN`, `NOBYPASSRLS`; `messora_app` yalnızca `SET ROLE` ile geçer, yetkileri **miras almaz**. Kayıt tabloları, katalog (`meta_rates`, `ai_model_prices`) ve platform tablolarında DML; kiracılar arası yalnızca `tenant_memberships`, `user_sessions`, `support_access_grants`'ta açık `platform_access` politikası |

Parolalar `messora init` tarafından üretilir (`POSTGRES_APP_PASSWORD`, `POSTGRES_SYSTEM_PASSWORD`) ve `messora migrate` her çalıştığında role yazılır. API'nin sahip parolasına ihtiyacı yoktur.

**Önbellekler.** Ayarlar, SMTP göndericisi, onay ayarları ve kampanya analitiği ayarları kiracı başına önbelleklenir (`tenant.Cache`); kiracısız iş hiç önbelleğe yazılmaz. Numaraların gönderim tavanı (kiracı, numara) çiftiyle önbelleklenir. Paneldeki görünüm önbelleği alan adı başınadır. Alan adı çözümü ve adillik ayarları platform genelidir. `APP_*` ortam ayarları yalnızca 1 numaralı kiracıya uygulanır.

**Redis anahtarları (WP2).** Kişiye ait olanlar global (`tenant.GlobalRedisKey`): TOTP tekrar koruması `messora:totp:used:<user>:<adım>`, kimlik hız sınırları `messora:rl:login:*`, `rl:reset:*`, `rl:reauth:*`, `rl:password:*`, `rl:signup:*`, `rl:resend:*` (kilitlenme veritabanında) ve firma değiştirme token'ı `messora:handoff:<hash>` (60 sn, `GETDEL`, içinde hedef firma). Oturuma ait olanlar kapsamına bağlı: firma girişinin MFA denemesi `messora:t:{<tenant>}:mfa:<hash>`, platform panelininki `messora:mfa:platform:<hash>` (başka yerde bulunmaz); e-posta değişikliği onayı `messora:t:{<tenant>}:email-change:<hash>`; API bütçesi firmada `messora:t:{<tenant>}:rl:api:…`, platform oturumu için `messora:rl:api:platform:…`.

**Arka plan (WP3).** `outbox.Write` olayı yazanın kiracısıyla kaydeder; aktarıcı kiracıyı asynq görevinin başlığına (`messora-tenant`) koyar ve worker her işleyiciyi o kiracı olarak çalıştırır. Zamanlanmış işler kiracı başına dağıtılır: zamanlayıcı dilimi tek bir platform görevi olarak kuyruğa alır, worker aktif kiracıları sistem havuzundan okuyup işi her biri için başlıkla kuyruğa alır (askıdaki ve silinen kiracılar hariç). **Başlıksız görev reddedilir** (çalıştırılmadan ölü mektuba): yalnızca iki platform görevi (`platform:tenant_fanout`, `maintenance:outbox_cleanup`) başlıksız, `tenant.System` olarak çalışır. Tek kiracı bir worker'ın eşzamanlılığının en fazla yarısını tutar, kampanyaları birlikte en fazla 200 mesaj/sn gönderir (platform ayarı, ARCHITECTURE.md §10.1). Başarısız işler sayfası yalnızca isteğin kiracısının görevlerini gösterir; tekrar deneme ve silme de öyle.

**Realtime (WP3).** Her Centrifugo kanalı kiracı önekini taşır (`tenant:<tenant_id>:inbox:all`). Yayıncı öneki outbox satırının kiracısından alır; bağlantı ve abonelik token'ları yalnızca oturumun kiracısının kanallarını verir (istek alan adının kiracısı da aynı olmalı); kullanıcıya sınırlı kanal yoktur, kanal adı tek başına erişim açmaz. Centrifugo kullanıcısı `<tenant>:<user>` olduğundan bir kiracıda alınan abonelik token'ı başka kiracının bağlantısında geçmez. `/realtime/sync` aynı kuralla çalışır.

**Redis (WP3).** Kiracı işinin anahtarları `messora:t:{<tenant_id>}:…` önekindedir (API hız sınırı oturum/anahtar başına, gönderim zamanlayıcısı, AIMD). Kişi geneli anahtarlar bilerek globaldir: e-posta başına giriş kilidi, 2FA challenge'ı, TOTP tekrar koruması; anonim istek sınırı IP başınadır (ARCHITECTURE.md §10.2). Advisory lock anahtarları kiracıyı içerir.

**Medya (WP3).** Yerel sürücünün imzalı bağlantıları (`/media/s/`) nesnenin kiracısını imzaya katar ve yalnızca o kiracının alan adlarında açılır: `/media/s/` kiracı ara katmanının arkasındadır, isteğin kiracısı anahtarınkiyle uyuşmazsa `403`. Bağlantı onu üreten isteğin alan adına işaret eder. Kiracı bağlamı başka kiracının nesnesini imzalayamaz. `/media/p/` (logolar) alan adından bağımsızdır.

**Meta webhook'ları (WP5).** Her firma kendi Meta uygulamasını bağlar; platform genelinde Meta uygulaması, app secret'ı, token'ı ya da doğrulama belirteci yoktur (`META_*` kimlik değişkenleri yok sayılır).

**Platform paneli (WP2).** `admin.<base>` host'u platform işi olarak çalışır: havuzdan alınan bağlantı `SET ROLE messora_platform` yapar, bırakılırken `RESET ROLE; RESET app.tenant_id` (başarısızsa bağlantı atılır). Platform oturumu (`msp_`, `platform_sessions`) yalnızca BFF sırrıyla ve yalnızca admin host'unda geçerlidir; firma oturumu (`mss_`) admin host'unda, platform oturumu firma adresinde `401` alır. OpenAPI uzantısı `x-messora-host` her işlemin hangi tür host'ta çalıştığını söyler; yanlış host'ta işlem yokmuş gibi `404`. `/platform/*` işlemleri yalnızca `platform` şemasını kabul eder, public API dokümanına hiçbir zaman girmez. Platform yöneticileri için TOTP zorunludur (kurulmadan yalnızca kayıt işlemleri açık; firma panelinden kapatılamaz); oturum 12 saat, boşta 2 saat. Her platform işlemi `platform_audit_logs`'a, firmayı etkileyenler ayrıca firmanın `audit_logs`'una yazılır.

**Destek erişimi (WP2).** Platform yöneticisi bir firmanın panelini gerekçe ve süreyle (5–240 dk) açar; varsayılan **salt okunur**, yazma açıkça seçilir. İzin firmanın kendi satırıdır (`support_access_grants`, RLS; firma `GET /support-access` ile görür) ve firmanın denetim kaydına yazılır. Oturum hand-over ile firmanın adresinde açılır, izin bitince ya da sonlandırılınca hemen geçersiz olur. Salt okunur oturum GET dışında hiçbir şey yapamaz (`403 SUPPORT_READ_ONLY`); yazma iznindeki oturum da yeniden doğrulama yapamaz (API anahtarı, 2FA, rol değişikliği gibi hassas işlemler kapalı), kişinin hesabını değiştiremez, üye listesinde görünmez. Firma panelinde süre boyunca banner görünür.

- **Adres:** `https://hooks.<TENANT_BASE_DOMAIN>/meta/<anahtar>`. Anahtar 32 rastgele bayttır (küçük harf base32, 52 karakter), firmaya aittir ve panelde gösterilir; bu yüzden hash'lenmez, `Sealer` ile şifreli saklanır (`meta_connections.webhook_key_enc`). Aramada yalnızca SHA-256'sı kullanılır: platform yönlendirme tablosu `meta_webhook_keys` (hash → kiracı) RLS'siz bir platform tablosudur, sır içermez.
- **Kiracı önce çözülür:** istek gelince anahtarın biçimi kontrol edilir, sonra hash'i yönlendirme tablosunda aranır (uygulama havuzunda `tenant.System` ile tek satırlık sorgu); kiracıya ait hiçbir tablo, kiracı bağlamı kurulmadan okunmaz. Sonrası o kiracının bağlamında ve RLS altında yürür. **Neden SECURITY DEFINER arama fonksiyonu değil:** `FORCE ROW LEVEL SECURITY` tablo sahibine de uygulandığı için sahibin fonksiyonu kiracı tablosunu ayarsız okuyamazdı; `messora_system` (BYPASSRLS) ise API'de bilerek yok (WP1). `tenant_domains` ile aynı desen: yönlendirme için platform tablosu, `messora_app` yalnızca okur. **Yazma** yalnızca `meta_webhook_key_rotate()` ile olur: sahip olarak çalışır ama satırı yalnızca çağıran bağlantının `app.tenant_id`'sine yazar; bir firma anahtarını başka firmaya yönlendiremez, kiracısız bağlantı hiçbir şey yazamaz.
- **Bilinmeyen anahtar** `404` alır; biçimi bozuk anahtar veritabanına sorulmadan `404`. Zamanlama sızıntısı yoktur: karşılaştırma anahtarın kendisiyle değil SHA-256'sıyla, B-tree indeksinde yapılır; bir saldırgan yanıt süresinden anahtarın hiçbir parçasını öğrenemez. Askıdaki firmanın adresi `403`, silinmiş firmanınki `404`.
- **GET doğrulaması** o firmanın doğrulama belirtecini sabit zamanlı karşılaştırır (`meta_connections.verify_token_enc`, firma başına üretilir, yenilenebilir). Bağlantı satırı yoksa hiçbir belirteç eşleşmez.
- **POST imzası** yalnızca o firmanın aktif hesaplarının app secret'larıyla (`whatsapp_accounts.app_secret_enc`, RLS) sabit zamanlı doğrulanır. İmza yok/yanlış → `401`, hiçbir şey saklanmaz, sayaç artar (`signature_failures`). A firmasının sırrı B'nin adresinde geçmez. Hesap bağlarken App ID ve App secret zorunludur.
- **WABA eşleşmesi:** imzası doğru gövdedeki her `entry[].id`, o firmada bağlı bir WhatsApp Business Account olmalıdır; olmayan girdiler atılır, loglanır ve sayılır (`waba_mismatches`, son WABA kimliği panelde görünür). Böylece bir firmanın uygulaması başka bir firmanın WABA'sı adına olay enjekte edemez. Yanıt `200`'dür (Meta'nın yeniden denemesi sonucu değiştirmez); kalan girdiler saklanır. `messages` olaylarında numara da RLS altında aranır.
- **Rotasyon:** yeni anahtar üretilir; eski anahtar seçilen süre (0–168 saat) boyunca da yanıt verir, sonra `404`. Bir firmanın en fazla bir eski anahtarı olur. Doğrulama belirteci yenilenince eskisi hemen geçersizdir. İkisi de denetim kaydına yazılır.
- **Geriye uyumluluk:** firmanın kendi alan adındaki `/webhooks/whatsapp` ve `/webhooks/meta` aynı kurallarla (o alan adının firmasının belirteci ve sırları, WABA eşleşmesi) çalışır; kiracıyı alan adı ara katmanı çözer.
- **Dedupe** `(tenant_id, payload_sha256)` tekilidir (WP1): aynı gövde iki firmada ayrı sayılır.
- **Giden çağrılar** her zaman hesabın kendi token'ıyla yapılır (`channels.accountToken`); global geri dönüş yoktur.
- **Tech Provider (WP6):** `platform_settings.tech_provider.enabled=false`. Açıldığında tek uç nokta olacak; olaylar yalnızca `meta_routes` (WABA/numara → kiracı) üzerinden yönlendirilecek, eşleşmeyen reddedilecek.

**CI kontrolü.** `TestEveryTableIsTenantScoped` (`internal/platform/database`) `pg_catalog`'u okur: platform listesi dışındaki her tabloda `NOT NULL tenant_id`, açık ve zorlanmış RLS ve iki yönlü `tenant_isolation` politikası yoksa test kırılır. `TestTenantIsolationMatrix` (`internal/app`) iki kiracıyla bütün liste/getir uçlarını, değiştirme denemelerini ve çapraz kimlik bilgilerini dener (WP5'ten beri Meta bağlantısı, bağlantı testi ve webhook anahtarı da). `TestKeyedWebhookPerTenant` ve `TestWebhookKeyRotation` webhook yalıtımını dener.

**Bilinen açıklar (sonraki paketler).** Özel alan adlarının TLS'i (on-demand `ask`) WP7'de doğrulanmış alan adlarına bağlanacak. `custom_smtp` gibi plan yetkileri WP8'de; firma SMTP'si şimdilik yalnızca platform göndericisi (`SMTP_HOST`) yokken kullanılır. Webhook adreslerinde istek başına hız sınırı yoktur (imzasız istekler de bir sayaç yazar); hooks alan adının TLS'i WP7'de. Platform tabloları (`meta_rates`, `ai_model_prices`) firma panelinden yazılabiliyor; platform paneline taşınacak (WP2). `APP_*` çalışma alanı ortam değişkenleri bütün kiracılara uygulanır (WP2'de platform ayarlarına taşınacak). Kullanıcıların e-posta adresi şimdilik bütün kiracılarda tekildir (WP2: global kullanıcı + üyelik).

## 4. Audit log

Yazılan event'ler:
- login başarılı/başarısız, logout, 2FA aç/kapat, parola değişikliği ve sıfırlama, e-posta değişikliği
- kayıt (firma açılışı), e-posta doğrulama; platform işlemleri (`platform_audit_logs`): firma oluşturma, askıya alma/açma, deneme değişikliği, alan adı ekleme/doğrulama/silme, hesap kapatma/açma, zorunlu sıfırlama, destek izni açma/kapatma, platform ayarları, fiyat kartı ve AI fiyatları
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
