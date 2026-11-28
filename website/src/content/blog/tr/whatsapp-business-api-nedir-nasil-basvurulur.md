---
title: "WhatsApp Business API nedir, nasıl başvurulur? Adım adım rehber"
seoTitle: "WhatsApp Business API başvurusu: adım adım"
description: "WhatsApp Business API başvurusu için gerekenler: Meta işletme portföyü, işletme doğrulaması, numara, görünen ad, mesaj limitleri ve sık ret nedenleri."
date: 2026-11-12
updated: 2026-11-28
translationKey: "api-apply"
tags: ["WhatsApp Business API", "Başvuru", "Meta"]
faq:
  - q: "WhatsApp Business API başvurusu için bir aracıya ihtiyacım var mı?"
    a: "Hayır. Meta'nın Cloud API'si herkese açık; kendi Meta uygulamanızı oluşturup bir System User token'ı ile doğrudan bağlanabilirsiniz. Bir sağlayıcının Embedded Signup akışı yalnızca kurulumu kısaltır."
  - q: "WhatsApp uygulamasında kullandığım numarayı API'de kullanabilir miyim?"
    a: "Kullanabilirsiniz, ama Meta'nın kuralına göre WhatsApp'ta kayıtlı bir numara, uygulamadaki hesap silinmeden Cloud API'ye kaydedilemez. Bazı kurulumlar için Meta'nın coexistence seçeneği de var; güncel koşullarını Meta'nın sayfasından kontrol edin."
  - q: "İşletme doğrulaması zorunlu mu?"
    a: "Başlamak için zorunlu değil. Doğrulanmamış yeni bir portföy 24 saatte 250 farklı kişiye işletme başlatımlı mesaj gönderebilir. Daha yüksek limitler, daha fazla numara ve daha fazla şablon için doğrulama gerekir ya da işi kolaylaştırır."
  - q: "Mesaj limiti nasıl artar?"
    a: "Doğrulama veya 30 günde 2.000 kaliteli mesajla 2.000'e çıkar. Sonrasında son 7 günde limitinizin en az yarısını kullanır ve kaliteyi yüksek tutarsanız Meta limiti bir üst seviyeye kendisi taşır."
---

WhatsApp Business API, işletmelerin WhatsApp'ı yazılım üzerinden kullanmasını sağlayan resmi arayüz. Bu yazıda kavramı kısaca hatırlatıp asıl soruya odaklanıyoruz: başvuru nasıl yapılır, neler hazır olmalı, hangi adımlar nerede takılır?

Kavramın tamamı için [WhatsApp Business API nedir?](/whatsapp-business-api-nedir) rehberimize bakabilirsiniz. Burada kısa bir özetle başlıyoruz.

## WhatsApp Business API kısaca nedir?

Bugün "WhatsApp Business API" denince pratikte Meta'nın **Cloud API**'si kastediliyor. Mesajlarınız Meta'nın sunucuları üzerinden gider; sizin bir sunucu kurmanız gerekmez. Eskiden işletmelerin kendi sunucularında çalıştırdığı "On-Premises API" Meta tarafından emekliye ayrıldı, yeni kurulumlar Cloud API ile yapılıyor.

API ile şunları yapabilirsiniz:

- Birden fazla temsilcinin aynı numarayı ortak bir gelen kutusundan yönetmesi
- Onaylı şablonlarla kampanya, sipariş bildirimi ve doğrulama kodu gönderimi
- CRM, e-ticaret altyapısı ve kendi sistemlerinizle entegrasyon
- Otomatik cevaplar, sohbet botları ve yapay zeka ajanları

Karşılığında birkaç kurala uyarsınız: müşteri size son 24 saat içinde yazmadıysa yalnızca Meta'nın onayladığı şablonlarla mesaj başlatabilirsiniz, alıcının onayını almanız gerekir ve teslim edilen mesajlar kategoriye göre ücretlendirilir.

## Başvurudan önce hazırlamanız gerekenler

Başvuru tek bir form değil; Meta tarafında birkaç varlığın doğru kurulması. Aşağıdakileri önceden hazırlarsanız süreç çok daha hızlı ilerler.

### 1. Meta işletme portföyü (Business Portfolio)

Meta Business Suite'te işletmeniz adına bir **işletme portföyü** (eski adıyla Business Manager hesabı) olmalı. WhatsApp hesabınız (WhatsApp Business Account, kısaca WABA), telefon numaralarınız, Meta uygulamanız ve kullanıcı yetkileri bu portföyün altında durur.

Dikkat edilecek noktalar:

- Portföyü kişisel bir Facebook hesabıyla açarsınız, ama portföy şirkete aittir. En az iki yönetici ekleyin; tek kişiye bağlı kalan portföyler, o kişi ayrıldığında sorun çıkarır.
- Portföydeki şirket adı, adres ve web sitesi, resmi belgelerinizle aynı olsun. Doğrulamada bu tutarlılık aranır.
- Kurumsal bir e-posta adresi (alan adınızla biten) kullanın.

### 2. İşletme doğrulaması (Business Verification)

İşletme doğrulaması, Meta'nın portföydeki şirketin gerçek ve belirtilen bilgilerle tutarlı olduğunu kontrol etmesi. Başlamak için zorunlu değil, ama limitleri büyütmek için neredeyse her işletmenin bu adımdan geçmesi gerekir.

Doğrulamanın getirdikleri (Meta'nın geliştirici belgelerine göre):

- Mesaj limitinin 250'den 2.000'e çıkmasının yollarından biri
- Portföy başına kayıtlı numara sınırının ikiden 20'ye çıkması
- Şablon sınırının WhatsApp hesabı başına 250'den 6.000'e çıkması (onaylı görünen adı olan en az bir numara da gerekir)

Türkiye'deki bir şirket için genellikle şu belgeler işe yarar: vergi levhası, ticaret sicil gazetesi, şirket adına düzenlenmiş bir fatura veya resmi yazışma. Belgedeki unvan ve adres, portföyde yazdığınızla birebir aynı olmalı. Web sitenizde şirket adının görünmesi de süreci kolaylaştırır. İstenen belge türleri zaman zaman değişebildiği için doğrulama ekranındaki güncel listeyi esas alın.

### 3. Telefon numarası

API'ye bağlayacağınız numara için Meta'nın şartları şöyle:

- Numara size ait olmalı ve ülke kodu ile alan kodu içermeli. Kısa numaralar (short code) desteklenmiyor.
- Doğrulama için SMS veya sesli arama alabilmeli. Sabit hat da kullanılabilir; bu durumda sesli arama ile kod alırsınız.
- **WhatsApp'ta kullanılan bir numara, oradaki hesap silinmeden kaydedilemez.** Numara API'ye kaydedildikten sonra normal arama ve SMS için kullanılmaya devam eder, ama WhatsApp veya WhatsApp Business uygulamasında kullanılamaz.
- Kayıt sırasında iki adımlı doğrulama PIN'i belirlemeniz gerekir. Bu PIN'i güvenli bir yerde saklayın; numarayı silmek veya taşımak için de istenir.

Şu an WhatsApp Business uygulamasında kullandığınız numarayı taşımak istiyorsanız iki yol var. Birincisi uygulamadaki hesabı silip numarayı API'ye kaydetmek; bu durumda uygulamadaki sohbet geçmişi API tarafına gelmez. İkincisi, Meta'nın bazı çözüm ortakları üzerinden sunduğu **coexistence** seçeneği: numara hem uygulamada hem API'de çalışır ve son altı aya kadar sohbet geçmişi eşitlenebilir. Coexistence yalnızca Embedded Signup ile, bir Tech Provider veya Solution Partner üzerinden kurulabiliyor ve bazı özellikler kısıtlı. Güncel koşulları Meta'nın [coexistence sayfasından](https://developers.facebook.com/documentation/business-messaging/whatsapp/embedded-signup/onboarding-business-app-users) kontrol edin. Ayrıntılı karşılaştırma için [WhatsApp Business uygulaması ile API farkı](/blog/whatsapp-business-uygulamasi-ve-api-farki) yazımıza bakabilirsiniz.

Pratik öneri: yeni başlıyorsanız ve mevcut numaranızın geçmişini kaybetmek istemiyorsanız, API için ayrı bir numara almak çoğu zaman en az sürtünmeli yoldur. Müşterilerinizin zaten bildiği numarayı kullanmak istiyorsanız taşımayı sakin bir güne planlayın.

### 4. Görünen ad (Display Name)

Görünen ad, müşterinin WhatsApp'ta sizin numaranızın profilinde gördüğü isim. Numarayı kaydederken girersiniz; Meta bunu kendi kurallarına göre inceler. Meta'nın belgelerine göre görünen ad, numara daha yüksek bir mesaj limitine ulaştığında da otomatik olarak yeniden doğrulamadan geçer.

Meta'nın [görünen ad kurallarının](https://www.facebook.com/business/help/757569725593362) özü şu:

- Ad, işletmenizle açık bir ilişki taşımalı. Web sitenizde, tabelanızda veya faturalarınızda aynı adı görebilmeliler.
- Yalnızca genel bir kelime olamaz ("Mobilya", "Kuaför" gibi). Markanızın adını kullanın; gerekirse "Kaya Mobilya" gibi bir açıklama ekleyin.
- Büyük harf kullanımı doğal olmalı. Kısaltma değilse tamamı büyük harfle yazılmış adlar sorun çıkarır.
- Emoji, gereksiz noktalama ve başka bir markayı çağrıştıran ifadeler kullanmayın.
- Kişisel bir isim tek başına genelde kabul edilmez; işletmenin adı olmalı.

Görünen adı sonradan değiştirebilirsiniz, ama her değişiklik yeniden incelenir ve onaydan sonra numarayı yeniden kaydetmek gerekir. Bu yüzden ilk seferde doğru adı seçmek zaman kazandırır.

## Adım adım başvuru: kendi Meta uygulamanızla Cloud API

Herhangi bir aracıya gerek kalmadan, doğrudan Meta üzerinden kurulum yapabilirsiniz. Adımlar aşağıdaki gibi; menü adları Meta'nın arayüz güncellemeleriyle değişebilir.

1. **Meta geliştirici hesabı açın.** developers.facebook.com adresine işletme portföyünüze erişimi olan hesapla giriş yapın.
2. **Bir uygulama oluşturun.** "Business" türünde bir uygulama açın ve işletme portföyünüze bağlayın.
3. **WhatsApp ürününü ekleyin.** Uygulama panelinde WhatsApp'ı ekleyin. Meta size test numarası ve geçici bir token verir; bunlarla ilk denemeyi yapabilirsiniz.
4. **Gerçek numaranızı ekleyin.** WhatsApp Manager'da numaranızı ekleyin, görünen adı girin, SMS veya sesli aramayla gelen kodla doğrulayın ve iki adımlı doğrulama PIN'ini belirleyin.
5. **Ödeme yöntemi ekleyin.** WhatsApp Manager'da WhatsApp hesabınıza bir ödeme yöntemi (genellikle kredi kartı) tanımlayın. Ödeme yöntemi olmadan şablon mesajları gönderemezsiniz. Hesabın para birimi sonradan değiştirilemiyor ve **TRY seçenekler arasında yok**; çoğu Türk işletme USD seçiyor.
6. **Bir System User oluşturun.** Business Settings içinde "Sistem kullanıcıları" bölümünden bir sistem kullanıcısı açın. Bu kullanıcıya uygulamanızı ve WhatsApp hesabınızı atayın.
7. **Kalıcı token üretin.** Sistem kullanıcısı için `whatsapp_business_messaging` ve `whatsapp_business_management` izinleriyle, süresi dolmayan ("Never") bir token oluşturun. Uygulama panelindeki geçici token birkaç saat içinde geçersiz olur; canlı kullanımda mutlaka sistem kullanıcısı token'ı kullanın.
8. **Webhook'u ayarlayın.** Gelen mesajları ve mesaj durumlarını alabilmek için uygulamanızda bir webhook adresi (herkese açık HTTPS) ve doğrulama token'ı tanımlayın, `messages` başta olmak üzere ihtiyacınız olan alanlara abone olun.
9. **İlk şablonunuzu onaya gönderin.** Müşteriye ilk mesajı başlatmak için onaylı bir şablon gerekir. Meta'ya göre onay kararı 24 saate kadar sürebilir; çoğu şablon daha kısa sürede sonuçlanır.
10. **Test edin.** Kendi numaranıza şablon gönderin, cevap yazın, 24 saatlik pencerede serbest mesajla yanıt verin ve webhook'un bu olayları aldığını kontrol edin.

Bu adımların teknik olanları (token, webhook, imza doğrulaması) bir platform kullandığınızda büyük ölçüde sizin yerinize yapılır. Örneğin bir platform; WABA kimliğini, token'ı ve uygulama gizli anahtarını aldığında token'ı Meta'ya sorarak kontrol eder, numaralarınızı ve şablonlarınızı içeri aktarır ve webhook için girmeniz gereken değerleri gösterir.

## Alternatif yol: sağlayıcı üzerinden Embedded Signup

Meta'nın **Embedded Signup** akışı, kurulumu bir sağlayıcının sitesindeki tek bir pencereye indirir. Facebook hesabınızla giriş yaparsınız, portföyünüzü seçer veya oluşturursunuz, numaranızı doğrularsınız ve sağlayıcı gerekli izinleri otomatik alır. Uygulama oluşturmak, sistem kullanıcısı açmak ve token kopyalamak gerekmez.

Bu akışı yalnızca Meta'nın Tech Provider veya Solution Partner olarak onayladığı sağlayıcılar sunabilir. Messora'da tek tıkla Embedded Signup **yakında** geliyor; şu an Messora'ya kendi Meta uygulamanızla (App ID ve System User token) bağlanıyorsunuz ve panel bu adımları sırayla gösteriyor.

Hangi yolu seçmeli?

| Durum | Önerilen yol |
|---|---|
| Teknik ekibiniz var, kontrolün tamamen sizde olmasını istiyorsunuz | Kendi Meta uygulamanız + System User token |
| Kurulumla uğraşmak istemiyorsunuz | Embedded Signup sunan bir sağlayıcı |
| Mevcut uygulama numaranızı geçmişiyle korumak istiyorsunuz | Coexistence destekleyen bir sağlayıcı (koşulları kontrol edin) |

## Mesaj limitleri: başlangıçta 250, sonra kademeli artış

Mesaj limiti, 24 saatlik kayan bir zaman diliminde **müşteri hizmeti penceresi dışında** mesaj gönderebileceğiniz farklı kişi sayısıdır. Yani size son 24 saatte yazmamış kişilere başlattığınız mesajları sınırlar. Size yazan müşteriye pencere içinde vereceğiniz cevaplar bu limiti tüketmez.

Önemli bir ayrıntı: Meta'nın güncel belgelerine göre limit **numara başına değil, işletme portföyü başına**. Portföydeki tüm numaralar aynı limiti paylaşır.

| Seviye | 24 saatte farklı alıcı |
|---|---|
| Başlangıç (yeni portföy) | 250 |
| 1. kademe | 2.000 |
| 2. kademe | 10.000 |
| 3. kademe | 100.000 |
| En üst | Sınırsız |

### 250'den 2.000'e nasıl çıkılır?

Meta'nın belgelerine göre şu yollardan biri yeterli:

- İşletmenizi doğrulamak
- Sizi kaydeden bir çözüm ortağı varsa, doğrulamayı onun yapması
- 30 günlük kayan bir dönemde, yüksek kaliteli şablonlarla, pencere dışında farklı numaralara 2.000 teslim edilmiş mesaj göndermek

Ardından Meta mesaj kalitenizi inceler; onaylarsa limit hemen 2.000'e çıkar.

### 2.000'den sonrası otomatik

İki koşul birlikte sağlanırsa Meta limiti bir üst seviyeye kendisi taşır:

- Tüm numaralarınızda ve şablonlarınızda mesaj kaliteniz yüksek
- Son 7 günde mevcut limitinizin en az yarısını kullanmışsınız

Koşullar sağlandığında yükseltme altı saat içinde gelir. Pratikte bu şu anlama geliyor: 2.000 limitiniz varken düzenli olarak günde 1.000'den fazla kişiye kaliteli mesaj gönderirseniz 10.000'e çıkarsınız. Limiti büyütmek için kampanyaları tek bir büyük gönderim yerine haftaya yaymak daha sağlıklı.

Limitler zaman zaman güncellenebildiği için kendi limitinizi WhatsApp Manager'dan veya kullandığınız platformdan takip edin.

## Kalite puanı: limitinizin görünmeyen bekçisi

Meta, hem numaranız hem de her şablonunuz için bir kalite puanı tutar: yeşil (yüksek), sarı (orta), kırmızı (düşük). Puan, müşterilerin mesajlarınıza verdiği tepkilerden beslenir: engellemeler, şikayetler, düşük okunma oranları.

Kalite düştüğünde neler olabilir?

- Limit artışı durur, çünkü otomatik yükseltme yüksek kalite ister.
- Kırmızıya düşen bir şablon Meta'nın belgelerine göre ilk seferde 3 saat, ikinci seferde 6 saat duraklatılır; üçüncüde devre dışı bırakılır.
- Ağır veya tekrarlanan politika ihlallerinde hesap kısıtlanabilir ya da kapatılabilir.

Kaliteyi yüksek tutmanın yolları tahmin edilebilir: yalnızca onay veren kişilere yazın, mesajı kişiye ve ilgisine göre seçin, sıklığı abartmayın, abonelikten çıkmayı kolaylaştırın ve çıkan kişiye bir daha pazarlama mesajı göndermeyin. Onay konusunun yasal tarafını [WhatsApp toplu mesaj gönderme ve yasal çerçeve](/blog/whatsapp-toplu-mesaj-gonderme-yasal-cerceve) yazımızda ayrıntılı anlattık.

## Sık karşılaşılan ret ve takılma nedenleri

Başvuru sürecinde en çok vakit kaybettiren durumlar ve çözümleri:

| Sorun | Neden olur? | Çözüm |
|---|---|---|
| İşletme doğrulaması reddedildi | Belgedeki unvan veya adres portföydekiyle aynı değil; belge okunaksız ya da güncel değil | Portföy bilgilerini belgeyle birebir eşleyin, güncel ve net bir belge yükleyin |
| Görünen ad reddedildi | Ad, işletmeyle ilişkilendirilemiyor ya da genel bir kelime | Web sitenizde ve belgelerinizde geçen marka adını kullanın, yeniden başvurun |
| Numara kaydedilemiyor | Numara hâlâ WhatsApp veya WhatsApp Business uygulamasında kayıtlı | Uygulamadaki hesabı silin ya da coexistence seçeneğini değerlendirin |
| Doğrulama kodu gelmiyor | Hat SMS almıyor veya santral arkasında | Sesli arama ile kod isteyin; santral numarasıysa yönlendirmeyi kontrol edin |
| Mesajlar birkaç saat sonra durdu | Geçici token kullanıldı | Süresiz bir System User token'ı üretin |
| Şablon gönderilemiyor | Ödeme yöntemi eklenmemiş veya şablon henüz onaylı değil | WhatsApp Manager'da ödeme yöntemini ve şablon durumunu kontrol edin |
| Şablon reddedildi | Değişken kuralları, politika ihlali, yanlış kategori | [Şablon onay süreci rehberimizdeki](/blog/whatsapp-sablon-mesajlari-onay-sureci) kontrol listesini uygulayın |

Bir de dolaylı bir neden var: resmi olmayan araçlarla daha önce toplu mesaj gönderilmiş ve bu nedenle yasaklanmış bir numara. Yasaklanan bir numarayı API'ye kaydetmeden önce Meta'nın itiraz sürecinden geçmeniz gerekir.

## Başvurudan sonra ilk iki hafta

Kurulum bittiğinde işin asıl kısmı başlar. İlk günlerde şu sırayı öneriyoruz:

1. **Gelen kutusunu açın.** Numaranızı web sitenize, Instagram profilinize ve faturalarınıza ekleyin. Müşterilerin size yazması hem kaliteyi yükseltir hem de ücretsiz servis penceresi açar.
2. **İki üç fayda şablonu onaylatın.** Sipariş onayı, randevu hatırlatma gibi.
3. **Onay toplamaya başlayın.** Web formlarınıza ve kasaya açık bir WhatsApp onay kutusu ekleyin.
4. **Küçük bir kampanyayla başlayın.** Onaylı müşterilerinizin bir kısmına gönderin, okunma ve çıkış oranlarına bakın.
5. **Limitinizi izleyin.** Gönderimleri günlere yayarak limitin doğal olarak büyümesini sağlayın.

## Başvuru kontrol listesi

- Meta işletme portföyü açıldı, en az iki yönetici eklendi
- Portföydeki unvan, adres ve web sitesi resmi belgelerle aynı
- İşletme doğrulaması için güncel belgeler hazır
- API'de kullanılacak numara belirlendi; WhatsApp uygulamasında kayıtlı değil ya da taşıma planı var
- Numara SMS veya sesli arama ile kod alabiliyor
- Görünen ad, marka adıyla uyumlu ve Meta kurallarına uygun
- İki adımlı doğrulama PIN'i güvenli bir yerde saklanıyor
- WhatsApp hesabına ödeme yöntemi eklendi, para birimi bilinçli seçildi
- Süresiz System User token'ı üretildi, geçici token kullanılmıyor
- Webhook adresi ve abonelikler ayarlandı
- İlk şablonlar onaya gönderildi
- Onay toplama yöntemi ve abonelikten çıkma yolu hazır

## Messora ile kurulum

Messora, resmi WhatsApp Business Cloud API üzerinde çalışır. Kendi Meta uygulamanızı App ID ve System User token ile bağlarsınız; panel token'ı Meta'ya sorarak kontrol eder, süresi yaklaşan token için sizi önceden uyarır, numaralarınızı ve şablonlarınızı içeri aktarır. Tek tıkla Embedded Signup yakında geliyor.

Kurulum bittikten sonra [ekip gelen kutusu](/ozellikler/ekip-gelen-kutusu), [şablon oluşturucu](/ozellikler/sablon-olusturucu) ve [kampanyalar](/ozellikler/whatsapp-kampanyalari) aynı panelde hazır olur. Planları [fiyatlar sayfasında](/fiyatlar) görebilir ya da kurulumu birlikte yapmak için [demo talep edebilirsiniz](/demo).
