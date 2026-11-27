# Phase 17 — Çok kiracılı SaaS (Messora Cloud)

Durum: **kapsam taslağı** (2026-11-27). Uygulamaya başlarken, 16C'de yaptığımız gibi önce teknik analiz yapılır: mevcut kod taranır, riskler ve iş paketleri bu belgeye eklenir, ürün sahibinin onayıyla başlanır. Tasarım kanvası: https://claude.ai/artifact/5gT44f3kyVA6ePXMGMidCX.

## 1. Kararlar

### 1.1 Ürün yönü
- **Messora yalnızca çok kiracılı SaaS olarak çalışır.** Lisanslı self-hosted mod kapatılır: lisans doğrulama, kısıtlı mod, lisans registry'si, `messora-license` CLI ve müşteri kurulum dosyası devre dışı kalır ya da kaldırılır. Hangisinin silinip hangisinin pasif bırakılacağı teknik analizde netleşir. `MESSORA_MODE` anahtarı şimdilik yok.
- **Veri izolasyonu:** tek PostgreSQL, her satırda `tenant_id`, satır bazlı güvenlik (RLS). İstek bağlamı, kuyruk işleri, Centrifugo kanalları, Redis anahtarları, medya depolaması ve API anahtarları kiracıya göre ayrılır.
- **Adresleme:** `firma.messora.tech` alt alan adları (wildcard DNS hazır) ve isteğe bağlı özel alan adı, otomatik TLS ile.
- **Firma oluşturma:** ilk etapta platform yönetici panelinden. Self sign-up kanvastaki akışla sonra açılır.

### 1.2 Kaldırılanlar
- **Firma bazında tema motoru kaldırılır.** Tüm firmalar Messora'nın tek, sabit görünümünü kullanır. Firma logosu yalnızca giriş ekranında ve e-postalarda gösterilir. Bu kararın netleşmesi bekleniyor; yalnızca logo mu kalacak, o da mı kalkacak?
- **PWA (uygulama olarak yükleme) kaldırılır:** manifest, service worker ve ayarlardaki PWA sekmesi.
- **Masaüstü modu:** karar verilmedi. Teknik analizde sorulacak.

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

## 5. Kabul kriterleri (taslak)
1. İki firmanın verisi hiçbir API, olay, dışa aktarma, kuyruk işi ya da realtime kanalında birbirine görünmez. RLS ve uygulama katmanı testlerle doğrulanır; bir firmanın anahtarıyla diğerinin adresine yapılan istek reddedilir.
2. Her firma kendi adresinden API'ye erişir, kendi webhook'larını alır; firmaya özel gelen Meta webhook'u doğru firmaya işlenir.
3. Tech Provider kapalıyken hiçbir arayüzde görünmez. Açılınca Embedded Signup akışı mock Meta ile uçtan uca çalışır.
4. Tema ve PWA kodu ile ayarları kaldırılmıştır; panel tek temayla çalışır.
5. Lisans modu kapalıdır; lisans olmadan da hiçbir özellik kısıtlı moda düşmez.
6. Firma oluşturma, askıya alma ve alan adı ekleme platform panelinden yapılır ve denetim kaydına yazılır.

## 6. Durum

| İş | Durum |
|---|---|
| Teknik analiz ve iş paketleri | bekliyor (onaylı başlangıç) |
| Marka kimliği (kanvas) | taslak 1 — geri bildirim bekliyor |
| Abonelik tasarımı (kanvas) | taslak — öncelik sonra |
