---
title: "Alt İşleyenler"
description: "Messora'nın hizmeti sunmak için kullandığı alt işleyenler: barındırma, e-posta, yapay zekâ ve ödeme sağlayıcıları, konumları ve değişiklik bildirimi süreci."
updated: 2026-11-28
version: "1.0"
---

Bu sayfa, {{unvan}} ("**{{marka}}**") tarafından Hizmet'in sunulması sırasında kişisel verileri işleyen üçüncü taraf hizmet sağlayıcıları listeler. Liste, [Veri İşleme Sözleşmesi](/yasal/veri-isleme-sozlesmesi)'nin 8. maddesi kapsamında yayımlanır.

**Son güncelleme:** 28 Kasım 2026

## 1. Hizmet altyapısı

| Alt işleyen | Konum | Hizmet | İşlenen veriler | Ne zaman |
|---|---|---|---|---|
| Amazon Web Services EMEA SARL | AWS eu-central-1, Frankfurt (Almanya) | Barındırma, veritabanı, dosya depolama, şifreli yedekleme | Hizmet'te işlenen tüm veriler | Her zaman |
| Resend, Inc. | Amerika Birleşik Devletleri | İşlem e-postaları: e-posta doğrulama, parola sıfırlama, bildirimler, zamanlanmış raporlar, form yanıtları | Alıcının adı ve e-posta adresi, e-posta içeriği | Her zaman |

## 2. Yapay zekâ sağlayıcıları

Aşağıdaki sağlayıcılar **yalnızca Müşteri yapay zekâ özelliklerini etkinleştirip ilgili sağlayıcıyı seçtiğinde** ve yalnızca bu özellik kullanıldığında devreye girer. Sağlayıcıya, yanıt üretmek için gereken konuşma içeriği ve Müşterinin AI temsilcisi talimatları gönderilir. Konuşmalar model eğitiminde kullanılmaz.

| Alt işleyen | Konum | Hizmet | Not |
|---|---|---|---|
| OpenAI, L.L.C. | Amerika Birleşik Devletleri | Yapay zekâ yanıtları ve bilgi çıkarma | İstekler `store: false` ile gönderilir |
| Anthropic, PBC | Amerika Birleşik Devletleri | Yapay zekâ yanıtları ve bilgi çıkarma | Ticari koşullar kapsamında API verileri eğitimde kullanılmaz |
| Google LLC — Gemini API | Amerika Birleşik Devletleri | Yapay zekâ yanıtları ve bilgi çıkarma | Faturalandırması etkin (ücretli) proje anahtarı kullanılmalıdır |
| DeepSeek | Çin Halk Cumhuriyeti olabilir | Yapay zekâ yanıtları ve bilgi çıkarma | Yalnızca Müşteri seçerse. Sunucuları Çin'de olabilir; kişisel veri içeren yanıtlarda kullanmadan önce sağlayıcının koşullarını ve yurt dışı aktarım şartlarını değerlendirin. |

Kurumsal Planda Müşteri kendi sağlayıcı anahtarını kullanıyorsa, sağlayıcıyla sözleşme ilişkisi Müşteri ile sağlayıcı arasındadır ve sağlayıcı {{marka}}'nın alt işleyeni sayılmaz.

## 3. Ödeme

| Alt işleyen | Konum | Hizmet | Ne zaman |
|---|---|---|---|
| PayTR Ödeme ve Elektronik Para Kuruluşu A.Ş. | Türkiye | Kartla ödeme ve iade işlemleri | Kartla ödeme devreye alındığında |

PayTR, ödeme hizmetleri mevzuatı kapsamında kendi yükümlülükleri bakımından ayrıca veri sorumlusu sıfatıyla hareket edebilir.

## 4. Müşterinin kendi sözleşmesine dayanan taraflar

| Taraf | Konum | Hizmet | Not |
|---|---|---|---|
| Meta Platforms Ireland Ltd. / WhatsApp | İrlanda, Amerika Birleşik Devletleri ve Meta'nın veri merkezlerinin bulunduğu diğer ülkeler | WhatsApp Business Cloud API üzerinden mesaj iletimi | Müşterinin bağladığı kendi WhatsApp Business hesabı ve Meta ile yaptığı sözleşme kapsamında. Meta, {{marka}}'nın görevlendirdiği bir alt işleyen değildir; şeffaflık için listelenmiştir. |

## 5. Yalnızca web sitesi ziyaretçileri için

| Sağlayıcı | Konum | Hizmet | Ne zaman |
|---|---|---|---|
| Google LLC — Google Analytics, Google Ads | Amerika Birleşik Devletleri | {{web}} web sitesinde kullanım ve reklam dönüşümü ölçümü | Yalnızca çerezlere onay veren ziyaretçiler için. Panelde ve Müşteri Verisinde kullanılmaz. Ayrıntılar: [Çerez Politikası](/yasal/cerez-politikasi) |

## 6. Yurt dışına aktarım

Yurt dışında bulunan alt işleyenlere aktarımlar, KVKK'nın 9. maddesi uyarınca Kişisel Verileri Koruma Kurulu'nun ilan ettiği standart sözleşmeler, GDPR'nin uygulandığı hâllerde Avrupa Komisyonu'nun standart sözleşme hükümleri ile yapılır. Ayrıntılar [Veri İşleme Sözleşmesi](/yasal/veri-isleme-sozlesmesi)'nin 10. maddesinde yer alır.

## 7. Değişikliklerin bildirilmesi

- Yeni bir alt işleyen eklemeden veya mevcut bir alt işleyeni değiştirmeden **en az 30 gün** önce Müşterilerimizi Sahip (Owner) e-posta adresine veya panel bildirimiyle bilgilendiririz ve bu sayfayı güncelleriz.
- Müşteri, bildirimden itibaren 30 gün içinde veri korumaya ilişkin makul gerekçelerle {{kvkkEposta}} adresine yazılı olarak itiraz edebilir. İtirazın sonuçları [Veri İşleme Sözleşmesi](/yasal/veri-isleme-sozlesmesi)'nin 8.3. maddesinde düzenlenmiştir.
- Bir alt işleyenin kaldırılması veya mevcut bir alt işleyenin işlediği veri kapsamının daraltılması için önceden bildirim yapılmaz; bu sayfa güncellenir.

Sorularınız için: {{kvkkEposta}}
