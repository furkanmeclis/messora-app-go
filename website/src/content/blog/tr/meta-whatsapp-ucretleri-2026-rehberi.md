---
title: "Meta WhatsApp ücretleri 2026: mesaj başı fiyatlar ve TL örnekleri"
seoTitle: "WhatsApp API fiyat rehberi 2026"
description: "WhatsApp API fiyatları 2026: mesaj başı model, kategoriler, Türkiye tarifesi, 1 Ekim 2026 değişiklikleri ve TL hesap örnekleriyle maliyet düşürme yolları."
date: 2026-11-18
updated: 2026-11-28
translationKey: "pricing"
tags: ["WhatsApp API fiyat", "Meta ücretleri", "Maliyet"]
faq:
  - q: "WhatsApp Business API'de mesaj başı ne kadar ödenir?"
    a: "Meta, Türkiye'deki alıcılar için 1 Nisan 2026'dan beri pazarlama mesajı başına yaklaşık 0,0109 USD, fayda ve doğrulama mesajı başına yaklaşık 0,0009 USD alıyor. Fiyatlar çeyrek başlarında değişebildiği için Meta'nın resmi fiyat sayfasını kontrol edin."
  - q: "Müşteriye verdiğim cevaplar ücretli mi?"
    a: "1 Ekim 2026'dan itibaren her işletme numarası için ayda ilk 1.000 servis mesajı ücretsiz. Bu kotayı aşan servis mesajları ücretlendiriliyor; açık pencere içinde gönderilen fayda şablonları da artık ücretli."
  - q: "Meta'ya TL ile ödeme yapabilir miyim?"
    a: "Hayır. TRY, Meta'nın WhatsApp için desteklediği fatura para birimleri arasında yok. Türk işletmeler genellikle USD hesapla ve kartla ödüyor; hesabın para birimi oluşturulduktan sonra kolayca değiştirilemiyor."
  - q: "Teslim edilmeyen mesaj için ücret ödenir mi?"
    a: "Hayır. Mesaj başı modelde Meta yalnızca teslim edilen mesajları ücretlendiriyor."
---

WhatsApp Business API'nin en çok sorulan sorusu: "Mesaj başına ne ödeyeceğim?" Cevap, mesajın türüne, alıcının ülkesine ve ayda kaç mesaj gönderdiğinize göre değişiyor. Bu rehberde Meta'nın 2026'daki fiyatlandırma modelini, Türkiye tarifesini, 1 Ekim 2026'da yürürlüğe giren değişiklikleri ve TL cinsinden örnek hesapları bulacaksınız.

Önce iki ayrımı netleştirelim:

- **Meta ücretleri:** Meta'nın her teslim edilen mesaj için aldığı ücret. Hangi platformu kullanırsanız kullanın ödersiniz.
- **Platform ücreti:** Gelen kutusu, kampanya ve otomasyon için kullandığınız yazılımın abonelik bedeli. Bu yazı yalnızca Meta ücretlerini anlatıyor.

> Bu yazıdaki rakamlar 28 Kasım 2026 itibarıyla derlenmiştir. Meta fiyatlarını yılda dört kez güncelleyebiliyor. Karar vermeden önce mutlaka [Meta'nın resmi fiyat sayfasını](https://developers.facebook.com/docs/whatsapp/pricing) kontrol edin.

## Fiyatlandırma modeli: mesaj başı ücret

Meta, 1 Temmuz 2025'te konuşma başı modelden **mesaj başı** modele geçti. Bugünkü model şöyle işliyor:

- Yalnızca **teslim edilen** mesajlar ücretlendirilir.
- Ücret, mesajın **kategorisine** ve alıcının **pazarına** (telefon numarasının ülke koduna) göre belirlenir.
- Her mesajın tek bir kategorisi vardır ve en fazla bir kez ücretlendirilir.

Yani +90 ile başlayan bir numaraya gönderdiğiniz pazarlama mesajı Türkiye pazarlama fiyatından, +49 ile başlayan bir numaraya gönderdiğiniz mesaj Almanya fiyatından ücretlendirilir. İşletmenizin nerede olduğu fiyatı değiştirmez.

## Mesaj kategorileri

| Kategori | Ne için? | Ne zaman gönderilir? |
|---|---|---|
| Pazarlama (marketing) | Kampanya, indirim, yeni ürün, sepet hatırlatma, yeniden etkileşim | Her zaman, onaylı şablonla |
| Fayda (utility) | Sipariş onayı, kargo bildirimi, randevu hatırlatma, fatura bildirimi | Her zaman, onaylı şablonla |
| Doğrulama (authentication) | Tek kullanımlık şifre (OTP), giriş doğrulama | Her zaman, onaylı şablonla |
| Servis (service) | Müşteriye serbest metinli cevaplar | Yalnızca müşteri size son 24 saat içinde yazdıysa |

Şablonun kategorisini siz seçersiniz, ama son sözü Meta söyler. İçinde tanıtım barındıran bir fayda şablonu pazarlama olarak onaylanır veya sonradan pazarlamaya taşınır. Kategori kuralları için [şablon mesajları ve onay süreci](/blog/whatsapp-sablon-mesajlari-onay-sureci) yazımıza bakabilirsiniz.

## Türkiye tarifesi

Türkiye, Meta'nın fiyat tablosunda ayrı bir pazar. 1 Nisan 2026'dan beri geçerli olan ve Türkiye'deki sağlayıcıların sayfalarında da aynı görünen fiyatlar şöyle:

| Kategori | Teslim edilen mesaj başına (USD) |
|---|---|
| Pazarlama | ≈ 0,0109 |
| Fayda | ≈ 0,0009 |
| Doğrulama | ≈ 0,0009 |
| Servis | Numara başına ayda 1.000 mesaj ücretsiz; aşan kısım fayda fiyatından |

Meta'nın sayfası, Türkiye'nin fayda ve doğrulama fiyatlarının 1 Nisan 2026'da düşürüldüğünü ve 1 Ekim 2026'daki fiyat güncellemesinde Türkiye'nin değişen pazarlar arasında olmadığını belirtiyor. Rakamlar Meta'nın indirilebilir fiyat tablosunda yer alıyor; kesin değerler için o dosyayı esas alın.

Fayda ve doğrulama kategorilerinde **hacim kademeleri** de var. Bir pazarda ay içinde gönderdiğiniz ücretli mesaj sayısı arttıkça üst kademelerdeki mesajlar daha düşük fiyattan ücretlendirilebilir. Kademeler işletme portföyündeki tüm WhatsApp hesaplarında birlikte sayılır ve her ay sıfırlanır. Kademe eşikleri ve indirimli fiyatlar da Meta'nın tablosunda.

### Fiyatlar ne zaman değişir?

Meta fiyatları yalnızca çeyrek başlarında değiştirebiliyor: **1 Ocak, 1 Nisan, 1 Temmuz ve 1 Ekim**. Fiyat tablosu değişiklikleri en az bir ay önceden duyuruluyor. Bütçe planlarken bu dört tarihi takviminize ekleyin.

## 1 Ekim 2026'da ne değişti?

2026'nın en önemli fiyat değişikliği servis mesajlarında oldu.

**1. Servis mesajları artık sınırsız ücretsiz değil.** Her işletme numarası ayda 1.000 servis mesajını ücretsiz gönderebiliyor. Kullanılmayan kota devretmiyor ve her ay sıfırlanıyor. Bu kotayı aşan servis mesajları ücretli. Ödeme yöntemi tanımlı değilse kotayı aşan servis mesajları teslim edilmiyor.

**2. Açık pencere içindeki fayda şablonları yeniden ücretli.** Temmuz 2025'ten Ekim 2026'ya kadar, müşteri size yazdıktan sonraki 24 saat içinde gönderilen fayda şablonları ücretsizdi. 1 Ekim 2026'dan itibaren bu mesajlar da normal fayda fiyatından ücretlendiriliyor.

Bu değişikliğin pratik etkisi şu: Çok konuşan bir müşteri hizmetleri ekibi veya her mesaja cevap veren bir sohbet botu, artık Meta tarafında da maliyet oluşturabiliyor. Türkiye'de servis mesajının fayda fiyatından (yaklaşık 0,0009 USD) ücretlendirildiğini düşünürsek tutar küçük, ama hacim büyüdükçe takip edilmesi gereken bir kalem.

Kotanın numara başına mı yoksa WhatsApp hesabı başına mı sayıldığı konusunda bazı sağlayıcıların açıklamaları farklı. Meta'nın kendi sayfası "her işletme telefon numarası" ifadesini kullanıyor.

### Ücretsiz kalan durumlar

- Numara başına aylık ilk 1.000 servis mesajı
- Tıkla-WhatsApp reklamlarından veya Facebook sayfası butonundan gelen müşterilerle açılan **ücretsiz giriş noktası** penceresindeki mesajlar (koşullar ve süre için Meta'nın sayfasına bakın)
- Teslim edilmeyen mesajlar

## Fatura para birimi: TL yok

Meta, WhatsApp ücretlerini 16 para biriminde faturalandırıyor: USD, EUR, GBP, AUD, IDR, INR, AED, ARS, BRL, CLP, COP, MXN, MYR, PEN, SAR ve SGD. **TRY bu listede yok.** Türkiye'deki işletmeler genellikle USD hesap açıp kartla ödüyor.

Dikkat edilmesi gereken iki nokta:

- Hesabın para birimi oluşturulduktan sonra serbestçe değiştirilemiyor. Meta'nın para birimi taşıma araçları var, ama baştan doğru seçmek daha kolay.
- USD ile ödediğiniz için maliyetiniz kurla birlikte değişir. Bütçenizi TL olarak yaparken bir kur varsayımı kullanın ve düzenli güncelleyin.

Yurt dışından alınan hizmetlerin KDV ve muhasebe boyutu için mali müşavirinize danışın.

## TL ile örnek hesaplar

Aşağıdaki hesaplarda **1 USD = 50,5 TL varsayımı** kullandık. Bu varsayımdır; kendi hesabınızda güncel kuru kullanın ve sonuçları buna göre yeniden hesaplayın. Hesaplar KDV ve banka masraflarını içermez, hacim kademesi indirimlerini de hesaba katmaz.

Birim fiyatlar bu kurla:

| Kategori | USD | TL (50,5 kurla, yaklaşık) |
|---|---|---|
| Pazarlama | 0,0109 | 0,55 TL |
| Fayda / doğrulama / kota üstü servis | 0,0009 | 0,045 TL |

### Örnek 1: Aylık bir kampanya

Bir giyim mağazası, onay vermiş 5.000 müşterisine sezon kampanyası gönderiyor ve hepsi teslim ediliyor.

- 5.000 × 0,0109 USD = 54,50 USD
- 54,50 × 50,5 = **yaklaşık 2.752 TL**

Teslim edilmeyen mesajlar ücretlendirilmediği için gerçek tutar biraz daha düşük olabilir.

### Örnek 2: Sipariş ve kargo bildirimleri

Bir e-ticaret sitesi ayda 1.500 sipariş alıyor; her sipariş için bir onay, bir kargo bildirimi gönderiyor: toplam 3.000 fayda mesajı.

- 3.000 × 0,0009 USD = 2,70 USD
- 2,70 × 50,5 = **yaklaşık 136 TL**

Aynı bilgilendirmeyi pazarlama kategorisinde gönderseydiniz yaklaşık 1.650 TL öderdiniz. Doğru kategori, bu örnekte maliyeti on ikide birine indiriyor.

### Örnek 3: Müşteri hizmetleri

Bir numaradan ayda 2.500 servis mesajı (müşteriye cevap) gönderiliyor.

- İlk 1.000 mesaj ücretsiz
- 1.500 × 0,0009 USD = 1,35 USD
- 1,35 × 50,5 = **yaklaşık 68 TL**

### Örnek 4: Doğrulama kodları

Bir uygulama ayda 10.000 giriş kodu gönderiyor.

- 10.000 × 0,0009 USD = 9 USD
- 9 × 50,5 = **yaklaşık 455 TL**

### Hepsi bir arada

| Kalem | Adet | USD | TL (50,5 kur varsayımı) |
|---|---|---|---|
| Kampanya (pazarlama) | 5.000 | 54,50 | ≈ 2.752 |
| Sipariş ve kargo (fayda) | 3.000 | 2,70 | ≈ 136 |
| Kota üstü servis | 1.500 | 1,35 | ≈ 68 |
| Doğrulama kodu | 10.000 | 9,00 | ≈ 455 |
| **Toplam** | | **67,55** | **≈ 3.411** |

Tablo bir şeyi açıkça gösteriyor: **Meta faturanızın büyük kısmını pazarlama mesajları oluşturur.** Maliyeti düşürmek istiyorsanız önce oraya bakın.

## Maliyeti düşürmenin yolları

### 1. Kategoriyi doğru seçin

Sipariş, kargo, randevu ve fatura bildirimleri fayda kategorisine girer ve Türkiye'de pazarlamanın yaklaşık on ikide biri fiyatındadır. Ama fayda şablonuna tanıtım eklemeyin. "Siparişiniz kargoda, bu arada yeni koleksiyona göz atın" gibi bir cümle şablonu pazarlamaya taşır. Meta, pazarlamayı tekrar tekrar fayda gibi gösteren işletmelere uyarı, hız sınırı ve fayda şablonu kısıtlaması gibi yaptırımlar uyguluyor.

### 2. Segmentlere gönderin

Tüm listeye aynı mesajı göndermek en pahalı yöntemdir. İlgisi olmayan kişiye giden mesaj hem ücret hem de kalite puanı kaybettirir. Son 90 günde alışveriş yapanlar, belirli bir kategoriye ilgi gösterenler veya belirli şehirdekiler gibi segmentlerle daha az mesajla daha çok sonuç alırsınız.

### 3. Servis penceresini iyi kullanın

Müşteri size yazdığında 24 saatlik pencere açılır. Bu pencerede serbest metinle cevap verirsiniz; numara başına ayda 1.000 mesaja kadar ücret ödemezsiniz. Müşterinin sorusunu ilk mesajda tam cevaplamak, gereksiz mesaj trafiğini azaltır. 1 Ekim 2026'dan sonra pencere içinde fayda şablonu göndermek artık ücretli olduğu için, pencere açıkken serbest metinli servis mesajı çoğu zaman daha uygun seçenek.

### 4. Müşterinin size yazmasını kolaylaştırın

Web sitenize, faturalarınıza ve mağazanıza WhatsApp bağlantısı veya QR kod koyun. Müşterinin başlattığı sohbetler hem pencere açar hem de kalite puanınızı destekler. Tıkla-WhatsApp reklamlarından gelen müşterilerle ücretsiz giriş noktası penceresi de açılır.

### 5. Teslim edilemeyen numaraları temizleyin

Teslim edilmeyen mesaj ücretlendirilmez, ama listenizdeki geçersiz numaralar raporlarınızı bozar ve gönderim kapasitesi harcar. Meta'nın "teslim edilemiyor" hatası aldığınız numaraları işaretleyin.

### 6. Çıkanları gerçekten çıkarın

Abonelikten çıkmış birine pazarlama mesajı göndermek hem yasal risk hem de boşa ücrettir. Çıkış taleplerini otomatik işleyin. Yasal çerçeve için [WhatsApp toplu mesaj ve yasal çerçeve](/blog/whatsapp-toplu-mesaj-gonderme-yasal-cerceve) yazımıza bakın.

### 7. Gönderim sıklığını ölçün

Aynı kişiye çok sık pazarlama mesajı gönderdiğinizde Meta bazı mesajları kişi başı pazarlama sınırı nedeniyle teslim etmeyebilir ve okunma oranlarınız düşer. Kampanya sonuçlarını izleyip sıklığı buna göre ayarlayın.

## Sağlayıcı seçerken sorulacak sorular

Meta'nın ücreti herkes için aynı, ama bu ücretin size nasıl yansıdığı sağlayıcıdan sağlayıcıya değişiyor. Teklif alırken şu soruları sorun:

- **Meta ücretini kim ödüyor?** Bazı platformlar Meta'yı doğrudan sizin kartınızdan ödetir; bazıları mesajı TL bakiyeden satar. İkisi de olabilir, önemli olan hangisi olduğunu bilmek.
- **Meta ücretinin üstüne ek pay var mı?** TL bakiye satan sağlayıcılar genellikle Meta fiyatının üstüne kendi payını ekler. Bu payın mesaj başına ne kadar olduğunu kategori bazında sorun.
- **Hangi kur kullanılıyor?** TL bakiye modelinde kurun hangi gün ve hangi kaynaktan alındığı, toplam maliyeti doğrudan etkiler.
- **Ücretsiz servis mesajlarından ücret alınıyor mu?** Meta'nın ücretsiz saydığı bir mesaj için sağlayıcının kendi payını alıp almadığını kontrol edin.
- **Teslim edilmeyen mesaj iade ediliyor mu?** Meta teslim edilmeyen mesajı ücretlendirmez; sağlayıcınız da etmemeli.
- **Maliyeti gönderimden önce görebiliyor musunuz?** Kampanyayı başlatmadan önce tahmini tutarı görmek, bütçe sürprizlerini önler.
- **Abonelik ücreti mesaj hacmine bağlı mı?** Bazı planlar kişi veya mesaj sayısıyla ölçeklenir; büyüdükçe maliyetin nasıl değişeceğini hesaplayın.

## Aylık bütçe nasıl planlanır?

Yeni başlayan bir işletme için basit bir yöntem:

1. **Kategorilere ayırın.** Ayda kaç pazarlama, kaç fayda, kaç doğrulama mesajı göndereceğinizi tahmin edin. Servis mesajlarını numara başına ayrı sayın.
2. **Teslim oranını hesaba katın.** Teslim edilmeyen mesajlar ücretlendirilmez; ilk ay için listenizin tamamının teslim edileceğini varsaymak güvenli taraftır.
3. **Kur için pay bırakın.** Bütçenizi güncel kurla hesaplayın ve birkaç puanlık bir oynama payı ekleyin.
4. **Çeyrek başlarını işaretleyin.** 1 Ocak, 1 Nisan, 1 Temmuz ve 1 Ekim'den bir ay önce Meta'nın duyurularını kontrol edin.
5. **İlk aydan sonra gerçek veriyle güncelleyin.** Kampanya raporlarındaki teslim ve maliyet rakamları, tahmininizden daha değerlidir.

## Messora maliyeti gönderimden önce nasıl gösterir?

Messora'da bir kampanyayı başlatmadan önce **ön analiz** ekranını görürsünüz:

- Hedef kitlenizden kaç kişiye gönderilebileceği, kaç kişinin onay veya çıkış nedeniyle engellendiği
- Gönderimin tahmini süresi ve bitiş zamanı
- Meta'nın ülke ve kategori fiyat tablosuna göre **tahmini Meta maliyeti**
- En az 3 geçmiş kampanya ve 500 alıcı varsa beklenen performans; yoksa "yeterli geçmiş yok" bilgisi

Gönderim tamamlandıktan sonra her mesajın kategorisi Meta'nın bildirdiği bilgiyle eşleşir ve maliyet, gönderildiği günkü fiyattan hesaplanır. Meta'nın kendi günlük maliyet verisi de karşılaştırma için çekilir. [Kampanya](/ozellikler/whatsapp-kampanyalari) ve [analitik](/ozellikler/analitik-ve-raporlar) sayfalarında ayrıntıları bulabilirsiniz.

Messora'nın abonelik planları Meta ücretlerinden ayrıdır; plan fiyatlarını [fiyatlar sayfasında](/fiyatlar) TL olarak görebilirsiniz. Meta ücretleri gönderimden önce açıkça gösterilir.

## Özet

- Meta, teslim edilen her mesajı kategori ve alıcı ülkesine göre ücretlendiriyor.
- Türkiye'de pazarlama yaklaşık 0,0109 USD, fayda ve doğrulama yaklaşık 0,0009 USD (1 Nisan 2026'dan beri; Meta'nın sayfasından doğrulayın).
- 1 Ekim 2026'dan beri numara başına ayda 1.000'i aşan servis mesajları ve pencere içindeki fayda şablonları ücretli.
- TL ile faturalama yok; kur varsayımınızı düzenli güncelleyin.
- Maliyetin büyük kısmı pazarlamadan gelir; doğru kategori ve segmentasyon en büyük tasarruf kalemidir.
