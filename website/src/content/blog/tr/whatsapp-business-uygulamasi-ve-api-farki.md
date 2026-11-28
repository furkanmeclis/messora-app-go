---
title: "WhatsApp Business uygulaması ile API arasındaki fark"
seoTitle: "WhatsApp Business uygulaması mı, API mı?"
description: "WhatsApp Business uygulaması ile API karşılaştırması: ekip kullanımı, otomasyon, toplu mesaj, entegrasyon, maliyet, numara taşıma ve resmi olmayan araçlar."
date: 2026-11-16
updated: 2026-11-28
translationKey: "app-vs-api"
tags: ["WhatsApp Business API", "WhatsApp Business uygulaması", "Karşılaştırma"]
faq:
  - q: "Aynı numarayı hem WhatsApp Business uygulamasında hem API'de kullanabilir miyim?"
    a: "Normalde hayır: WhatsApp'ta kayıtlı bir numara, oradaki hesap silinmeden API'ye kaydedilemez. Meta'nın coexistence seçeneği bazı kurulumlarda numarayı iki tarafta birlikte kullanmaya izin veriyor; bu seçenek çözüm ortakları üzerinden sunuluyor, güncel koşullarını kontrol edin."
  - q: "WhatsApp Business uygulaması ücretli mi?"
    a: "Uygulamanın temel kullanımı ücretsiz. API'de ise Meta teslim edilen mesajları kategoriye göre ücretlendirir ve genellikle bir platform aboneliği de gerekir."
  - q: "Ne zaman API'ye geçmeliyim?"
    a: "Birden fazla kişi aynı numaradan cevap veriyorsa, düzenli kampanya gönderiyorsanız, mesajları CRM veya e-ticaret sisteminizle bağlamak istiyorsanız ya da otomasyon ve yapay zeka kullanmak istiyorsanız API'ye geçme zamanı gelmiştir."
  - q: "WhatsApp Web üzerinden toplu mesaj gönderen programlar güvenli mi?"
    a: "Hayır. WhatsApp, ürünlerini toplu veya otomatik mesajlaşma için kullanmanın hizmet koşullarına aykırı olduğunu açıkça belirtiyor. Bu araçlar numaranızın yasaklanmasına yol açabilir ve onay kaydı tutmaz."
---

"WhatsApp Business'ı zaten kullanıyoruz, API'ye neden ihtiyacımız olsun?" Bu soru çoğu KOBİ'nin ilk sorusu. Cevap işletmenin büyüklüğüne değil, WhatsApp'ı nasıl kullandığına bağlı. Tek kişinin telefondan cevap verdiği bir dükkan için uygulama yeterli olabilir. Üç temsilcinin aynı numaraya cevap verdiği, haftada bir kampanya gönderilen bir işletme için değil.

Bu yazıda iki ürünü yan yana koyuyoruz, hangisinin ne zaman yeterli olduğunu ve geçişte nelere dikkat etmeniz gerektiğini anlatıyoruz.

## İki ürün, iki farklı amaç

**WhatsApp Business uygulaması**, küçük işletmeler için telefonda çalışan ücretsiz bir uygulama. Normal WhatsApp'a işletme profili, katalog, hızlı cevaplar, etiketler ve karşılama mesajı gibi özellikler ekler.

**WhatsApp Business API** (bugün pratikte Meta'nın Cloud API'si) ise bir arayüz değil, bir altyapı. Kendi başına bir ekranı yok; bir yazılım platformu veya kendi geliştirdiğiniz sistem üzerinden kullanılır. Ekip gelen kutusu, kampanya aracı, otomasyon ve entegrasyonlar bu altyapının üstüne kurulur.

Kavramın ayrıntısı için [WhatsApp Business API nedir?](/whatsapp-business-api-nedir) rehberimize bakabilirsiniz.

## Karşılaştırma tablosu

| | WhatsApp Business uygulaması | WhatsApp Business API |
|---|---|---|
| Nasıl kullanılır? | Telefonda uygulama, ek olarak sınırlı sayıda bağlı cihaz | Bir platform veya kendi yazılımınız üzerinden, tarayıcıdan |
| Kullanıcı sayısı | Az sayıda cihaz; herkes aynı hesabı görür | Platformun izin verdiği kadar kullanıcı, rol ve yetkiyle |
| Konuşma atama ve sahiplik | Yok; etiketlerle elle takip | Konuşmayı temsilciye atama, iç notlar, devir |
| Otomasyon | Karşılama, uzakta mesajı, hızlı cevaplar | Tetikleyici ve koşullu akışlar, chatbot, yapay zeka ajanı |
| Toplu mesaj | Yayın listeleri; yalnızca numaranızı rehberine kaydetmiş kişilere ulaşır | Onaylı şablonlarla kampanya; mesaj limitleri dahilinde |
| Mesaj başlatma kuralları | Kişiye istediğiniz zaman yazabilirsiniz (spam kuralları geçerli) | Son 24 saatte size yazmamış kişiye yalnızca onaylı şablonla |
| Entegrasyon | Yok | CRM, e-ticaret, ERP, webhook ve API ile |
| Raporlama | Temel mesaj istatistikleri | Teslim, okunma, cevap, çıkış ve maliyet raporları |
| Maliyet | Temel kullanım ücretsiz | Meta'nın mesaj başı ücreti + genellikle platform aboneliği |
| Numara | Bir telefona bağlı | Telefondan bağımsız; numara Meta'nın altyapısında |
| Onay ve çıkış yönetimi | Elle | Sistemli: kişi bazında onay, çıkış kelimeleri, otomatik engelleme |
| Kurulum | Dakikalar | Meta işletme hesabı, numara kaydı, şablon onayı; birkaç gün sürebilir |

## Uygulama ne zaman yeterli?

Şu durumların çoğu sizin için geçerliyse, WhatsApp Business uygulaması bir süre daha işinizi görür:

- WhatsApp'a tek bir kişi, genellikle işletme sahibi cevap veriyor.
- Günde birkaç düzine mesaj geliyor.
- Kampanya göndermiyorsunuz ya da ayda birkaç kez, küçük bir listeye gönderiyorsunuz.
- Mesajları başka bir sisteme aktarma ihtiyacınız yok.
- Mesaj başı ücret ve abonelik maliyeti şu an için bütçenize uymuyor.

Uygulamanın gücü sadeliği. Kurulum gerektirmez, ücretsizdir ve müşteriye istediğiniz zaman yazabilirsiniz.

## API'ye geçme zamanı geldiğinin işaretleri

- **Aynı telefonu birden fazla kişi kullanıyor.** Kim hangi müşteriye cevap verdi, belli değil. Bazı mesajlar iki kez cevaplanıyor, bazıları hiç.
- **Mesajlar kaçıyor.** Yoğun saatlerde cevapsız kalan müşteriler var ve bunu ölçemiyorsunuz.
- **Kampanya göndermek istiyorsunuz.** Yayın listeleri yalnızca numaranızı kaydetmiş kişilere ulaşıyor ve sonuçları ölçemiyorsunuz.
- **Müşteri bilgisi dağınık.** Sipariş bilgisi e-ticaret panelinde, konuşma telefonda, notlar bir defterde.
- **Mesai dışında cevap veremiyorsunuz.** Otomatik cevap veya yapay zeka ile ilk karşılamayı yapmak istiyorsunuz.
- **Telefon değişince her şey gidiyor.** Çalışan ayrıldığında veya telefon bozulduğunda konuşma geçmişi risk altında.
- **Sipariş ve randevu bildirimlerini otomatik göndermek istiyorsunuz.**

Bu işaretlerden ikisi üçü sizdeyse API size zaman kazandırır.

## Günlük işte fark: aynı gün, iki farklı kurulum

Farkı somutlaştırmak için bir mobilya mağazasının sıradan bir gününü iki kurulumda düşünelim.

### Uygulama ile

Sabah 9'da mağaza telefonunda gece gelen 14 mesaj bekliyor. Satış sorumlusu telefonu alıp sırayla cevaplıyor. Öğleden sonra bir müşteri "dün konuştuğum arkadaşınız yok mu?" diye yazıyor; dün mesajı cevaplayan kişi bilgisayardaki bağlı cihazdan yazmıştı ve ne konuşulduğunu herkes yukarı kaydırarak buluyor. Akşam yeni sezon için bir duyuru gönderilmek isteniyor; yayın listesi kullanılıyor, ama numarayı rehberine kaydetmemiş müşterilere ulaşmıyor ve kaç kişinin mesajı okuduğu bilinmiyor. Kargo firmasından gelen takip numaraları müşterilere tek tek kopyalanıp gönderiliyor.

### API ve bir platform ile

Sabah gece gelen mesajların bir kısmını yapay zeka ajanı cevaplamış, cevaplayamadıklarını "insan bekleyen" olarak işaretlemiş. Konuşmalar temsilcilere atanıyor; her konuşmanın kimde olduğu belli. Dün konuşan müşteri yazdığında konuşma geçmişi ve iç notlar aynı ekranda. Akşamki duyuru, kampanya onayı veren müşterilere onaylı bir şablonla gidiyor; gönderimden önce kaç kişiye ulaşılacağı ve tahmini Meta maliyeti görülüyor, ertesi gün okunma ve cevap oranları raporda. Kargo takip numaraları e-ticaret sisteminden otomatik olarak fayda şablonuyla gidiyor.

İkinci senaryo daha fazla kurulum ve aylık bir maliyet gerektiriyor. Karşılığında ekip, tekrar eden işleri değil müşteriyle konuşmayı yapıyor.

## Maliyeti nasıl karşılaştırmalı?

Uygulama ücretsiz olduğu için karşılaştırma ilk bakışta tek taraflı görünür. Ama doğru soru "API kaç para?" değil, "API olmadan bu iş bize kaça mal oluyor?" sorusudur. Şu kalemleri düşünün:

- Bildirimleri elle göndermeye harcanan temsilci zamanı
- Cevapsız kalan veya geç cevaplanan müşteri mesajları
- Ölçülemediği için tekrarlanan etkisiz kampanyalar
- Telefon değiştiğinde veya çalışan ayrıldığında kaybolan konuşma geçmişi

API tarafındaki maliyet iki parçadır: Meta'nın mesaj başı ücreti ve platform aboneliği. Türkiye'de fayda mesajları (sipariş, kargo, randevu bildirimi) çok düşük ücretlidir; maliyetin büyük kısmı pazarlama mesajlarından gelir. Kendi hacminizle bir hesap yapmak için [ücret rehberimizdeki](/blog/meta-whatsapp-ucretleri-2026-rehberi) örnekleri kullanabilirsiniz.

## Sık yapılan geçiş hataları

- **Tüm rehberi API'ye aktarıp ilk gün kampanya göndermek.** Rehberdeki herkes onay vermiş değildir; onaysız gönderim hem şikayet hem kalite kaybı getirir. Önce onay durumunu ayırın.
- **Fayda şablonlarına kampanya eklemek.** Tanıtım içeren fayda şablonu pazarlamaya taşınır ve daha pahalıya gelir.
- **24 saat kuralını ekibe anlatmamak.** Temsilciler pencere kapandıktan sonra neden serbest mesaj atamadığını anlamazsa müşteriler cevapsız kalır.
- **Geçici token ile canlıya çıkmak.** Kendi Meta uygulamanızla bağlanıyorsanız süresiz bir sistem kullanıcısı token'ı kullanın.
- **Eski numarayı duyurmadan değiştirmek.** Yeni numarayla başlıyorsanız web sitenizi, Google işletme profilinizi ve faturalarınızı aynı gün güncelleyin.

## API'nin getirdiği yeni kurallar

API'ye geçerken bazı alışkanlıkları değiştirmeniz gerekir:

1. **24 saat kuralı.** Müşteri size son 24 saat içinde yazmadıysa serbest metin gönderemezsiniz, yalnızca Meta'nın onayladığı şablonları kullanırsınız. Ayrıntılar için [şablon mesajları ve onay süreci](/blog/whatsapp-sablon-mesajlari-onay-sureci) yazımıza bakın.
2. **Mesaj başı ücret.** Teslim edilen mesajlar kategoriye göre ücretlendirilir. Türkiye fiyatları ve TL örnekleri için [Meta WhatsApp ücretleri 2026 rehberi](/blog/meta-whatsapp-ucretleri-2026-rehberi).
3. **Mesaj limitleri.** Yeni bir hesap 24 saatte 250 farklı kişiye mesaj başlatabilir; limit işletme doğrulaması ve kaliteli gönderimlerle kademeli olarak artar.
4. **Onay zorunluluğu.** Meta, kişinin işletmenizden mesaj almayı kabul etmesini istiyor. Türkiye'deki ticari ileti mevzuatı ve İYS de ayrıca geçerli; [yasal çerçeve yazımız](/blog/whatsapp-toplu-mesaj-gonderme-yasal-cerceve) bu konuyu ayrıntılı anlatıyor.

Bu kurallar ilk bakışta kısıtlayıcı görünse de numaranızın kalitesini ve müşterinin güvenini korur.

## Geçiş notları: numaranızı taşımak

### Bir numara, iki yerde olamaz (genel kural)

Meta'nın kuralına göre WhatsApp'ta kullanılan bir numara, oradaki hesap silinmeden API'ye kaydedilemez. API'ye kaydedilen numara artık WhatsApp veya WhatsApp Business uygulamasında kullanılamaz; normal arama ve SMS için kullanılmaya devam eder.

Seçenekleriniz:

| Seçenek | Artısı | Eksisi |
|---|---|---|
| Yeni bir numarayla başlamak | Mevcut numara ve geçmiş olduğu gibi kalır; risk yok | Müşterilerin yeni numarayı öğrenmesi gerekir |
| Mevcut numarayı taşımak (uygulamadaki hesabı silerek) | Müşteriler aynı numaraya yazmaya devam eder | Uygulamadaki sohbet geçmişi API'ye gelmez; taşıma sırasında kısa bir kesinti olur |
| Coexistence | Numara hem uygulamada hem API'de çalışır; geçmiş eşitlenebilir | Yalnızca bazı sağlayıcılarla ve belirli koşullarda; bazı özellikler kısıtlı |

### Coexistence nedir?

Meta'nın coexistence seçeneği, WhatsApp Business uygulamasında kullandığınız numarayı Cloud API'ye bağlarken uygulamayı da kullanmaya devam etmenizi sağlıyor. Meta'nın belgelerine göre:

- Kurulum, bir Tech Provider veya Solution Partner'ın Embedded Signup akışıyla yapılıyor.
- Uygulamanın güncel bir sürümü gerekiyor.
- Son altı aya kadar bire bir sohbet geçmişi ve kişiler eşitlenebiliyor; grup sohbetleri eşitlenmiyor.
- Hem uygulamada hem API'de kullanılan numaralar için gönderim hızı sabit ve daha düşük.
- Yayın listeleri, sesli ve görüntülü arama gibi bazı uygulama özellikleri API tarafında desteklenmiyor; kaybolan mesajlar gibi bazı özellikler bire bir sohbetlerde kapanıyor.
- Uygulamadan gönderdiğiniz mesajlar ücretsiz kalıyor; API'den gönderdikleriniz API fiyatlarıyla ücretlendiriliyor.

Coexistence'ın hangi ülkelerde ve hangi hesaplarda kullanılabildiği zaman içinde değişebiliyor. Güncel koşulları Meta'nın [coexistence sayfasından](https://developers.facebook.com/documentation/business-messaging/whatsapp/embedded-signup/onboarding-business-app-users) kontrol edin.

### Taşıma öncesi kontrol listesi

- Uygulamadaki önemli sohbetleri yedekleyin veya dışa aktarın.
- Müşteri listenizi (ad, numara, varsa onay bilgisi) bir tabloya çıkarın; API tarafında içe aktaracaksınız.
- Kataloğunuzu ve hızlı cevaplarınızı not edin; API tarafında yeniden kuracaksınız.
- Taşımayı yoğun olmayan bir güne planlayın.
- İlk şablonlarınızı (sipariş bildirimi, randevu hatırlatma) önceden hazırlayın.
- Ekibinize 24 saat kuralını ve şablon kullanımını anlatın.

## Platform seçerken nelere bakmalı?

API'ye geçmeye karar verdiyseniz bir sonraki soru hangi platformla çalışacağınız. Şu soruların cevabını netleştirin:

- **Resmi Cloud API mi kullanılıyor?** Bağlantının Meta'nın resmi API'si üzerinden kurulduğundan emin olun.
- **Meta ücretleri nasıl yansıtılıyor?** Ücreti doğrudan Meta'ya mı ödüyorsunuz, yoksa sağlayıcı kendi payını ekleyerek mi satıyor?
- **Onay ve çıkış yönetimi var mı?** DUR yazan müşterinin otomatik olarak listelerden çıkması gerekir.
- **Kullanıcı ve numara sınırları ihtiyacınıza uyuyor mu?** Ekibinizin büyüklüğünü ve ileride ek numara ihtiyacınızı düşünün.
- **Veriler nerede tutuluyor ve nasıl korunuyor?** Barındırma bölgesi, yedekleme, iki adımlı doğrulama ve yetki rolleri.
- **Taahhüt var mı?** Deneme süresi ve aylık iptal imkanı, yanlış seçimin maliyetini düşürür.

## Peki "WhatsApp Web" tabanlı araçlar nerede duruyor?

Piyasada uygulama ile resmi API arasında bir "üçüncü yol" gibi sunulan araçlar var: WhatsApp Web'i veya telefondaki uygulamayı otomatik kullanan, QR kodla bağlanan, "sınırsız toplu mesaj" vaat eden programlar. Bunlar resmi API değil.

Riskleri:

- **Kullanım koşullarına aykırı.** WhatsApp, ürünlerini toplu veya otomatik mesajlaşma için kullanmanın her zaman hizmet koşullarının ihlali sayıldığını belirtiyor.
- **Numara yasaklanabilir.** WhatsApp bu tür kullanımı tespit edip hesapları engelliyor. Kaybedilen numara çoğu zaman yıllardır kullanılan işletme numarasıdır.
- **Bir gün çalışmayabilir.** WhatsApp Web'deki bir değişiklik aracı bozabilir; arkasında Meta'nın bir taahhüdü yoktur.
- **Onay ve çıkış takibi yok.** Kimin onay verdiği, kimin DUR yazdığı sistemli tutulmaz.
- **Veri güvenliği belirsiz.** QR kodla bağladığınız araç, tüm sohbetlerinize erişebilir.

Bazı paneller hem resmi API hem de "QR ile bağlanan" resmi olmayan hat seçeneği sunuyor. Bir platform seçerken hangi bağlantı türünü kullandığını açıkça sorun.

## Özet: hangisi sizin için?

| Durumunuz | Öneri |
|---|---|
| Tek kişi cevap veriyor, kampanya yok, entegrasyon gerekmiyor | WhatsApp Business uygulaması |
| Birkaç kişi cevap veriyor, mesajlar kaçıyor | API + ekip gelen kutusu |
| Düzenli kampanya ve ölçüm istiyorsunuz | API + kampanya aracı |
| Sipariş, randevu, kargo bildirimlerini otomatik göndermek istiyorsunuz | API + entegrasyon |
| Mesai dışı karşılama ve yapay zeka istiyorsunuz | API + otomasyon ve yapay zeka |
| "Sınırsız toplu mesaj" vaadi | Uzak durun |

## Messora ile geçiş

Messora resmi WhatsApp Business Cloud API üzerinde çalışır; resmi olmayan bağlantı seçeneği yoktur. [Ekip gelen kutusunda](/ozellikler/ekip-gelen-kutusu) konuşmaları temsilcilere atar, iç not alır ve 24 saat kuralını panelde görürsünüz. Müşteri listenizi CSV veya XLSX ile içe aktarabilir, [kampanyaları](/ozellikler/whatsapp-kampanyalari) onay kurallarına uygun şekilde gönderebilirsiniz. Meta bağlantısı şu an kendi Meta uygulamanızla yapılıyor; tek tıkla Embedded Signup yakında geliyor, bu nedenle coexistence de şu an Messora üzerinden sunulmuyor.

14 günlük ücretsiz denemeyle başlamak için [fiyatlar sayfasına](/fiyatlar) göz atabilir ya da kendi durumunuzu konuşmak için [demo talep edebilirsiniz](/demo).
