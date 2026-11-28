# Messora'yı Meta Tech Provider olarak açma (platform yöneticisi)

Bu belge platform yöneticisi içindir: Messora'nın Meta'ya **Tech Provider** olarak başvurması, onaydan sonra gereken ayarlar ve özelliğin açılması. Firmalar için rehber: [META_SETUP.md](META_SETUP.md) "Messora üzerinden bağlanma". Tasarım: [PHASE-17.md](PHASE-17.md) §3.2, §5.9; güvenlik: [SECURITY.md](SECURITY.md) "Tech Provider webhook'u".

**Durum:** kod ve testler hazır, özellik **kapalı** (`platform_settings.tech_provider.enabled = false`). Kapalıyken firma panelinde "Meta ile bağlan" kartı "Yakında" olarak devre dışıdır, Tech Provider uç noktaları ve `/meta/tp` webhook adresi `404` döner.

## 1. Meta'da hazırlık

Messora'nın kendi işletme hesabında (Meta Business Portfolio) yapılır.

1. **İşletme doğrulaması:** Business Settings → Security Center → **Start verification**. Tech Provider olmak ve App Review için zorunludur (resmî şirket belgeleri, alan adı doğrulaması `messora.tech`).
2. **Uygulama:** [developers.facebook.com/apps](https://developers.facebook.com/apps) → **Create app** → türü **Business**, Messora'nın portföyüne bağlı. **Add product → WhatsApp**.
3. **Uygulama ayarları (App settings → Basic):** gizlilik politikası ve hizmet koşulları adresleri, uygulama simgesi, kategori, **App domains: `messora.tech`**. App ID ve App secret'ı not edin.
4. **Facebook Login for Business** ürününü ekleyin → Settings:
   - **Allowed domains for the JavaScript SDK:** firmaların panel adresleri. Alt alan adları için `messora.tech` ve kullanılan özel alan adları (Meta joker kabul etmiyorsa her firma alan adı ayrı eklenir; bkz. "Riskler").
   - **Valid OAuth Redirect URIs:** aynı alan adları (`https://<firma>.messora.tech/`). Embedded Signup kodu sunucuda `redirect_uri` olmadan takas edilir, ama Meta alan adının listede olmasını ister.
   - HTTPS zorunlu.
5. **Tech Provider başvurusu:** App Dashboard → WhatsApp → **Become a Tech Provider** (ya da Partner Center üzerinden) adımlarını izleyin; Meta işletme doğrulamasını, uygulama incelemesini ve erişim doğrulamasını (access verification) ister.

## 2. App Review: istenecek izinler

**Advanced access** istenir; her biri için ekran kaydı ve kullanım açıklaması gerekir (Embedded Signup akışını ve paneldeki kullanımı gösteren bir video yeterlidir).

| İzin | Neden |
|---|---|
| `whatsapp_business_management` | Firmanın WABA'sını yönetmek: numaralar, şablonlar, `subscribed_apps`, numara kaydı, sağlık sinyalleri. |
| `whatsapp_business_messaging` | Firmanın numarasından mesaj göndermek ve almak. |
| `business_management` | Embedded Signup'ta firmanın seçtiği işletme portföyünü okumak (yalnızca paylaştığı varlıklar). |

İnceleme sırasında Meta'nın test kullanıcısı akışı gerçekten çalıştırabilmelidir. Anahtar platform genelidir (bir firmada açılıp diğerlerinde kapalı kalamaz); bu yüzden inceleme için ayrı bir hazırlık ortamı (staging) kullanın ve özelliği orada açın.

## 3. Embedded Signup yapılandırması (configuration ID)

1. App Dashboard → **Facebook Login for Business → Configurations → Create configuration**.
2. **Login variation:** WhatsApp Embedded Signup.
3. **Access token:** Meta'nın sunduğu "business integration system user" token'ı; süre seçeneği varsa **Never**.
4. **Assets:** WhatsApp accounts. **Permissions:** `whatsapp_business_management`, `whatsapp_business_messaging` (ve `business_management`).
5. Oluşan **Configuration ID**'yi not edin.
6. Embedded Signup sürümü: Meta v2/v3'ü 15 Ekim 2026'da kaldırdı; yapılandırma **v4** olmalı. Panel `FB.login` çağrısında `extras: { setup: {} }` gönderir (v4 biçimi).

## 4. Webhook

1. App Dashboard → **WhatsApp → Configuration → Webhook**:
   - **Callback URL:** `https://hooks.messora.tech/meta/tp` (alt alan adı TLS'i yoksa `https://<PUBLIC_APP_URL>/webhooks/meta/tp`).
   - **Verify token:** `messora tech-provider show` çıktısındaki değer (aşağıda).
   - **Verify and save**, sonra alanlara abone olun: en az `messages`; ayrıca `message_template_status_update`, `message_template_quality_update`, `phone_number_quality_update`, `account_update`, `business_capability_update`, `user_preferences`.
2. Firmaların WABA'ları bu uygulamaya `subscribed_apps` ile Messora tarafından abone edilir; bütün olaylar bu tek adrese gelir ve WABA kimliğiyle firmaya yönlendirilir.

## 5. System user token

Messora'nın kendi işletmesinde: Business Settings → System users → **Add** (Admin) → uygulamayı atayın → **Generate new token**, **Never**, izinler: `whatsapp_business_management`, `whatsapp_business_messaging`, `business_management`. Bu token yapılandırma testinde (`debug_token`) ve Messora'nın kendi varlıklarını yönetmekte kullanılır; firmaların işleri her zaman onların Embedded Signup'ta verdiği business token'la yapılır.

## 6. Messora'ya girme ve açma

Platform yönetici paneli (admin.<domain>, WP2) bu ayarların formunu alacak (`GET/PATCH /api/v1/platform/meta/tech-provider`, `POST …/test`). O gelene kadar komut satırı kullanılır; komut `messora migrate` gibi veritabanı sahibi olarak yazar. Sırlar ortamdan okunur, komut satırına yazılmaz:

```sh
# Değerleri girin (henüz kapalı):
docker compose exec -T \
  -e MESSORA_TP_APP_SECRET='<App secret>' \
  -e MESSORA_TP_SYSTEM_USER_TOKEN='<system user token>' \
  messora-api messora tech-provider set \
  -app-id <App ID> -configuration-id <Configuration ID>
# İsteğe bağlı: -graph-api-version v26.0 (boş: META_GRAPH_API_VERSION)

# Callback URL ve verify token'ı görün; Meta'ya girin (4. adım):
docker compose exec messora-api messora tech-provider show

# Meta ile test edin: app kimlik bilgileri, system user token'ı ve izinleri,
# yapılandırma kimliği, uygulamanın webhook aboneliği (adres /meta/tp olmalı):
docker compose exec messora-api messora tech-provider test

# Açın (bütün firmalarda "Meta ile bağlan" görünür):
docker compose exec messora-api messora tech-provider set -enable

# Kapatın (yeni bağlantılar durur, /meta/tp 404 olur; bağlı hesapların
# olayları da kabul edilmez):
docker compose exec messora-api messora tech-provider set -disable
```

Diğer seçenekler: `-rotate-verify-token` (yeni doğrulama belirteci; Meta'da da değiştirip yeniden doğrulayın), `-app-secret-stdin` (App secret'ı standart girdiden okur). Açmak için App ID, App secret ve configuration ID gerekir; eksikse komut reddeder.

Dokploy/AWS: aynı komut uygulama konteynerinde çalıştırılır (Dokploy → servis → Terminal ya da `docker exec`).

## 7. Açtıktan sonra kontrol listesi (gerçek uçtan uca test)

Sahte Meta ile yazılmış testler (`internal/app/tech_provider_integration_test.go`, `frontend/e2e/tech-provider.spec.ts`) bütün adımları kapsar; onaydan sonra gerçek Meta ile bir kez:

1. `messora tech-provider test` → bütün satırlar OK.
2. Bir test firmasında Ayarlar → WhatsApp / Meta → **Meta ile bağlan**; Meta penceresinde test işletmesi, WABA ve numara; PIN Messora'ya bırakılsın.
3. Başarı kartında PIN bir kez görünür; Kanallar'da hesap "aktif".
4. Başka bir telefondan numaraya yazın → gelen kutusunda görünür (webhook `/meta/tp` → doğru firma).
5. Yanıt gönderin → teslim/okundu durumları gelir.
6. **Bağlantıyı kes** → WhatsApp Manager'da uygulama listeden kalkar; yeni mesaj Messora'ya düşmez.
7. Firma Meta'da ödeme yöntemi eklemelidir (WhatsApp Manager → Overview → Add payment method); yoksa ücretli şablonlar gönderilemez. Kredi hattı (Messora'nın ödemesi) Solution Partner gerektirir, kapsam dışı.

## Riskler ve bilinen sınırlar

- **JS SDK alan adları:** Facebook Login for Business "Allowed domains" joker alt alan adını kabul etmezse her firma alan adının eklenmesi gerekir; çok firmalı kurulumda bu bir operasyon yükü. Alternatif: Embedded Signup'ı tek bir adresten (ör. `app.messora.tech/connect-meta`) açıp sonucu firmaya aktarmak (sonraki iş).
- **Business token ömrü:** Embedded Signup'ın business integration token'ı Meta'nın politikasına göre süreli olabilir; token sağlığı (`debug_token`) her gün kontrol edilir, süresi yaklaşınca sahipler uyarılır ve firma "Meta ile yeniden bağlan" der.
- **Coexistence** (WhatsApp Business uygulamasında kalan numaralar, `FINISH_WHATSAPP_BUSINESS_APP_ONBOARDING`) desteklenmiyor; panel açıklar.
- **Sayaç:** bilinmeyen WABA'ya gelen olaylar süreç başına sayılır (`unrouted_entries`, platform API'si) ve loglanır; kalıcı değil.
- Anahtar platform genelidir; plan ya da firma bazında yöntem izni WP8'le gelecek.
