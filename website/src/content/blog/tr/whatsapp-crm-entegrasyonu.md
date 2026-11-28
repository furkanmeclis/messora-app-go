---
title: "WhatsApp CRM entegrasyonu: API ve webhook ile nasıl kurulur?"
seoTitle: "WhatsApp CRM entegrasyonu rehberi"
description: "WhatsApp CRM ve e-ticaret entegrasyonu: webhook olayları, imza doğrulama, tekrar korumalı istekler, sipariş ve sepet akışları, kendin yap mı hazır çözüm mü?"
date: 2026-11-26
updated: 2026-11-28
translationKey: "crm"
tags: ["WhatsApp CRM", "WhatsApp entegrasyonu", "Webhook", "API"]
faq:
  - q: "WhatsApp'ı CRM'e bağlamak için yazılımcı gerekir mi?"
    a: "Hazır bir entegrasyon veya otomasyon aracı kullanıyorsanız çoğu zaman gerekmez. Kendi CRM'inize özel alanlar eşlemek, imzalı webhook almak veya sipariş sisteminizden mesaj tetiklemek istiyorsanız birkaç günlük geliştirme işi gerekir."
  - q: "Webhook imzası neden doğrulanmalı?"
    a: "İmza, isteğin gerçekten mesajlaşma platformunuzdan geldiğini ve yolda değiştirilmediğini kanıtlar. Doğrulamayan bir sistem, adresini bilen herkesin sahte sipariş veya sahte müşteri olayı göndermesine açıktır."
  - q: "Aynı webhook iki kez gelirse ne olur?"
    a: "Çoğu platform webhook'ları 'en az bir kez' teslim eder; aynı olay birden fazla gelebilir. Olay kimliğini kaydedip daha önce işlenmiş olanları atlayarak çift kayıt ve çift mesajı önlersiniz."
  - q: "Sepet hatırlatma mesajı fayda mı, pazarlama mı?"
    a: "Sepet hatırlatma bir satış teşviki olduğu için pazarlama kategorisindedir. Yalnızca pazarlama mesajı almayı kabul etmiş müşterilere gönderilmelidir."
---

WhatsApp, müşterinizle en çok konuştuğunuz kanal olabilir; ama konuşmalar bir telefonda, sipariş bilgisi e-ticaret panelinde, müşteri notları CRM'de duruyorsa ekibiniz sürekli sekme değiştirir. WhatsApp CRM entegrasyonu bu kopukluğu giderir: sipariş durumu WhatsApp'tan otomatik gider, müşterinin mesajı CRM'de görünür, kampanyadan çıkan bir müşteri her sistemde çıkmış olarak işaretlenir.

Bu yazıda WhatsApp entegrasyonunun nasıl çalıştığını, hangi olayların işe yaradığını, güvenlik ve güvenilirlik için dikkat edilmesi gereken iki kavramı (imza doğrulama ve tekrar koruması) ve en yaygın akışları anlatıyoruz.

## Entegrasyonun iki yönü

Her WhatsApp entegrasyonu iki yönlü bir trafikten oluşur:

| Yön | Teknik karşılığı | Örnek |
|---|---|---|
| Sizin sisteminizden WhatsApp'a | API çağrısı | Sipariş kargoya verildiğinde müşteriye şablon mesajı göndermek |
| WhatsApp'tan sizin sisteminize | Webhook (olay bildirimi) | Müşteri mesaj yazdığında CRM'de bir aktivite oluşturmak |

**API**, sizin sisteminizin mesajlaşma platformuna "şunu yap" demesidir: mesaj gönder, kişi oluştur, etiket ekle.

**Webhook** ise platformun sizin sisteminize "şu oldu" demesidir: yeni mesaj geldi, mesaj okundu, müşteri abonelikten çıktı. Platform, sizin belirlediğiniz bir adrese her olayda bir istek gönderir.

## Meta'ya doğrudan mı, bir platform üzerinden mi?

Meta'nın Cloud API'si de API ve webhook'lardan oluşur. Doğrudan Meta'ya bağlanabilirsiniz; ama o zaman şunları sizin yazmanız gerekir: 24 saatlik pencere takibi, şablon yönetimi, onay ve çıkış kuralları, teslim durumlarının sırası, yeniden deneme mantığı, ekip için bir arayüz.

Çoğu işletme için daha pratik yol, WhatsApp'ı bir mesajlaşma platformuna bağlamak ve CRM'i o platformun API'si ve webhook'larıyla entegre etmektir. Böylece CRM'iniz Meta'nın teknik ayrıntılarıyla değil, "müşteri mesaj yazdı", "kampanya bitti" gibi iş olaylarıyla ilgilenir.

## İşe yarayan olay örnekleri

Bir mesajlaşma platformunun webhook'larında görmeyi bekleyeceğiniz olaylar ve CRM'deki karşılıkları:

| Olay | Ne zaman gelir? | CRM'de ne yapılır? |
|---|---|---|
| Mesaj alındı | Müşteri yeni bir mesaj yazdığında | Kişi kartına aktivite eklenir; yoksa yeni potansiyel müşteri oluşturulur |
| Mesaj durumu | Mesaj gönderildi, teslim edildi, okundu veya başarısız oldu | Bildirimin müşteriye ulaşıp ulaşmadığı işaretlenir |
| Konuşma atandı | Bir temsilci konuşmayı üstlendiğinde | CRM'deki müşteri sorumlusu güncellenir |
| Konuşma kapandı | Ekip konuşmayı kapattığında | Destek kaydı kapatılır, memnuniyet anketi tetiklenebilir |
| Kampanya tamamlandı | Toplu gönderim bittiğinde | Kampanya sonuçları pazarlama raporuna yazılır |
| Kişi oluşturuldu / güncellendi | Yeni kişi eklendiğinde veya bilgisi değiştiğinde | CRM'deki kişi eşitlenir |
| Kişi abonelikten çıktı / yeniden abone oldu | Müşteri DUR yazdığında veya yeniden katıldığında | Tüm pazarlama kanallarında onay durumu güncellenir |

Abonelikten çıkma olayı özellikle önemli. Müşteri WhatsApp'ta "DUR" yazdıysa, aynı kişinin e-posta veya SMS pazarlamasında da tercihini gözden geçirmek ve ret bildirimlerini ilgili sistemlere (Türkiye'de İYS dahil) iletmek isteyebilirsiniz. Yasal çerçeve için [WhatsApp toplu mesaj ve yasal çerçeve](/blog/whatsapp-toplu-mesaj-gonderme-yasal-cerceve) yazımıza bakabilirsiniz.

## Güvenlik: imzalı webhook ve HMAC doğrulaması

Webhook adresiniz internete açık bir adres. Adresi bilen biri, sahte bir "sipariş ödendi" veya "müşteri mesajı" isteği gönderebilir. Bunu önlemenin standart yolu **imzalı webhook**'tur.

### Nasıl çalışır?

1. Webhook'u tanımladığınızda platform size gizli bir anahtar (signing secret) verir. Bu anahtarı yalnızca siz ve platform bilirsiniz.
2. Platform her istekte, isteğin gövdesini ve bir zaman damgasını bu anahtarla **HMAC-SHA256** algoritmasından geçirir ve sonucu bir başlıkta (header) gönderir.
3. Siz isteği aldığınızda aynı hesaplamayı kendi tarafınızda yaparsınız. Sonuç başlıktaki imzayla aynıysa istek gerçektir ve değiştirilmemiştir.

### Doğru doğrulama için dört kural

- **Ham gövdeyi kullanın.** İmzayı, isteğin JSON olarak ayrıştırılmış halinden değil, gelen ham metinden hesaplayın. Ayrıştırıp tekrar yazdığınız JSON'da boşluklar veya sıralama değişebilir.
- **Sabit zamanlı karşılaştırma yapın.** İmzaları karşılaştırırken dilinizin sabit zamanlı karşılaştırma fonksiyonunu kullanın (`timingSafeEqual`, `hmac.compare_digest` gibi). Normal string karşılaştırması zamanlama saldırılarına açıktır.
- **Zaman damgasını kontrol edin.** Birkaç dakikadan eski istekleri reddedin. Bu, yakalanmış eski bir isteğin tekrar gönderilmesini (replay) önler.
- **Anahtarı kodun içine yazmayın.** Gizli anahtarı ortam değişkeninde veya bir gizli anahtar yöneticisinde saklayın; sızdığından şüphelenirseniz yenileyin.

Mantığı özetleyen bir sözde kod:

```text
imza_basligi = istek.baslik("Signature")      # örn. t=1795426205,v1=e3d1...
t, v1 = ayristir(imza_basligi)
eger simdi - t > 300 saniye: reddet
beklenen = HMAC_SHA256(gizli_anahtar, t + "." + ham_govde)
eger sabit_zamanli_esit(beklenen, v1) degilse: reddet
kabul et
```

Meta da kendi webhook'larını uygulama gizli anahtarıyla imzalar (`X-Hub-Signature-256` başlığı). Doğrudan Meta'ya bağlanıyorsanız aynı doğrulamayı yapmanız gerekir.

### Webhook adresiniz için diğer önlemler

- Yalnızca HTTPS kullanın.
- İsteği aldığınızda hızlıca `2xx` cevabı verin, uzun işleri sonra yapın. Yavaş cevap veren adresler zaman aşımına düşer ve yeniden denenir.
- Kalıcı olarak hata veren bir adresi izleyin; teslim günlüğünü düzenli kontrol edin.

## Güvenilirlik: tekrar koruması (idempotency)

İnternet güvenilir değildir. Bir istek gider ama cevabı kaybolur; gönderen taraf "gitmedi" sanıp tekrar dener. WhatsApp entegrasyonunda bunun iki tehlikeli sonucu olabilir: müşteriye aynı mesaj iki kez gider ya da CRM'de aynı kayıt iki kez oluşur.

### Webhook'ları alırken

Çoğu platform webhook'ları **en az bir kez** teslim eder ve sıra garantisi vermez. Bu yüzden:

- Her olayın benzersiz bir kimliği (event ID) vardır. İşlediğiniz kimlikleri kaydedin; aynı kimlik tekrar gelirse işlemi atlayıp yine `2xx` dönün.
- Sıraya güvenmeyin. "Okundu" bilgisi "teslim edildi"den önce gelebilir. Durumları geriye gitmeyecek şekilde güncelleyin: okunmuş bir mesajı, geç gelen "teslim edildi" bilgisiyle geri almayın.
- Olayın zaman damgasını kullanın; daha eski bir olay, daha yeni bir bilginin üstüne yazılmamalı.

### API çağrısı yaparken

Mesaj gönderen bir API çağrısını tekrar ederken aynı mesajın iki kez gitmemesi için **Idempotency-Key** başlığı kullanılır:

1. Her gönderim için benzersiz bir anahtar üretin, örneğin `siparis-10482-kargo`.
2. İsteği bu anahtarla gönderin.
3. Cevap gelmezse aynı anahtarla tekrar deneyin. Platform bu anahtarla daha önce bir istek aldıysa ikinci kez mesaj oluşturmaz, ilk isteğin sonucunu döner.

Anahtarı iş olayından türetmek (sipariş numarası + olay türü) en sağlam yöntemdir; aynı sipariş için kargo bildirimi iki kez tetiklense bile müşteriye bir kez gider.

## Tipik akışlar

### 1. Sipariş durumu bildirimleri

**Tetikleyici:** E-ticaret sisteminizde sipariş durumu değişir (onaylandı, kargoya verildi, teslim edildi).

**Akış:**

1. E-ticaret sistemi bir olay üretir veya sizin entegrasyonunuz durumu fark eder.
2. Entegrasyon, mesajlaşma platformunun API'sine onaylı bir **fayda şablonu** ile gönderim isteği yapar; değişkenler sipariş numarası, kargo firması, takip numarasıdır.
3. İstek, sipariş numarası ve durumdan türetilmiş bir Idempotency-Key taşır.
4. Mesaj durumu webhook'u geldiğinde sipariş kaydına "müşteriye bildirildi" notu düşülür.

Fayda şablonlarında tanıtım olmamasına dikkat edin; aksi halde Meta şablonu pazarlama olarak sınıflandırır ve maliyet artar. Fiyat farkı için [Meta WhatsApp ücretleri 2026 rehberi](/blog/meta-whatsapp-ucretleri-2026-rehberi).

### 2. Sepet hatırlatma (onaylı müşterilere)

**Tetikleyici:** Müşteri sepete ürün ekledi, belirli bir süre (örneğin 2 saat) içinde ödeme yapmadı.

**Akış:**

1. E-ticaret sistemi terk edilen sepet olayını üretir.
2. Entegrasyon, müşterinin **pazarlama onayı** olup olmadığını kontrol eder. Onay yoksa gönderim yapılmaz.
3. Onaylı müşteriye bir **pazarlama şablonu** gönderilir: sepetteki ürün, sepete dönüş bağlantısı.
4. Müşteri cevap verirse konuşma ekip gelen kutusuna düşer; temsilci soruları yanıtlar.
5. Müşteri satın alırsa ikinci hatırlatma iptal edilir.

Sepet hatırlatmayı aynı müşteriye sık göndermeyin; kalite puanınızı ve müşterinin sabrını tüketir. Bir sepet için tek hatırlatma çoğu zaman yeterlidir.

### 3. Gelen mesajdan potansiyel müşteri

**Tetikleyici:** Kayıtlı olmayan bir numaradan mesaj gelir.

**Akış:**

1. "Mesaj alındı" webhook'u CRM'e ulaşır.
2. CRM numarayı arar; bulamazsa yeni bir potansiyel müşteri oluşturur.
3. Bir yapay zeka ajanı veya karşılama akışı müşteriden ad, şehir, ilgilendiği ürün gibi bilgileri toplar.
4. "Kişi güncellendi" webhook'u bu bilgileri CRM'e taşır ve satış temsilcisine görev açılır.

### 4. Kapanan konuşmadan memnuniyet ölçümü

**Tetikleyici:** "Konuşma kapandı" olayı.

**Akış:** CRM destek kaydını kapatır, pencere hâlâ açıksa kısa bir memnuniyet sorusu gönderir, cevabı kişi kartına yazar.

## Veri eşleme: küçük ayrıntılar, büyük sorunlar

Entegrasyonların çoğu teknik olarak değil, veri tutarsızlığı yüzünden aksar. Başlamadan şu konularda karar verin.

### Telefon numarası biçimi

Aynı numara CRM'de "0532 123 45 67", e-ticaret sisteminde "5321234567", WhatsApp'ta "905321234567" olarak durabilir. Eşleştirme için tek bir biçim seçin: ülke koduyla başlayan uluslararası biçim (E.164, örneğin +905321234567). Entegrasyonunuz her iki yönde de numarayı bu biçime çevirsin. Aksi halde aynı müşteri için iki ayrı kayıt oluşur ve onay bilgisi yanlış kişiye yazılır.

### Hangi sistem neyin sahibi?

Her alan için tek bir "doğruluk kaynağı" belirleyin:

| Alan | Önerilen kaynak |
|---|---|
| Ad, e-posta, adres | CRM |
| Sipariş ve ödeme durumu | E-ticaret veya ERP |
| Konuşma geçmişi, temsilci ataması | Mesajlaşma platformu |
| WhatsApp pazarlama onayı ve çıkış | Mesajlaşma platformu (çıkış anında diğer sistemlere bildirilir) |
| Etiketler ve segmentler | İhtiyaca göre; ama iki yönlü eşitlenecekse çakışma kuralı yazılmalı |

İki sistemin aynı alanı birbirinin üstüne yazdığı döngüler en sık görülen hatadır. Örneğin CRM bir güncellemeyi platforma gönderir, platform "kişi güncellendi" olayı üretir, CRM bunu yeni bir değişiklik sanıp tekrar gönderir. Güncellemeyi kimin yaptığını olayda taşıyın ve kendi yaptığınız değişikliklerden gelen olayları yok sayın.

### Kişisel veri sınırları

CRM'e aktardığınız her alan, KVKK açısından işlediğiniz bir veridir. Konuşmanın tamamını değil, ihtiyacınız olan özeti ve alanları aktarmak çoğu zaman yeterlidir. Mesaj içeriğini taşıyan webhook'ları aldığınız sunucunun da erişim ve saklama kurallarını belirleyin.

## Kendin yap mı, hazır çözüm mü?

| Soru | Kendin yap (Meta'ya doğrudan) | Platform + entegrasyon |
|---|---|---|
| Geliştirme ekibiniz var mı? | Gerekli; sürekli bakım da gerekir | API ve webhook bilen biri yeterli; çoğu iş hazır |
| Ekip gelen kutusu? | Ayrıca geliştirmeniz gerekir | Hazır |
| 24 saat penceresi, şablon, onay kuralları | Sizin sorumluluğunuzda | Platform uygular |
| Meta'nın API değişikliklerini takip | Sizin işiniz | Platformun işi |
| Esneklik | Tam | Platformun API'si kadar |
| Maliyet | Geliştirme ve bakım zamanı | Abonelik |

Kendin yap yolu, WhatsApp'ın ürününüzün çekirdeği olduğu durumlarda mantıklıdır; örneğin WhatsApp üzerinden hizmet veren bir yazılım geliştiriyorsanız. Ürününüz başka bir şeyse ve WhatsApp bir iletişim kanalıysa, bir platform kullanıp yalnızca iş akışlarınızı entegre etmek genellikle daha hızlı ve daha az risklidir.

Karar verirken şu soruları sorun:

- Platformun API'si ihtiyacınız olan işlemleri (mesaj gönderme, kişi güncelleme, etiketleme) destekliyor mu?
- Webhook'lar imzalı mı, yeniden deneme yapılıyor mu, teslim günlüğü görülebiliyor mu?
- API anahtarları yetki kapsamıyla (scope) sınırlandırılabiliyor mu?
- Idempotency-Key destekleniyor mu?
- Hız sınırları sizin hacminize yetiyor mu?

## Canlıya almadan önce test

Entegrasyonu gerçek müşterilere açmadan önce bir test numarasıyla şu senaryoları deneyin: aynı olayın iki kez gelmesi, olayların ters sırayla gelmesi, webhook adresinizin birkaç dakika kapalı kalması, geçersiz imzalı bir istek ve pazarlama onayı olmayan bir kişiye sepet hatırlatma tetiklenmesi. Her senaryoda sistemin doğru davrandığını görmek, canlıdaki sürprizleri büyük ölçüde azaltır.

## Entegrasyon kontrol listesi

- Hangi olayların CRM'e, hangi işlemlerin WhatsApp'a gideceği yazılı olarak belirlendi
- Webhook adresi HTTPS ve hızlı `2xx` dönüyor
- İmza ham gövde üzerinden, sabit zamanlı karşılaştırmayla doğrulanıyor
- Eski zaman damgalı istekler reddediliyor
- Olay kimlikleri kaydediliyor, tekrarlar atlanıyor
- Durum güncellemeleri sıradan bağımsız ve geriye gitmeyecek şekilde
- Mesaj gönderen API çağrıları Idempotency-Key taşıyor
- Pazarlama şablonları yalnızca onaylı kişilere gidiyor; çıkışlar tüm sistemlere yansıyor
- API anahtarları yalnızca gereken yetkilere sahip ve güvenli saklanıyor
- Teslim günlüğü ve hata bildirimleri izleniyor

## Messora'da API ve webhook

Messora'nın [herkese açık REST API'si](/ozellikler/api-ve-webhook) yetki kapsamıyla sınırlanan API anahtarları, hız sınırları ve Idempotency-Key desteğiyle çalışır. Giden webhook'lar HMAC-SHA256 ile imzalanır, başarısız teslimler yaklaşık 24 saat boyunca yeniden denenir ve her teslimin kaydını panelde görürsünüz. Her şirketin API belgeleri kendi adresinde, kendi temel URL'siyle yayımlanır. Kod yazmadan yapılacak akışlar için [otomasyonlar](/ozellikler/otomasyonlar) da kullanılabilir.

API erişimi plana göre değişir; Başlangıç planında yalnızca webhook, üst planlarda aylık API çağrısı kotası bulunur. Ayrıntılar [fiyatlar sayfasında](/fiyatlar). Kendi sisteminize özel bir akışı konuşmak isterseniz [demo talep edebilirsiniz](/demo).
