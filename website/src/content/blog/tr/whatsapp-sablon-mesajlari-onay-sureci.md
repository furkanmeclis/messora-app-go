---
title: "WhatsApp şablon mesajları: kategoriler, örnekler ve onay süreci"
seoTitle: "WhatsApp şablon mesaj onay süreci"
description: "WhatsApp şablon mesajı nasıl hazırlanır ve onaylatılır? Kategoriler, bileşenler, değişken kuralları, ret nedenleri, kalite puanı ve Türkçe şablon örnekleri."
date: 2026-11-20
updated: 2026-11-28
translationKey: "templates"
tags: ["WhatsApp şablon mesaj", "Şablon onayı", "WhatsApp Business API"]
faq:
  - q: "WhatsApp şablon onayı ne kadar sürer?"
    a: "Meta'nın belgelerine göre onay kararı 24 saate kadar sürebilir. Kurallara uygun şablonlar çoğu zaman çok daha kısa sürede sonuçlanır; ret durumunda düzeltip yeniden gönderebilir veya itiraz edebilirsiniz."
  - q: "Şablonum neden pazarlama kategorisine alındı?"
    a: "Fayda şablonu içinde indirim, öneri veya tanıtım gibi pazarlama içeriği varsa Meta onu pazarlama olarak sınıflandırır. Onaylı şablonlar da düzenli taramayla yeniden sınıflandırılabilir; çoğu durumda bir gün önceden bildirim yapılır ve 60 gün içinde inceleme talep edebilirsiniz."
  - q: "Şablonda kaç değişken kullanabilirim?"
    a: "Kesin bir sayı yok, ama değişken sayısı mesajın uzunluğuyla orantılı olmalı. Değişkenler sıralı olmalı, mesaj değişkenle başlayıp bitmemeli ve her değişken için örnek değer verilmelidir."
  - q: "Şablon neden duraklatılır?"
    a: "Müşterilerden olumsuz geri bildirim aldığında şablonun kalite puanı kırmızıya düşer ve Meta onu ilk seferde 3 saat, ikinci seferde 6 saat duraklatır. Üçüncü seferde şablon devre dışı kalır."
---

WhatsApp Business API'de müşteriye ilk mesajı siz başlatacaksanız ya da müşteri size son 24 saat içinde yazmadıysa, yalnızca Meta'nın onayladığı **şablon mesajları** gönderebilirsiniz. Kampanya, sipariş bildirimi, randevu hatırlatma, doğrulama kodu; hepsi bir şablonla başlar.

Bu rehberde şablonların nasıl kurulduğunu, Meta'nın onları nasıl incelediğini ve en sık ret nedenlerinden nasıl kaçınacağınızı anlatıyoruz. Sonda da kullanıma hazır Türkçe örnekler var.

## Şablon mesajı nedir, ne zaman gerekir?

Şablon, içeriği önceden Meta'ya onaylatılmış bir mesaj kalıbıdır. Müşteri adı, sipariş numarası, tarih gibi kısımlar değişken olarak bırakılır ve her gönderimde doldurulur.

| Durum | Ne gönderebilirsiniz? |
|---|---|
| Müşteri size son 24 saat içinde yazdı | Serbest metin, görsel, dosya; isterseniz şablon da |
| Müşteri size hiç yazmadı veya 24 saatten önce yazdı | Yalnızca onaylı şablon |

Şablonun amacı, işletmelerin istenmeyen mesaj göndermesini sınırlamak. Meta her şablonu içeriği, kategorisi ve politikalara uygunluğu açısından inceler.

## Şablon kategorileri

Her şablon üç kategoriden birine girer. Kategori hem fiyatı hem de hangi içeriğe izin verildiğini belirler.

### Pazarlama (Marketing)

En esnek kategori. Bilinirlikten satışa ve yeniden etkileşime kadar her pazarlama amacını kapsar: kampanya duyurusu, yeni ürün, sepet hatırlatma, doğum günü indirimi, etkinlik daveti. Türkiye'de en pahalı kategori budur. Ücretler için [Meta WhatsApp ücretleri 2026 rehberimize](/blog/meta-whatsapp-ucretleri-2026-rehberi) bakabilirsiniz.

### Fayda (Utility)

Kullanıcının bir eylemiyle veya talebiyle tetiklenen, tanıtım içermeyen mesajlar. Fayda sayılması için mesajın kullanıcıya özel olması ya da onun talep ettiği bir şey olması, veya güvenlik gibi kullanıcı için zorunlu bir bilgi taşıması gerekir. Örnekler: sipariş onayı, kargo durumu, randevu hatırlatma, fatura ve ödeme bildirimi, hesap değişikliği uyarısı.

### Doğrulama (Authentication)

Kimlik doğrulama için tek kullanımlık şifre (OTP) gönderen şablonlar. Doğrulama kodu yalnızca bu kategoriyle gönderilebilir. Meta'nın hazır formatlarını kullanır, bir kod kopyalama veya otomatik doldurma butonu içerir ve URL, medya ya da emoji içeremez.

### Karışık içerik pazarlama sayılır

Meta'nın kuralı net: fayda ve pazarlama içeriğini birlikte barındıran bir şablon pazarlama olarak sınıflandırılır. Yalnızca bir değişkenden oluşan, içeriği belirsiz şablonlar da pazarlama sayılır.

| Şablon metni | Kategori |
|---|---|
| "Siparişiniz kargoya verildi. Takip no: {{1}}" | Fayda |
| "Siparişiniz kargoya verildi. Bir sonraki alışverişinizde %10 indirim için KOD10" | Pazarlama |
| "Randevunuz yarın saat {{1}}'de." | Fayda |
| "{{1}}" | Pazarlama (içerik belirsiz) |

## Şablonun bileşenleri

Bir şablon dört bölümden oluşur. Meta'nın bileşen belgelerine göre sınırlar şöyle:

| Bileşen | Zorunlu mu? | Sınırlar |
|---|---|---|
| Başlık (header) | Hayır | Metin: en fazla 60 karakter, 1 değişken. Medya: görsel, video veya belge. Konum başlığı yalnızca fayda ve pazarlamada |
| Gövde (body) | Evet | En fazla 1.024 karakter, birden çok değişken |
| Altbilgi (footer) | Hayır | En fazla 60 karakter |
| Butonlar | Hayır | Toplam en fazla 10 buton |

### Buton türleri

| Buton | Bir şablonda en fazla | Not |
|---|---|---|
| Hızlı cevap (quick reply) | 10 | Metin en fazla 25 karakter |
| Web sitesi (URL) | 2 | Metin en fazla 25 karakter; URL'nin sonuna 1 değişken eklenebilir |
| Telefon numarası | 1 | Metin en fazla 25 karakter |
| Kodu kopyala | 1 | Kopyalanacak kod en fazla 20 karakter |

Üçten fazla buton eklediğinizde müşteri ilk ikisini görür, kalanlar "Tüm seçenekleri gör" altında toplanır. Dört veya daha fazla buton içeren ya da hızlı cevabı başka bir buton türüyle birleştiren şablonlar WhatsApp masaüstünde görüntülenemeyebilir. Önemli eylemi ilk iki butona koyun.

Pazarlama şablonlarında "Bu mesajları almak istemiyorum" gibi bir hızlı cevap butonu eklemek, müşterinin şikayet etmek yerine çıkmasını kolaylaştırır ve kalite puanınızı korur.

### Değişkenler

Değişkenler `{{1}}`, `{{2}}` gibi sıralı numaralarla ya da `{{musteri_adi}}` gibi isimlerle yazılır. Meta'nın kuralları:

- Değişkenler sıralı olmalı: `{{1}}`'den sonra `{{3}}` gelemez.
- Şablon bir değişkenle başlayamaz veya bitemez.
- Süslü parantezler eksiksiz olmalı; değişken içinde `#`, `$`, `%` gibi özel karakterler olmamalı.
- Değişken sayısı metnin uzunluğuyla orantılı olmalı. Kısa bir metinde çok sayıda değişken varsa ya değişken sayısını azaltın ya da metni uzatın.
- Her değişken için gerçekçi bir **örnek değer** verilmeli.

Kötü örnek:

> "{{1}} {{2}}, {{3}}"

İyi örnek:

> "Merhaba {{1}}, {{2}} numaralı siparişiniz bugün kargoya verildi. Kargo takip numaranız: {{3}}. Bizi tercih ettiğiniz için teşekkür ederiz."

## Özel şablon türleri

### Sınırlı süreli teklif (Limited-time offer)

Yalnızca pazarlama kategorisinde. Müşteriye geri sayımlı bir teklif ve kopyalanabilir indirim kodu gösterir.

- Başlık: isteğe bağlı görsel veya video
- Teklif metni: en fazla 16 karakter
- Gövde: en fazla 600 karakter
- Altbilgi kullanılamaz
- Kodu kopyala butonu (kod en fazla 15 karakter) ve URL butonu
- Bitiş zamanı gönderim sırasında verilir; son bir saatte sayaç kırmızıya döner

WhatsApp web ve masaüstü kullanıcıları teklifi göremez, yerine desteklenmediğini belirten bir mesaj görür.

### Kaydırmalı kartlar (Carousel)

Tek mesajda yan yana kaydırılan 2 ila 10 kart. Ürün vitrini, şube listesi, paket seçenekleri için uygundur.

- Her kartta görsel veya video başlık zorunlu
- Kartlarda gövde metni isteğe bağlı, ama bir kartta varsa hepsinde olmalı
- Her kartta en fazla iki buton (hızlı cevap, URL, telefon)
- Tüm kartlar aynı bileşenlere sahip olmalı
- Kart sayısı şablon oluştururken belirlenir; onaylı şablon hep aynı sayıda kartla gönderilir

### Doğrulama (OTP)

Meta'nın hazır metnini kullanır; metni serbestçe yazamazsınız. Kodun süresi ve güvenlik uyarısı gibi ek satırlar seçilebilir. Kodu kopyala veya otomatik doldur butonu içerir.

Müşterinin gördüğü mesaj, Meta'nın seçtiğiniz dildeki hazır metnine dayanır; anlam olarak şuna benzer:

> "{{1}} doğrulama kodunuzdur. Güvenliğiniz için bu kodu kimseyle paylaşmayın."

### WhatsApp Flows

Flows, sohbetin içinde çok adımlı formlar açar: randevu alma, başvuru formu, anket, adres toplama. Şablona bir "Flow" butonu eklenir; müşteri butona dokunduğunda form WhatsApp içinde açılır. Flow'u ayrıca tasarlayıp yayımlamanız gerekir.

## İsimlendirme ve dil

- Şablon adları küçük harf, rakam ve alt çizgiden oluşur: `siparis_kargoda`, `randevu_hatirlatma_v2`.
- Aynı adı farklı dillerde kullanabilirsiniz; her dil ayrı bir şablon olarak sayılır.
- Şablonu oluştururken bir dil kodu seçersiniz (Türkçe için `tr`). Meta metni çevirmez; her dil için metni kendiniz yazarsınız.
- Adı anlamlı tutun. Altı ay sonra `kampanya_3` adının neye ait olduğunu hatırlamak zordur; `kasim_sezon_indirimi` daha iyi.

Şablon sayısında da sınır var: doğrulanmamış bir işletme portföyünde WhatsApp hesabı başına 250 şablon, doğrulanmış ve onaylı görünen adı olan numarası bulunan portföylerde 6.000 şablon. Saatte en fazla 100 şablon oluşturulabilir.

## Onay süreci adım adım

1. **Şablonu oluşturun.** WhatsApp Manager'da, API ile veya kullandığınız platformun şablon oluşturucusunda kategoriyi, dili ve bileşenleri girin.
2. **Örnek değerleri ekleyin.** Her değişken ve medya başlığı için örnek verin. Medya örneği de incelemeye dahildir.
3. **Onaya gönderin.** Şablon "inceleniyor" durumuna geçer.
4. **Sonucu bekleyin.** Meta'ya göre onay kararı **24 saate kadar** sürebilir. Durum değişikliği e-posta, WhatsApp Manager bildirimi ve webhook ile bildirilir.
5. **Onaylandıysa kullanın.** Durum "Aktif - kalite bekleniyor" olur ve hemen gönderebilirsiniz.
6. **Reddedildiyse düzeltin veya itiraz edin.** Ret nedeni Business Support Home'da ve e-postada görünür. Şablonu düzenleyip yeniden gönderebilir ya da itiraz edebilirsiniz; itirazlar da genellikle 24 saat içinde sonuçlanır.

## Sık ret nedenleri ve çözümleri

| Ret nedeni | Örnek | Çözüm |
|---|---|---|
| Değişken biçimi hatalı | `{1}`, `{{1}} {{3}}`, mesajın sonunda `{{2}}` | Çift süslü parantez kullanın, sırayı koruyun, sona bir cümle ekleyin |
| Değişken oranı yüksek | 10 kelimelik metinde 4 değişken | Metni uzatın veya değişkenleri azaltın |
| Örnek değer yok veya anlamsız | `{{1}}` için "test" | Gerçekçi örnekler: "Ayşe", "SP-10482" |
| Yanlış kategori | Fayda olarak gönderilmiş indirim mesajı | Pazarlama seçin ya da tanıtımı çıkarın |
| Politika ihlali | Yasaklı ürün veya hizmet, yanıltıcı ifade | Meta'nın ticaret ve iş politikalarını kontrol edin |
| Hassas bilgi talebi | Tam kart numarası, şifre istemek | Bu bilgileri WhatsApp üzerinden istemeyin |
| Kopya şablon | Aynı gövde ve altbilgiyle ikinci şablon | Mevcut şablonu kullanın veya metni farklılaştırın |
| Karakter veya emoji sınırı | Çok uzun gövde, aşırı emoji | Sınırlara uyun, metni sadeleştirin |
| Kısaltılmış veya belirsiz bağlantı | Kısaltma servisli URL | Kendi alan adınızdaki tam URL'yi kullanın |

## Meta kategoriyi değiştirebilir

Onaylanmış bir şablonun kategorisi sonradan değişebilir. Meta, onaylı şablonları düzenli olarak tarar ve kurallara uymayanları yeniden sınıflandırır:

- **Faydadan pazarlamaya:** Normalde bir gün önceden bildirim yapılır. Daha önce kötüye kullanım uyarısı almış işletmelerde değişiklik bildirimsiz ve anında olur. Şablon onaylı kalır, ama artık pazarlama fiyatından ücretlendirilir.
- **Pazarlama veya faydadan doğrulamaya:** Bildirim yapılır, takip eden ayın ilk günü şablon reddedilmiş duruma geçer. Yerine Meta'nın şablon kütüphanesinden doğrulama şablonu oluşturmanız gerekir.

Değişikliğe itiraz etmek için 60 gününüz var; talep Business Support Home'dan yapılır. Pazarlamayı tekrar tekrar fayda gibi gösteren işletmelere Meta kademeli yaptırımlar uygular: uyarı, en az 7 günlük hız sınırı, fayda şablonu kısıtlaması ve portföy düzeyinde kısıtlama.

## Kalite puanı ve duraklatma

Her onaylı şablonun bir kalite puanı vardır: yeşil, sarı, kırmızı veya henüz bilinmiyor. Puan; müşterilerin engelleme ve şikayetleri, okunma oranı ve etkileşim gibi sinyallerden oluşur.

Kırmızıya düşen şablon gönderilebilir, ama duraklatılma riski taşır. Meta'nın kuralları:

| Durum | Sonuç |
|---|---|
| İlk duraklatma | 3 saat |
| İkinci duraklatma | 6 saat |
| Üçüncü kez | Şablon devre dışı |

Duraklatılmış şablonla gönderim denemeleri reddedilir ama ücretlendirilmez ve mesaj limitinizden düşmez. Devre dışı kalan şablonu düzenleyip yeniden onaya gönderebilirsiniz. Duraklatma bildirimleri e-posta, WhatsApp Manager ve webhook ile gelir; otomatik kampanyalarınızın bu bildirime tepki vermesi gerekir.

Kaliteyi korumak için:

- Şablonu yalnızca onay vermiş ve konuyla ilgilenecek kişilere gönderin.
- İlk satırda kim olduğunuzu ve neden yazdığınızı belli edin.
- Çıkış için bir buton veya "DUR yazın" satırı ekleyin.
- Yeni bir şablonu önce küçük bir grupla deneyin.

Onay ve çıkış kurallarının yasal tarafı için [WhatsApp toplu mesaj ve yasal çerçeve](/blog/whatsapp-toplu-mesaj-gonderme-yasal-cerceve) yazımıza bakabilirsiniz.

## Türkçe şablon örnekleri

Aşağıdaki örnekleri kendi işletmenize uyarlayabilirsiniz. Köşeli parantezler yerine kendi bilgilerinizi koyun.

### Sipariş onayı (fayda)

> Merhaba {{1}}, {{2}} numaralı siparişinizi aldık. Toplam tutar: {{3}} TL. Siparişiniz hazırlandığında size buradan haber vereceğiz.

Buton: "Siparişimi görüntüle" (URL, sonuna sipariş numarası)

### Kargo bildirimi (fayda)

> Merhaba {{1}}, {{2}} numaralı siparişiniz kargoya verildi. Takip numaranız: {{3}}. Teslimat genellikle 1-3 iş günü sürer.

Buton: "Kargomu takip et" (URL)

### Randevu hatırlatma (fayda)

> Merhaba {{1}}, {{2}} tarihinde saat {{3}}'teki randevunuzu hatırlatmak isteriz. Gelemeyecekseniz lütfen bize bildirin.

Butonlar: "Geleceğim", "Randevuyu değiştir" (hızlı cevap)

### Ödeme hatırlatma (fayda)

> Merhaba {{1}}, {{2}} tarihli {{3}} TL tutarındaki faturanızın son ödeme günü {{4}}. Ödemenizi yaptıysanız bu mesajı dikkate almayın.

### Sezon kampanyası (pazarlama)

> Merhaba {{1}}, kış koleksiyonunda seçili ürünlerde %20 indirim başladı. Kampanya {{2}} tarihine kadar geçerli.

Altbilgi: "Mesaj almak istemiyorsanız DUR yazın."
Butonlar: "Koleksiyonu incele" (URL), "Bu mesajları istemiyorum" (hızlı cevap)

### Sepet hatırlatma (pazarlama)

> Merhaba {{1}}, sepetinizde bıraktığınız ürünler hâlâ sizi bekliyor. Stoklar sınırlı olduğu için siparişinizi tamamlamak isterseniz bağlantı aşağıda.

Buton: "Sepetime git" (URL)

Sepet hatırlatmanın fayda değil pazarlama olduğunu unutmayın; yalnızca pazarlama onayı veren müşterilere gönderin.

### Geri bildirim isteği (pazarlama veya fayda)

> Merhaba {{1}}, {{2}} tarihli siparişinizden memnun kaldınız mı? Görüşünüz bizim için değerli.

Butonlar: "Çok memnunum", "Geliştirilebilir" (hızlı cevap)

Yalnızca belirli bir işleme dair geri bildirim isteyen ve tanıtım içermeyen mesajlar fayda olarak onaylanabilir; Meta'nın son kararı belirleyicidir.

## Messora'da şablon oluşturma

[Messora'nın şablon oluşturucusunda](/ozellikler/sablon-olusturucu) metin ve medya başlıkları, butonlar, değişkenler, sınırlı süreli teklif, kaydırmalı kartlar, doğrulama (OTP) şablonları ve WhatsApp Flows hazırlayıp doğrudan Meta'ya onaya gönderebilirsiniz. Onay durumu, duraklatma ve kalite puanı panelde senkron kalır. Onaylanan şablonu [kampanyalarda](/ozellikler/whatsapp-kampanyalari) ve otomasyonlarda kullanabilirsiniz. Denemek için [fiyatlar sayfasından](/fiyatlar) 14 günlük ücretsiz denemeyi başlatabilirsiniz.
