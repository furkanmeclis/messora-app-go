---
title: "WhatsApp Business API nedir?"
seoTitle: "WhatsApp Business API nedir? Kapsamlı rehber"
description: "WhatsApp Business API nedir, kimler için uygundur, ne yapılabilir, ne kadar tutar? Cloud API, gereksinimler, mesaj kuralları ve sık sorulan sorular."
updated: 2026-11-28
faq:
  - q: "WhatsApp Business API ücretsiz mi?"
    a: "Hayır. Meta teslim edilen mesajları kategoriye ve alıcının ülkesine göre ücretlendirir. Numara başına ayda 1.000 servis mesajı ücretsizdir. Bunun dışında kullandığınız platformun abonelik ücreti ayrıca ödenir."
  - q: "WhatsApp Business API'yi kullanmak için yazılımcı gerekir mi?"
    a: "Bir platform kullanıyorsanız gerekmez; gelen kutusu, kampanya ve otomasyon tarayıcıdan yönetilir. Kendi sisteminizle özel entegrasyon kurmak istiyorsanız teknik destek gerekebilir."
  - q: "Kaç kişiye mesaj gönderebilirim?"
    a: "Yeni bir işletme portföyü 24 saatte 250 farklı kişiye mesaj başlatabilir. İşletme doğrulaması ve kaliteli gönderimlerle limit 2.000, 10.000, 100.000 ve sınırsıza kadar kademeli olarak artar. Size yazan müşterilere verdiğiniz cevaplar bu limitten düşmez."
  - q: "Mevcut WhatsApp numaramı kullanabilir miyim?"
    a: "Evet, ama WhatsApp uygulamasındaki hesabın silinmesi gerekir; numara aynı anda hem uygulamada hem API'de kullanılamaz. Meta'nın coexistence seçeneği bazı kurulumlarda iki tarafı birlikte kullanmaya izin veriyor; güncel koşullarını kontrol edin."
  - q: "WhatsApp Business API ile toplu mesaj göndermek yasal mı?"
    a: "Onay vermiş kişilere, onaylı şablonlarla ve ret hakkı tanıyarak gönderim yapılabilir. Türkiye'de 6563 sayılı Kanun, ticari elektronik ileti yönetmeliği, İYS ve KVKK kuralları da geçerlidir. Bir hukuk danışmanıyla süreçlerinizi gözden geçirmenizi öneririz."
---

WhatsApp Business API, işletmelerin WhatsApp'ı bir telefon uygulaması üzerinden değil, yazılım üzerinden kullanmasını sağlayan resmi arayüzdür. Meta tarafından sunulur. Birden fazla temsilcinin aynı numaradan müşterilerle konuşmasına, onaylı şablonlarla kampanya ve bildirim göndermeye, WhatsApp'ı CRM ve e-ticaret sistemlerine bağlamaya, sohbet botu ve yapay zeka ajanı çalıştırmaya imkan verir.

Bu rehberde WhatsApp Business API'nin ne olduğunu, kimler için uygun olduğunu, nasıl çalıştığını, ne kadar tuttuğunu ve başlamak için nelere ihtiyacınız olduğunu tek sayfada topladık.

## Kısa tanım

WhatsApp'ın işletmeler için üç farklı yüzü var:

- **WhatsApp:** Bireysel kullanım için bildiğimiz uygulama.
- **WhatsApp Business uygulaması:** Küçük işletmeler için ücretsiz telefon uygulaması. İşletme profili, katalog, hızlı cevaplar, etiketler.
- **WhatsApp Business Platformu (API):** Orta ve büyük ölçekli kullanım için yazılım arayüzü. Kendi ekranı yoktur; bir platform veya kendi yazılımınız üzerinden kullanılır.

"Resmi WhatsApp API" ifadesi bu üçüncüsünü anlatır. İnternette "WhatsApp API" diye satılan ama WhatsApp Web'i otomatik kullanan araçlar resmi API değildir ve numaranızın yasaklanmasına yol açabilir.

## Cloud API, On-Premises API ve uygulama

| | Cloud API | On-Premises API | WhatsApp Business uygulaması |
|---|---|---|---|
| Nerede çalışır? | Meta'nın sunucularında | İşletmenin kendi sunucusunda | Telefonda |
| Durumu | Güncel ve önerilen yol | Meta tarafından emekliye ayrıldı | Güncel |
| Kurulum | Sunucu gerekmez | Sunucu kurulumu ve bakımı gerekirdi | Uygulamayı indirmek yeterli |
| Kimler için? | Ekipler, kampanya ve entegrasyon ihtiyacı olanlar | Artık yeni kurulum için seçenek değil | Tek kişinin yönettiği küçük işletmeler |

Bugün yeni başlayan bir işletme için WhatsApp Business API, pratikte **Cloud API** demektir. Mesajlarınız Meta'nın altyapısı üzerinden gider; güncellemeler ve kapasite Meta tarafından yönetilir.

Uygulama ile API arasındaki farkları ayrıntılı karşılaştırdığımız yazı için: [WhatsApp Business uygulaması ile API arasındaki fark](/blog/whatsapp-business-uygulamasi-ve-api-farki).

## Kimler için uygun?

WhatsApp Business API, işletmenin büyüklüğünden çok WhatsApp'ı nasıl kullandığıyla ilgilidir. Şu durumlardan biri sizin için geçerliyse API'yi değerlendirme zamanı gelmiştir:

- Aynı numaraya birden fazla kişi cevap veriyor.
- Günde onlarca, yüzlerce müşteri mesajı alıyorsunuz ve bazıları cevapsız kalıyor.
- Onay vermiş müşterilerinize düzenli kampanya göndermek ve sonuçlarını ölçmek istiyorsunuz.
- Sipariş, kargo, randevu, ödeme bildirimlerini otomatik göndermek istiyorsunuz.
- WhatsApp'ı CRM, e-ticaret altyapısı veya kendi yazılımınızla bağlamak istiyorsunuz.
- Mesai dışında otomatik cevap veya yapay zeka ile ilk karşılama yapmak istiyorsunuz.

Sektör örnekleri:

| Sektör | Tipik kullanım |
|---|---|
| [E-ticaret](/cozumler/e-ticaret) | Sipariş ve kargo bildirimi, sepet hatırlatma, iade talepleri |
| [Perakende](/cozumler/perakende) | Kampanya duyuruları, mağaza stok soruları, sadakat programı |
| [Sağlık ve klinikler](/cozumler/saglik-ve-klinikler) | Randevu alma ve hatırlatma, ön bilgi toplama |
| [Eğitim](/cozumler/egitim) | Kayıt süreçleri, veli bilgilendirme, ders hatırlatma |
| [Turizm ve otel](/cozumler/turizm-ve-otel) | Rezervasyon onayı, check-in bilgisi, misafir talepleri |
| [Emlak](/cozumler/emlak) | Portföy sorularına cevap, talep toplama, randevu |
| [Hizmet işletmeleri](/cozumler/hizmet-isletmeleri) | Teklif, randevu, servis takibi |

## Neler yapabilirsiniz?

### Ekip gelen kutusu

Tüm temsilciler aynı numaranın konuşmalarını tarayıcıdan yönetir. Konuşmalar kişilere atanır, ekibe özel notlar alınır, kimin neyle ilgilendiği görünür. Telefon değişse veya çalışan ayrılsa da geçmiş kaybolmaz.

### Kampanyalar ve toplu mesaj

Onay vermiş müşterilerinize Meta'nın onayladığı şablonlarla kampanya gönderirsiniz. Segment, etiket veya yüklenen bir listeye göre hedefleme yapılır; teslim, okunma, cevap ve abonelikten çıkış ölçülür. WhatsApp toplu mesaj gönderiminin yasal tarafı için [yasal çerçeve rehberimize](/blog/whatsapp-toplu-mesaj-gonderme-yasal-cerceve) bakabilirsiniz.

### Bildirimler

Sipariş onayı, kargo takibi, randevu hatırlatma, ödeme bildirimi ve doğrulama kodu (OTP) gibi mesajlar sisteminizden otomatik gönderilir.

### Otomasyon ve chatbot

Yeni müşteriyi karşılama, abonelikten çıkma talebini işleme, cevapsız müşteriyi takip etme gibi akışlar tetikleyici ve koşullarla kurulur.

### Yapay zeka ajanları

Yapay zeka, sık sorulan soruları cevaplar, müşteriden bilgi toplar ve emin olmadığında konuşmayı bir temsilciye devreder. Ayrıntılar için [WhatsApp'ta yapay zeka ile müşteri hizmetleri](/blog/whatsapp-yapay-zeka-musteri-hizmetleri).

### Entegrasyonlar

API ve webhook'larla WhatsApp'ı CRM, e-ticaret ve diğer sistemlere bağlarsınız. Bkz. [WhatsApp CRM entegrasyonu](/blog/whatsapp-crm-entegrasyonu).

## Temel kurallar

WhatsApp Business API'yi uygulamadan ayıran birkaç kural var. Bunları baştan bilmek, ilk haftadaki sürprizleri önler.

### 24 saatlik müşteri hizmeti penceresi

Müşteri size mesaj yazdığında 24 saatlik bir pencere açılır. Bu pencerede serbest metin, görsel ve dosya gönderebilirsiniz. Pencere kapandıktan sonra veya müşteri size hiç yazmadıysa yalnızca **onaylı şablonlarla** mesaj başlatabilirsiniz.

### Şablon mesajları

Şablonlar Meta'ya önceden onaylatılan mesaj kalıplarıdır. Pazarlama, fayda ve doğrulama olmak üzere üç kategoriye ayrılır; onay kararı 24 saate kadar sürebilir. Ayrıntılar: [WhatsApp şablon mesajları ve onay süreci](/blog/whatsapp-sablon-mesajlari-onay-sureci).

### Onay (opt-in)

Meta, kişinin telefon numarasını sizinle paylaşmış ve işletmenizden mesaj almayı kabul etmiş olmasını istiyor. Türkiye'de buna ek olarak ticari elektronik ileti mevzuatı ve İYS kuralları geçerli.

### Mesaj limitleri

Yeni bir işletme portföyü 24 saatte, size yazmamış 250 farklı kişiye mesaj başlatabilir. İşletme doğrulaması veya kaliteli gönderimlerle limit 2.000'e, sonra kullanım ve kaliteye göre 10.000, 100.000 ve sınırsıza çıkar. Limit numara başına değil, işletme portföyü başınadır.

### Kalite puanı

Meta, numaranız ve şablonlarınız için müşteri geri bildirimlerine dayalı bir kalite puanı tutar. Düşük kalite limit artışını durdurur, şablonların duraklatılmasına ve ağır durumlarda hesabın kısıtlanmasına yol açabilir.

## Maliyetlere genel bakış

WhatsApp Business API maliyeti iki parçadan oluşur.

**1. Meta'nın mesaj ücretleri.** Meta, 1 Temmuz 2025'ten beri teslim edilen her mesajı kategoriye ve alıcının ülkesine göre ücretlendiriyor. Türkiye'deki alıcılar için 1 Nisan 2026'dan beri yaklaşık fiyatlar:

| Kategori | Mesaj başına (USD) |
|---|---|
| Pazarlama | ≈ 0,0109 |
| Fayda | ≈ 0,0009 |
| Doğrulama | ≈ 0,0009 |
| Servis (müşteriye cevap) | Numara başına ayda 1.000 ücretsiz; aşan kısım ücretli (1 Ekim 2026'dan beri) |

Meta fiyatları yalnızca 1 Ocak, 1 Nisan, 1 Temmuz ve 1 Ekim'de değiştirebiliyor ve TRY fatura para birimleri arasında yok. Güncel fiyatları her zaman [Meta'nın resmi sayfasından](https://developers.facebook.com/docs/whatsapp/pricing) kontrol edin. TL hesap örnekleri için: [Meta WhatsApp ücretleri 2026 rehberi](/blog/meta-whatsapp-ucretleri-2026-rehberi).

**2. Platform ücreti.** API'yi kullandığınız yazılımın aylık aboneliği. Ekip gelen kutusu, kampanya aracı, otomasyon ve yapay zeka bu katmanda gelir.

## Başlamak için gerekenler

- **Meta işletme portföyü:** İşletmenizin Meta Business Suite'teki hesabı.
- **İşletme doğrulaması:** Başlamak için zorunlu değil, ama limitleri büyütmek, daha fazla numara ve şablon kullanmak için gerekli.
- **Telefon numarası:** Size ait, SMS veya sesli arama alabilen bir numara. WhatsApp uygulamasında kayıtlıysa oradaki hesabın silinmesi gerekir.
- **Görünen ad:** Müşterinin gördüğü isim; işletmenizle açıkça ilişkili olmalı ve Meta'nın kurallarına uymalı.
- **Ödeme yöntemi:** Meta'nın mesaj ücretleri için WhatsApp hesabınıza tanımlı bir ödeme yöntemi.
- **Onaylı şablonlar:** İlk mesajları başlatabilmek için.

Başvuru adımlarını ayrıntılı anlattığımız rehber: [WhatsApp Business API nedir, nasıl başvurulur?](/blog/whatsapp-business-api-nedir-nasil-basvurulur)

## Bağlanma yolları

| Yol | Nasıl? | Kimler için? |
|---|---|---|
| Kendi Meta uygulamanızla | Meta geliştirici panelinde uygulama, sistem kullanıcısı ve süresiz token | Kontrolü elinde tutmak isteyenler, teknik ekibi olanlar |
| Embedded Signup | Bir sağlayıcının sitesinde, Facebook hesabıyla tek pencerede kurulum | Kurulumla uğraşmak istemeyenler |
| Doğrudan kendi yazılımınız | Meta'nın API'sine doğrudan entegrasyon | WhatsApp'ı kendi ürününün çekirdeğine koyan yazılım ekipleri |

## Messora bu yapıda nerede duruyor?

Messora, resmi WhatsApp Business Cloud API üzerinde çalışan, Türkiye'deki KOBİ'ler için hazırlanmış bir platform. Meta'nın altyapısını kullanır; üstüne şunları ekler:

- [Ekip gelen kutusu](/ozellikler/ekip-gelen-kutusu): konuşma atama, sahiplik, ekibe özel notlar, 24 saat penceresinin takibi
- [WhatsApp kampanyaları](/ozellikler/whatsapp-kampanyalari): segment ve liste bazlı gönderim, onay ve çıkış yönetimi, gönderimden önce Meta maliyeti ve ön analiz
- [Şablon oluşturucu](/ozellikler/sablon-olusturucu): medya başlıkları, butonlar, sınırlı süreli teklif, kaydırmalı kartlar, OTP ve Flows
- [Otomasyonlar](/ozellikler/otomasyonlar): görsel akış editörü ve hazır otomasyon şablonları
- [Yapay zeka ajanları](/ozellikler/yapay-zeka-ajanlari): numara başına yapay zeka profili, insana devir, bilgi toplama
- [Analitik ve raporlar](/ozellikler/analitik-ve-raporlar): kampanya sonuçları, tahmin ve gerçekleşen karşılaştırması, dışa aktarım
- [API ve webhook](/ozellikler/api-ve-webhook): yetki kapsamlı API anahtarları, imzalı webhook'lar

Meta bağlantısı şu an kendi Meta uygulamanızla (App ID ve sistem kullanıcısı token'ı) yapılıyor; panel adımları sırayla gösteriyor. Tek tıkla Embedded Signup yakında geliyor. Meta'nın mesaj ücretleri plan ücretinden ayrıdır ve gönderimden önce açıkça gösterilir. Veriler AWS Frankfurt (AB) bölgesinde barındırılır; güvenlik ayrıntıları [güvenlik sayfasında](/guvenlik).

Planları [fiyatlar sayfasında](/fiyatlar) inceleyebilir, kart bilgisi vermeden 14 günlük denemeyi başlatabilir ya da [demo talep edebilirsiniz](/demo).
