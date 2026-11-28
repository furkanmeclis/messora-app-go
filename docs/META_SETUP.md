# Kendi Meta uygulamanızı bağlama

Bu rehber, firmanızın WhatsApp Business hesabını **kendi Meta uygulamanız** üzerinden Messora'ya bağlamanız içindir. Messora yalnızca sizin bilgilerinizi kullanır: Meta'nın webhook'ları size özel bir adrese gelir ve sizin uygulama gizli anahtarınızla (App secret) doğrulanır. Başka hiçbir firma bu adresi ya da bilgileri kullanamaz.

Panelde: **Ayarlar → WhatsApp / Meta**. Sayfadaki adımlar bu rehberle aynı sıradadır.

> İkinci yöntem olan **Messora üzerinden bağlanma** ("Meta ile bağlan", uygulama oluşturmadan) şimdilik kapalıdır; Meta, Messora'yı Tech Provider olarak onayladığında platform yöneticisi açacak. Açıldığında nasıl kullanılacağı en sonda: [Messora üzerinden bağlanma](#messora-üzerinden-bağlanma). O zamana kadar aşağıdaki yöntem kullanılır.

## Gerekenler

- Meta Business hesabı (business.facebook.com) ve içinde bir **WhatsApp Business Account (WABA)**, en az bir telefon numarasıyla.
- Business hesabında yönetici yetkisi.
- Yaklaşık 15 dakika.

## 1. Meta uygulaması oluşturun

1. [developers.facebook.com/apps](https://developers.facebook.com/apps) → **Create app**.
2. Kullanım türü olarak işletme (**Business**) seçin, uygulamayı Business hesabınıza bağlayın.
3. Uygulama panelinde **Add product → WhatsApp → Set up**. WhatsApp'ı Business hesabınızdaki WABA ile ilişkilendirin.
4. **App settings → Basic** sayfasından şunları not edin:
   - **App ID** (yalnızca rakam)
   - **App secret** (Show ile görünür). Bu, webhook imzalarını doğrulayan sırdır; kimseyle paylaşmayın.

## 2. Süresi dolmayan bir System User token'ı oluşturun

Uygulama panelindeki geçici token'lar birkaç saat ya da bir gün içinde biter; o zaman gönderim durur. Kalıcı bağlantı için **System User** token'ı kullanın.

1. [Business Settings → Users → System users](https://business.facebook.com/settings/system-users) → **Add**; rolü **Admin** olan bir sistem kullanıcısı ekleyin.
2. **Assign assets**:
   - **Apps:** 1. adımdaki uygulamanız (Full control).
   - **WhatsApp accounts:** WABA'nız (Full control).
3. **Generate new token**:
   - uygulama: 1. adımdaki uygulama,
   - **Token expiration: Never**,
   - izinler: `whatsapp_business_messaging` ve `whatsapp_business_management`.
4. Token'ı kopyalayın (bir daha gösterilmez).

**WABA ID'nizi** WhatsApp Manager'da ya da Business Settings → Accounts → WhatsApp accounts altında bulursunuz (yalnızca rakam).

## 3. Bilgileri Messora'ya girin

**Ayarlar → WhatsApp / Meta → Hesap bağla**:

| Alan | Nereden |
|---|---|
| Ad | Panelde görmek istediğiniz ad (ör. "Ana marka") |
| WhatsApp Business Hesap ID | 2. adımın sonu |
| Erişim belirteci | 2. adımdaki System User token'ı |
| Uygulama ID (App ID) | 1. adım |
| Uygulama gizli anahtarı (App secret) | 1. adım |

Messora kaydetmeden önce token'ı Meta ile doğrular (geçerlilik, izinler, WABA'ya erişim), telefon numaralarını ve şablonları içe aktarır. Token ve App secret şifrelenerek saklanır, panelde bir daha **gösterilmez**; değiştirmek için **Kanallar → Düzenle / token değiştir**.

## 4. Webhook'u uygulamanızda ayarlayın

Aynı sayfadaki **Geri çağırma adresi (Callback URL)** ve **Doğrulama belirteci (Verify token)** firmanıza özeldir; kopyala düğmeleriyle alın.

1. Uygulama panelinde **WhatsApp → Configuration → Webhook → Edit**.
2. **Callback URL:** Messora'daki geri çağırma adresi (`https://hooks.messora.tech/meta/…`).
3. **Verify token:** Messora'daki doğrulama belirteci.
4. **Verify and save**. Meta adresi hemen çağırır; belirteç eşleşirse kaydedilir.
5. **Webhook fields → Manage** altında şu alanlara **Subscribe** deyin:
   - `messages` (gelen mesajlar ve teslim durumları — zorunlu)
   - `message_template_status_update`
   - `message_template_quality_update`
   - `phone_number_quality_update`
   - `account_update`
   - `business_capability_update`
   - `user_preferences`

   Abone olunmayan alan hiç gönderilmez; Messora bunu fark edemez.

## 5. Bağlantıyı test edin

**Bağlantıyı test et** düğmesi hesap başına şunları kontrol eder:

| Kontrol | Ne bakılır |
|---|---|
| Uygulama bilgileri | App ID ve App secret kayıtlı mı |
| Erişim belirteci | Meta token'ı uygulamanızla doğruluyor mu (`debug_token`), süresi doluyor mu, aynı uygulamaya mı ait |
| Telefon numaraları | Token WABA'nın numaralarını okuyabiliyor mu |
| Webhook aboneliği | Uygulamanız WABA'nın webhook'larına abone mi (`subscribed_apps`); değilse Messora abone eder |
| Gelen webhook'lar | Meta'dan doğru imzalı bir webhook gelmiş mi |

İlk webhook bir sonraki mesaj ya da durum bildirimiyle gelir: telefonunuzdan numaraya "merhaba" yazıp testi tekrarlayın; mesaj Gelen kutusunda görünmelidir.

## Bağlantı durumu

**Bağlantı durumu** kartı şunları gösterir:

- **Son webhook:** Meta'dan gelen son doğru imzalı webhook (dakika hassasiyetinde).
- **Reddedilen imzalar:** imzası olmayan ya da yanlış olan istekler. Artıyorsa App secret, webhook'u buraya yönlendiren uygulamanın sırrı olmayabilir.
- **Bağlı olmayan hesaplara gelen olaylar:** uygulamanız, Messora'da bağlı olmayan bir WABA için olay gönderdi; bu olaylar işlenmez. O WABA'yı bağlayın ya da uygulamanızda aboneliğini kaldırın.
- **Erişim token'ları:** her hesabın token türü ve bitiş tarihi.

## Adres ya da belirteç sızarsa

- **Yeni adres:** yeni bir geri çağırma adresi üretir. Eski adres seçtiğiniz süre boyunca (hemen, 24 saat, 3 gün ya da 7 gün) çalışmaya devam eder; bu sürede yeni adresi uygulamanıza girip **Verify and save** deyin. Süre dolunca eski adres yanıt vermez.
- **Yeni doğrulama belirteci:** eski belirteç hemen geçersiz olur. Meta belirteci yalnızca adres doğrulanırken ister; yenisini girip tekrar doğrulayın.
- App secret sızdıysa Meta'da **App settings → Basic → Reset** ile yenileyin ve Messora'da hesabın App secret'ını değiştirin.

## Sık karşılaşılan sorunlar

| Belirti | Neden ve çözüm |
|---|---|
| Meta "The callback URL or verify token couldn't be validated" diyor | Verify token'ı Messora'dan yeniden kopyalayın; adresin tamamını (sondaki anahtar dahil) yapıştırdığınızdan emin olun. |
| Mesajlar gelmiyor, "Reddedilen imzalar" artıyor | App secret yanlış ya da başka bir uygulamaya ait. Hesabı düzenleyip doğru App secret'ı girin. |
| Mesajlar gelmiyor, sayaçlar sıfır | Webhook alanlarına (en az `messages`) abone olun; testte "Webhook aboneliği" kontrolüne bakın. |
| "Bağlı olmayan hesaplara gelen olaylar" artıyor | Uygulamanız birden fazla WABA'ya abone; diğer WABA'yı da bağlayın ya da aboneliğini kaldırın. |
| Token "süresi doluyor" uyarısı | Geçici token kullanılmış; 2. adımdaki gibi süresi dolmayan bir System User token'ı oluşturup **Düzenle / token değiştir** ile girin. |
| Eski adres 404 veriyor | Adres yenilenmiş ve süresi dolmuş; güncel adresi Ayarlar → WhatsApp / Meta'dan kopyalayın. |

## Messora üzerinden bağlanma

Bu yöntem, Messora Meta'nın **Tech Provider** onayını aldıktan ve platform yöneticisi özelliği açtıktan sonra kullanılabilir ([TECH_PROVIDER.md](TECH_PROVIDER.md)). Kapalıyken Ayarlar → WhatsApp / Meta'daki kart "Yakında" olarak görünür.

Meta uygulaması, App ID, App secret ya da token gerekmez; webhook adresi de girmezsiniz. Hesabınız Messora'nın Meta uygulamasına bağlanır, olaylar Messora'nın tek adresine gelir ve WABA kimliğinizle size yönlendirilir.

1. **Ayarlar → WhatsApp / Meta → Bağlantı yöntemi: Messora Tech Provider.**
2. Verdiğiniz izinleri okuyun: WABA'nızı yönetmek (`whatsapp_business_management`), mesaj gönderip almak (`whatsapp_business_messaging`), pencerede seçtiğiniz işletme portföyünü okumak (`business_management`).
3. **İki adımlı doğrulama PIN'i:** numaranızın zaten bir PIN'i varsa "Kendi PIN'imi kullanayım" deyip onu girin. Yoksa Messora bir PIN seçer ve bağlantı bitince **bir kez** gösterir; güvenli bir yere kaydedin (numarayı başka bir sağlayıcıya taşırken gerekir).
4. **Meta ile bağlan:** Meta'nın penceresi açılır (açılır pencere engelleyicisine izin verin). Facebook ile giriş yapın, işletmenizi, WhatsApp Business hesabınızı ve numaranızı seçin ya da oluşturun, numarayı SMS/sesli kodla doğrulayın, **Finish** deyin.
5. Messora adımları sırayla yapar ve ilerlemeyi gösterir: Meta'nın verdiği izinler kontrol edilir, hesap bu çalışma alanına ayrılır, webhook aboneliği yapılır, numara kaydedilir, hesap etkinleştirilir. Bitince hesap Kanallar'da "aktif" görünür.
6. WhatsApp Manager'da **ödeme yöntemi** ekleyin (Overview → Add payment method); yoksa ücretli şablon mesajları gönderilemez.

**Bir adım başarısız olursa** bağlantı o adımda bekler; kart nedenini ve "Tekrar dene"yi gösterir. Meta'nın penceresinden gelen kod 5 dakika geçerli ve tek kullanımlıktır: süresi dolduysa "Meta ile yeniden bağlan" yeterlidir, önceki adımlar tekrarlanmaz. Numara kaydı PIN yüzünden reddedildiyse doğru PIN'i girip tekrar deneyin. "Vazgeç" yarım bağlantıyı geri alır.

**Kurallar:** bir WABA yalnızca bir çalışma alanında ve tek yöntemle bağlı olabilir.
- Başka bir Messora çalışma alanına bağlı WABA reddedilir.
- Kendi uygulamanızla bağlı bir WABA'yı Messora üzerinden bağlamak için önce o hesabı Kanallar'da **devre dışı** bırakın (geçmişi korunur); bağlantı aynı hesabı devralır.
- **Bağlantıyı kes:** Messora'nın uygulaması WABA'nızdan ayrılır, olaylar artık kabul edilmez, hesap devre dışı kalır; konuşmalar ve geçmiş durur. Hesabı yeniden Meta ile ya da kendi uygulamanızla bağlayabilirsiniz (kendi uygulamanızla bağlarken aynı WABA ID'yi girin; geçmiş aynı hesapta kalır).
- Erişimi Meta tarafında da kaldırabilirsiniz: Business Settings → Integrations → Connected apps.

Teknik ayrıntılar: [WHATSAPP.md](WHATSAPP.md), güvenlik modeli: [SECURITY.md](SECURITY.md) "Meta webhook'ları" ve "Tech Provider webhook'u".
