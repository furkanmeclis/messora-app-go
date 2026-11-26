# Phase 16C — Teknik analiz ve uygulama planı

Durum: **onaylandı, uygulamada** (plan ve kararlar 2026-11-25). Bu belge Phase 16C gereksinim dokümanının mevcut kodla eşlenmesi, Meta ve AI sağlayıcılarının bugünkü gerçek yetenekleri, mimari riskler ve iş paketlerinin sırasıdır. Her iş paketi bitince bu belgedeki durum sütunu güncellenir; tamamlanmayan iş tamamlandı olarak işaretlenmez.

---

## 1. Doğrulanmış dış gerçekler (2026-11-25)

Kaynaklar resmi dokümanlardır (Meta: `developers.facebook.com/documentation/business-messaging/whatsapp/...`; sağlayıcıların kendi API ve fiyat sayfaları). "Belirsiz" işaretli olanlar uygulama sırasında Meta sözleşme testiyle veya canlı hesapta doğrulanır.

### 1.1 Meta — fiyatlandırma ve maliyet
- **Mesaj başı fiyatlandırma (PMP)** 2025-07-01'den beri geçerli. Kategoriler: `marketing`, `utility`, `authentication` (+`authentication-international`), `service`, `marketing_lite`, `referral_conversion` (ücretsiz giriş noktası), grup kategorileri.
- **2026-10-01 değişikliği (yürürlükte):** servis mesajları numara başına ayda 1.000 ücretsiz mesajdan sonra **ücretli**; açık pencere içindeki utility şablonları **yeniden ücretli**. 2025 kurallarına göre yapılmış her tahmin bugün yanlıştır.
- Durum webhook'undaki `pricing` nesnesi: `type` (`regular` = ücretli, `free_customer_service`, `free_entry_point`, …), `category`, `pricing_model`, `subtype`. **Tutar yoktur**; `billable` kullanımdan kalkıyor.
- **Fiyat listesi API'si yok.** Meta para birimi bazında CSV/PDF fiyat kartı yayımlar (47 pazar, çeyrek başlarında değişir). Fiyat tablosunu biz tutarız.
- **Gerçekleşen maliyet:** WABA üzerinde `pricing_analytics` (yarım saat / gün / ay; ülke, numara, kategori, tür kırılımı; `cost` "yaklaşık" ve hesabın para biriminde). Kampanya kırılımı yoktur → kampanya maliyeti mesaj başı hesaplanır, toplamlar `pricing_analytics` ile günlük uzlaştırılır.

### 1.2 Meta — hız ve limitler
- Numara başına varsayılan **80 mesaj/sn** (gelen + giden birlikte); koşullar sağlanırsa otomatik 1.000'e çıkar. Güncel değer `GET /{phone-id}?fields=throughput`; yükseltme sırasında 131057.
- **Mesajlaşma limiti artık portföy seviyesinde** (2025-10-07): 24 saat içinde pencere dışında mesaj atılan tekil kullanıcı sayısı; 250 → 2K → 10K → 100K → sınırsız. Alan: `whatsapp_business_manager_messaging_limit` (`messaging_limit_tier` kullanımdan kalktı). Kalite düşüşü artık limiti düşürmüyor.
- **Kullanıcı başı hız:** aynı kullanıcıya 6 sn'de 1 mesaj (kısa süreli 45'e kadar patlama); aşımda 131056, `4^n` sn bekle.
- Yönetim API'si: WABA başına saatte 200 / aktif WABA'da 5.000 çağrı; `X-Business-Use-Case-Usage` başlığı.
- Hata sınıfları:
  - Tekrar denenir (backoff): 130429, 131056, 80007, 80008, 4, 1, 2, 131000, 131016, 133004, 131057.
  - **131049** (sağlıklı ekosistem / kullanıcı başı pazarlama limiti): **en az 24 saat** sonra; erken tekrar durumu kötüleştirir. +1 (ABD) numaralarına pazarlama şablonu şu an hiç teslim edilmiyor.
  - Kişi için kalıcı: **131050** (kullanıcı bu işletmeden pazarlamayı durdurdu), **131026** (ulaşılamıyor).
  - Önce düzelt: 131047 (pencere dışı), 132015/132016 (şablon duraklatıldı/kapatıldı), 131042 (ödeme), 133010.
  - Hesap seviyesi: 131048 (spam hız sınırı), 131031, 368 → kampanya durur, yönetici uyarılır.

### 1.3 Meta — spam ve kalite sinyalleri
- **Meta, belirli bir kullanıcının işletmeyi engellediğini veya spam bildirdiğini API ya da webhook ile vermiyor.** Yalnızca toplu sinyaller var: numara `quality_rating`, şablon `quality_score` + `message_template_quality_update`, şablon duraklatma (`message_template_status_update`), `account_update` olayları.
- **Kullanıcı başı tek sinyal:** `user_preferences` webhook'u — kullanıcı WhatsApp'ta bu işletmeden pazarlamayı "Durdur/Devam" yaptığında (`marketing_messages`, `stop|resume`). Bu durumdaki kullanıcıya pazarlama gönderimi 131050 ile düşer.
- `template_analytics`: gönderildi/teslim/okundu/tıklama ve harcama (günlük; AB ve Japonya işletmelerinde yok; bir kez açılınca kapatılamaz).
- **Sonuç:** "Spam bildiren müşteriler" hedef kitlesi **yapılmayacak** (veri yok). "Spam oranı" metrikleri Meta'nın toplu kalite sinyalleri + bizim hesapladığımız 131048/131049/131050 oranları + abonelikten çıkış oranı olarak, kaynağı etiketlenerek sunulur.

### 1.4 Meta — şablon bileşenleri
- Başlık: TEXT, IMAGE, VIDEO, DOCUMENT, LOCATION. Gövde 1.024, altbilgi 60 karakter.
- Butonlar (toplam ≤10): QUICK_REPLY (≤10), URL (≤2, tek değişken sonda), PHONE_NUMBER (≤1), COPY_CODE (≤1), VOICE_CALL, OTP (COPY_CODE / ONE_TAP / ZERO_TAP, yalnızca authentication), FLOW, MPM/SPM/CATALOG (katalog gerektirir). Quick reply butonları tek blok halinde olmalı.
- **Carousel** (yalnızca marketing): 2–10 kart, her kartta görsel/video ve ≤2 buton, tüm kartlar aynı yapıda.
- **Süreli teklif (LTO)** — gerçek Meta bileşeni: yalnızca MARKETING; altbilgi yok; başlık yalnızca görsel/video; gövde ≤600; teklif metni ≤16; kupon ≤15; butonlar `copy_code` + `url`; bitiş zamanı **gönderimde** `expiration_time_ms` ile verilir; sayaç metni özelleştirilemez; WhatsApp web/masaüstü "desteklenmiyor" gösterir.

### 1.5 Meta — politika
- **AI (Şartlar 4.7, 2026-09-23):** LLM/genel amaçlı AI asistanını **ana işlev** olarak sunmak yasak; işletmenin kendi müşterisine hizmet eden AI (destek, satış) serbest. Platform verisiyle başka bir AI modeli eğitmek yasak → **AI modülü sağlayıcılara "eğitimde kullanma" ayarıyla çağrılır, sohbet verisi hiçbir modelin eğitimine gönderilmez.**
- **İş mesajlaşma politikası:** otomasyon serbest ama "hızlı ve açık" bir insana devir yolu zorunlu → AI profillerinde devir kapatılamaz.
- **İzin:** kişiye yalnızca numarasını verip bu işletmeden mesaj almayı kabul ettiyse yazılabilir; WhatsApp içi veya dışı her türlü "durdur" talebine uyulmalı. Meta kategori bazında (pazarlama/servis) izin öneriyor.
- **24 saat penceresi:** yalnızca kullanıcının mesajı veya araması pencereyi açar/yeniler; işletme mesajı uzatmaz. Pencereyi açık tutmaya yönelik mesajı açıkça yasaklayan madde bulunamadı (belirsiz); kalite ve spam yaptırımları geçerli.

### 1.6 AI sağlayıcıları
- **Hiçbiri fiyatı API'den vermiyor.** Model listeleri: Anthropic (`max_input_tokens`, `max_tokens`, `capabilities`), Gemini (`inputTokenLimit`, `outputTokenLimit`), DeepSeek (`context_window`, `max_output_tokens`), OpenAI (yalnızca kimlik).
- Fiyat kuralları sağlayıcıya özgü: DeepSeek **saat dilimine göre** (yoğun saatlerde 2x); Gemini'de **ilan edilmiş zamlar** (2026-12-01, 2027-01-01); Claude Haiku 5.5'te 100K token eşiği; OpenAI'da 272K eşiği ve önbellek yazma ücreti; önbellek okuma/yazma ayrı fiyatlı.
- Anahtar doğrulama: OpenAI/Anthropic/Gemini `GET models` (ücretsiz), DeepSeek `GET /user/balance` (bakiyeyi de verir). Gemini'de 200 yanıtı çıkarımın açık olduğunu garanti etmez.
- Go SDK'ları: `anthropic-sdk-go/v2` v2.15.0, `openai-go/v3` v3.107.0 (DeepSeek de bununla, base URL ile), `google.golang.org/genai` v1.79.1.

---

## 2. Mevcut kodla eşleme ve mimari riskler

Ayrıntılı envanter: kod haritası (2026-11-25). Gereksinimleri doğrudan etkileyen bulgular:

| # | Bulgu | Risk | Öneri (16C'de yapılır) |
|---|---|---|---|
| R1 | Durum webhook'undaki `pricing` ve `conversation` alanları **atılıyor**; maliyet kolonu yok | Maliyet, tahmin-gerçek karşılaştırması ve rapor imkânsız | `messages`'a `pricing_type`, `pricing_category`, `billable`, `cost_estimate`, `cost_currency`, `recipient_country` kolonları; fiyat kartı tablosu; `pricing_analytics` ile günlük uzlaştırma |
| R2 | Gönderim hızı numara başına **koda gömülü 80/sn**; kampanyalar arası paylaşım/öncelik yok; kampanya gönderimleri gelen kutusu cevaplarıyla aynı `critical` kuyruğunda | Gelen kutusu cevapları kampanya yükü altında gecikir; 1.000/sn numaralar boşa kullanılmaz; adil paylaşım yok | Numara başına merkezi **gönderim zamanlayıcısı**: Meta'dan okunan `throughput`, AIMD, kampanyalar arasında ağırlıklı adil paylaşım (öncelik), insan/API mesajlarına ayrılmış kapasite; kampanyalar `campaigns` kuyruğuna |
| R3 | Portföy mesajlaşma limiti (24 saatte tekil kullanıcı) **hiç uygulanmıyor**; dokümandaki `paused_limit` yazılmamış | Limit aşımında Meta toplu reddeder, kalite düşer | Portföy başına 24 saatlik kayan tekil kullanıcı sayacı; limit dolunca kampanya `paused_limit`, otomatik devam |
| R4 | 131049 tekrar denemesi 6 saat ve bu sürede kampanya penceresinde yer tutuyor; 131050/131026 genel "kalıcı" | Yanlış zamanda tekrar = kalite düşüşü; durdurulmuş kullanıcıya tekrar tekrar gönderim | 131049 → ≥24 saat ve pencere dışı; 131050 → kişinin pazarlama izni "Meta'da durdurdu"; 131026 → numara "ulaşılamıyor" işareti; başarısızlık sebebi sınıfı alanı |
| R5 | Anahtar kelimeyle abonelikten çıkış ve `user_preferences` webhook'u yok; izin tek boyutlu (`opted_out_at`) | Yasal/politika riski; 6. bölüm karşılanamaz | Kategori bazlı izin modeli (pazarlama / servis), durum makinesi, kaynaklı `consent_events`, anahtar kelime algılama, preset otomasyonlar |
| R6 | `inbox:all` kanalı temsilcinin göremediği sohbetlerin kimlik/durum bilgisini de yayınlıyor; kampanya okundu/teslim bildirimleri gelen kutusunu yeniliyor | Yetki sızıntısı; büyük kampanyada panel takılır | Kullanıcı başı kanallar (`personal:#<uid>`) üzerinden yetkiye göre yönlendirme; kampanya durumları yalnızca kampanya kanalına |
| R7 | Centrifugo geçmişi 2 dk, bellek motoru, tek node; istemcide sıra/tekrar kontrolü yok, her olayda sayfa yeniden yükleniyor | Kopmada olay kaybı → tam yenileme; "polling yok" hedefi kırılgan | Olay zarfı (`v`, `seq`, `ts`, `type`, `id`); istemcide tekrar eleme ve sıra; geçmiş TTL'i 10 dk; kurtarılamazsa yalnızca ilgili veriyi yeniden çeken **senkron uç noktası**; Redis motoru seçeneği (çoklu node) |
| R8 | Tenant kavramı yok (tüm benzersizlikler global) | 16.2 gereksinimi 16C'de gerçek anlamda karşılanamaz | Bkz. §4 karar 1 |
| R9 | İstatistikler her yayında `count(*)` | Analitik, segment kırılımı ve zaman serisi ölçeklenmez | Alıcı başı durum kolonları (teslim/okundu/cevap/sonuç) + kampanya bazlı artımlı sayaçlar; kırılımlar tek sorguda alıcı tablosundan |
| R10 | Kampanya başlat/duraklat/devam/iptal denetim kaydına yazılmıyor | 16.4 ihlali | Tüm kampanya ve AI kritik işlemleri audit'e |
| R11 | Ölü mektup yalnızca asynq "archived"; görünür değil | Başarısız iş fark edilmez | `failed_jobs` görünümü + operasyon merkezinde uyarı |
| R12 | E2E yığınında Meta mock'u yok; testler DB'ye tohumluyor | Gerçek uçtan uca gönderim E2E'de doğrulanmıyor | Meta mock'unu ayrı bir servis olarak E2E yığınına ekle (pricing, user_preferences, quality webhook'ları dahil) |
| R13 | ARCHITECTURE.md'de olmayan şeyler anlatılıyor (kampanya kuyruğu, `throughput_mps`, şablon senkron işi, `unknown` zaman aşımı) | Yanlış varsayımla geliştirme | İlgili paketlerde yazılır veya dokümandan çıkarılır |

---

## 3. İş paketleri ve sıra

Bağımlılık sırası: **P0 → P1 → P2 → P3 → P4 → P5 → P6 → P7 → P8 → P9 → P10**. Her paket kendi testleriyle `main`'e birleşir (uzun ömürlü dal yok); 16C tamamlanınca `0.12.0` yayınlanır. Paketler arası API sözleşmeleri OpenAPI'de önce yazılır.

### P0 — Altyapı temeli (gerçek zamanlı, kuyruk, maliyet yakalama)
- **Olaylar:** standart olay adları (`<alan>.<olay>`: `conversation.handoff`, `automation.run.updated`, `campaign.progress`, `ai.status`…), sürümlü zarf, kullanıcı başı kanallarla yetkiye göre yönlendirme (R6), istemcide sıra/tekrar eleme, kurtarılamayan kopmada `GET /realtime/sync?since=` ile hedefli yeniden senkron (R7), geçmiş TTL'i 10 dk. Kritik olaylar zaten outbox ile.
- **Kuyruk:** kampanya gönderimleri `campaigns` kuyruğuna; AI işleri için ayrı `ai` kuyruğu; ölü iş görünümü (R11).
- **Meta webhook abonelikleri:** `user_preferences`, `phone_number_quality_update`, `account_update`, `message_template_quality_update`, `business_capability_update` işlenir; Meta mock'u bunları üretir.
- **Maliyet yakalama:** `pricing` alanları mesaja yazılır (R1); fiyat kartı tablosu (`meta_rates`: pazar, kategori, para birimi, birim fiyat, hacim kademesi, geçerlilik tarihleri, kaynak) + CSV içe aktarma; `pricing_analytics` günlük uzlaştırma işi.
- **Numara bilgisi senkronu:** `throughput`, `quality_rating`, portföy mesajlaşma limiti periyodik okunur.
- **Kabul:** yetkisiz temsilci başka sohbetin olayını alamaz (test); kopma sonrası kaçan olaylar senkronla gelir (E2E); webhook'tan gelen fiyat bilgisi mesajda (entegrasyon).

### P1 — Abonelik, izin ve gönderim engelleme (Bölüm 6, kabul 11)
- Kişi başı **kategori bazlı izin**: `marketing` ve `service` için `opted_in | unknown | opt_out_requested | opted_out | resubscribed`; ayrıca Meta kaynaklı `meta_marketing_stopped` (user_preferences / 131050) ve `undeliverable` (131026).
- Anahtar kelime algılama (TR/EN varsayılan liste, firma düzenleyebilir; normalize edilmiş eşleşme; belirsiz ifadede "çıkmak istiyor" durumu + temsilci onayı).
- Gönderim kuralları üç sınıf: **kesin engel** (opted_out, Meta durdurdu, ulaşılamıyor — aşılamaz, API dahil); **firma tercihi** (ör. "izni bilinmeyenlere pazarlama"); **belirsiz** (uyarı + gerekçe). Kontrol hem kampanya oluşturmada hem **gönderim anında** tekrar yapılır.
- Preset otomasyonlar (P8'de kütüphaneye girer): çıkış talebini algıla → onay iste/çıkar → durum güncelle → teşekkür → temsilciye bildir; yeniden izin.
- **Kabul:** opted_out kişiye kampanya, API ve otomasyonla gönderim reddedilir (entegrasyon); `user_preferences stop` sonrası pazarlama engellenir, servis mesajı engellenmez.

### P2 — Kampanya hedefleme ve gönderim motoru (3.1, 3.2, 5.2, 14.2; kabul 4, 5, 10)
- **Merkezi gönderim zamanlayıcısı** (R2): numara başına Meta `throughput` tavanı, AIMD, kampanyalar arası ağırlıklı adil paylaşım (öncelik: yüksek/normal/düşük), insan ve API mesajlarına ayrılmış pay, 131056 için kullanıcı başı bekleme; portföy limiti sayacı ve `paused_limit` (R3).
- Hata sınıflandırması (R4) ve alıcı başı **başarısızlık sınıfı**: `rejected_by_meta`, `temporary`, `permanent`, `user_stopped_marketing`, `undeliverable`, `ecosystem_limit`, `blocked_by_consent`, `cancelled`.
- **Yeni hedef kitle: "önceki kampanyada ulaşılamayanlar"** — kampanya seçilir, sınıflar tek tek seçilir (ör. geçici hata + ekosistem limiti dahil, Meta'da durduranlar hariç — onlar zaten kesin engel), mükerrer kişi engeli, sınıf bazlı özet.
- Duraklatma/devam: durdurma sebebi, zamanı ve o anki metrik anlık görüntüsü saklanır; devam öncesi özet; yapılandırma düzenlenebilir; başarıyla gönderilmişlere tekrar gönderim yok (mevcut UNIQUE + at-most-once korunur, testle).
- **Kabul:** iki eşzamanlı kampanya tek numarada tavanı aşmadan ve önceliğe göre paylaşır (entegrasyon, mock throughput); devam etmede çift gönderim yok; yeni kitle doğru sınıflarla oluşur.

### P3 — Kampanya analitiği, tahmin ve rapor (3.3, 3.4, Bölüm 4; kabul 6, 7, 8)
- Alıcı başı durum kolonları (gönderildi/teslim/okundu/cevapladı/sonuç/maliyet) ve artımlı sayaçlar (R9).
- **Olumlu/olumsuz dönüş:** (a) şablonun quick reply butonlarına firma "olumlu/olumsuz" etiketi verir, (b) AI açıksa cevap sınıflandırması (P7). Her iki kaynak da "sistem hesaplaması" olarak etiketlenir.
- **Ön analiz ekranı:** toplam / uygun / engelli (sebep kırılımıyla), tahmini süre (gerçek throughput + portföy limiti + diğer kampanyalar), tahmini bitiş, tahmini Meta maliyeti (ülke × kategori × fiyat kartı, ücretsiz pencere/servis kotası dikkate alınarak), tahmini teslim/etkileşim/olumlu dönüş (yalnızca yeterli geçmiş varsa — eşik: aynı kategoride en az 3 kampanya ve 500 alıcı; değilse "yeterli veri yok"). Tahminler kampanyaya **sabitlenir** (snapshot).
- Detay: tahmin vs gerçek (aynı metrik tanımları), Meta metrikleri ile sistem metrikleri ayrı etiketli; spam/kalite bölümü (§1.3).
- **Alıcı tablosu:** sunucu taraflı sayfalama, arama, sıralama, durum ve gelişmiş filtre, toplu seçim, CSV/XLSX dışa aktarma (akışla, büyük kampanyada bellek şişmeden).
- Kırılımlar: grup, segment, etiket, kişi tipi (yeni `contact_type` alanı) ve özel alan bazında teslim/okunma/cevap/dönüş/maliyet.
- **Rapor dışa aktarma:** XLSX (çok sayfalı: özet, maliyet, teslimat, alıcılar, segment, etiket, tahmin-gerçek) + PDF özet.
- **Spam koruması (5.1):** firma ayarı eşikler (131048 görülürse anında; 131049/131050 oranı, çıkış oranı, numara kalitesinin düşmesi, şablon kalitesi RED/duraklatma) → kampanya otomatik durur, sebep kaydedilir.
- **Kabul:** tamamlanan kampanyada tahmin-gerçek tablosu; filtre + dışa aktarma E2E; eşik aşımında otomatik durdurma (entegrasyon, mock kalite webhook'u).

### P4 — Şablon oluşturucu ve süreli teklifler (Bölüm 8; kabul 12, 13)
- Sadeleştirilmiş oluşturucu, bileşen bazlı doğrulama (§1.4 limitleri backend'de de), WhatsApp görünümüne yakın önizleme, değişken yönetimi, Meta durum takibi (gönderildi/onaylandı/reddedildi + sebep, kalite).
- Eklenen bileşenler: LOCATION başlık, COPY_CODE, VOICE_CALL, OTP (COPY_CODE / ONE_TAP / ZERO_TAP), FLOW (mevcut Flow kimliğiyle; Flow oluşturucu kapsam dışı), **carousel**, **LTO**. Katalog butonları (MPM/SPM/CATALOG) yalnızca bağlı katalog varsa; katalog yönetimi kapsam dışı — (kapsam dışı, §4).
- Gönderim tarafı (kampanya, gelen kutusu, otomasyon) yeni bileşen parametrelerini destekler (kupon kodu, `expiration_time_ms`, carousel kart parametreleri).
- **Teklif nesnesi:** başlık, açıklama, indirim, başlangıç/bitiş, görsel, ürün/hizmet, CTA. Meta LTO şablonuyla eşlenir (sayaç Meta'da); Meta'nın desteklemediği alanlar (ör. başlangıç tarihi, ürün) sistemde tutulur ve alternatif olarak normal görselli şablona eşlenir. Bitişte teklif `expired` olur, bağlı zamanlanmış kampanyalar uyarı verir/başlamaz.
- **Kabul:** LTO ve carousel şablonu oluşturulup mock'a doğru JSON'la gider ve gönderilir (sözleşme testi).

### P5 — AI sağlayıcıları ve AI profilleri (Bölüm 10.1, 11; kabul 15, 16)
- Sağlayıcı bağlantıları (OpenAI, Anthropic, Gemini, DeepSeek): anahtar mevcut `crypto.Sealer` ile şifreli, frontend'e asla gönderilmez (yalnızca son 4 karakter), doğrulama, bağlantı durumu, birden çok sağlayıcı.
- Model kataloğu senkronu (sağlayıcı API'si + yetenek alanları); **fiyat tablosu** (`ai_model_prices`: girdi, önbellek okuma, önbellek yazma, çıktı, eşik/saat kuralları, para birimi, kaynak, geçerlilik) — ilk veri MIT lisanslı topluluk listesinden, yönetici onaylı; fiyat değişse de geçmiş maliyet değişmez.
- Tek arayüz (`ai.Client`): sohbet, araç çağrısı, yapılandırılmış çıktı, kullanım ve maliyet; Anthropic ve Gemini resmi SDK, OpenAI ve DeepSeek `openai-go` ile. 429/kota ayrımı, sağlayıcıya özgü bekleme.
- **AI profili:** ad, bağlı numara (numara başına bir profil), sistem promptu (tam sayfa editör), model (firma varsayılanını ezebilir), otomatik cevap, devir ayarları, bilgi toplama ayarları, profil oluşturma yetkisi, izinli otomasyonlar, güven eşiği, **AI olduğunu açıklama politikası**, çalışma durumu.
- Firma geneli AI anahtarı (kapalıysa hiçbir AI işi çalışmaz).
- **Kabul:** anahtar şifreli saklanır ve API yanıtlarında hiç görünmez (test); profil numaraya bağlanır; kapalı modülde AI düğümü anlamlı hata verir.

### P6 — Sohbet zekâsı ve AI devri (Bölüm 2, 10.2–10.4; kabul 1, 2, 3, 19)
- Sohbetin **sorumlusu**: `owner_type human|ai|none`, `ai_mode active|off|suggest|auto_reply|awaiting_approval`; sohbet bazlı ve kişi bazlı AI kapatma.
- Devir işlemleri (insan → AI, AI → insan, temsilci değişikliği) açık durum makinesiyle; **iç notlar** (`conversation_notes`, müşteriye asla gitmez; AI bağlamına ayrı rol olarak girer — testle).
- AI cevap döngüsü: gelen mesaj → `ai` kuyruğu → bağlam (son mesajlar, iç notlar, kişi alanları, profil promptu) → yapılandırılmış çıktı (`reply`, `confidence`, `intent`, `escalate`, `extracted_fields`) → yetkili aksiyonlar → gönderim (pencere ve izin kontrolüyle). "Öneri" modunda temsilciye taslak olarak gelir.
- **Escalation:** müşteri insan istedi / cevap yok / güven eşiğin altı / kritik niyet / yetki dışı aksiyon → temsilciye özet, ihtiyaç, sebep, önerilen aksiyon ve toplanan bilgilerle devir.
- **Aktivite zaman çizelgesi:** devirler, temsilci değişiklikleri, AI aksiyonları, iç notlar, otomasyon çalışmaları (başlatan, durum, adımlar, hata, bitiş), mesaj durumları, escalation — tek `conversation_activity` akışı, canlı.
- **Kabul:** devir her iki yönde (entegrasyon + E2E); iç not müşteri mesajlarına sızmaz (test, gönderilen yükte not metni yok); escalation özetiyle temsilciye düşer; aktivite canlı güncellenir (E2E).

### P7 — AI müşteri zekâsı ve maliyet takibi (Bölüm 12, 13; kabul 17, 18)
- Kayıtsız numaradan gelen mesajda AI karşılama ve bilgi toplama (firma ve profil bazında açılıp kapanır); toplanacak alanlar yapılandırılabilir (özel alanlar dahil); alan güncelleme yetkisi ayrı; toplanan bilgi "AI tarafından toplandı, doğrulanmadı" olarak işaretlenir; veri minimizasyonu (yalnızca tanımlı alanlar).
- **AI kullanım kaydı** (`ai_usage`): istek başı sağlayıcı, model, token türleri, maliyet (istek anındaki fiyatla), süre, sonuç, ilişkili sohbet/kampanya.
- **Kampanyaya atama:** sohbet, kampanya mesajına cevaben açıldıysa o kampanyaya; bir sonraki kampanyanın o kişiye ulaştığı ana kadar (ve en fazla ayarlanabilir bir süre) etkileşim o kampanyaya yazılır; her AI isteği **en fazla bir** kampanyaya atanır; atanamayanlar "kampanya dışı" görünür.
- Metrikler: toplam istek, cevap, token, model/sağlayıcı kırılımı, toplam ve kampanya bazlı maliyet, başarısız istek, ortalama süre, insana devir oranı. Kampanya maliyetinde Meta + AI = toplam operasyon maliyeti.
- **Kabul:** aynı AI isteği iki kampanyaya yazılmaz (test); bilinmeyen numaradan profil oluşur (entegrasyon, sahte sağlayıcı).

### P8 — Otomasyon motoru ve preset kütüphanesi (Bölüm 9; kabul 14)
- Yeni tetikleyiciler: **pencere kapanmadan X önce** (zamanlayıcı ile, pencere gerçekten açıksa ve son mesaj müşteriden geldiyse; aynı pencere için bir kez), cevapsız müşteri (X süre), kampanya cevabı, olumlu/olumsuz dönüş, abonelik talebi, AI escalation.
- Yeni adımlar: AI ile mesaj oluştur, mesajı analiz et, niyet tespit et, bilgi çıkar, segment belirle, otomatik cevap, temsilciye yönlendir, müşteri profili oluştur; AI'ı devret / insana devret; iç not ekle. AI adımları çalışmadan önce modül ve profil izni kontrolü.
- Adım bazlı tekrar politikası ve idempotent anahtarlar; çalışan otomasyon durumları sohbete canlı yayın (P6).
- **Preset kütüphanesi:** belgedeki 11 senaryo; tek tıkla eklenir, eklenen kopya serbestçe düzenlenir.
- **Kabul:** pencere tetikleyicisi yalnızca doğru koşulda ve bir kez çalışır (entegrasyon, sahte saat).

### P9 — Operasyon merkezi (Bölüm 14; kabul 20)
- Genel bakış: aktif/bekleyen/tamamlanan/duran kampanyalar, ilerleme, tahmini bitiş, gerçekleşen maliyet, spam/hata uyarıları; gönderim sırası ve kapasite paylaşımı (P2); hızlı aksiyonlar (durdur/devam/detay/sohbet aç); escalation talepleri; atanmayı bekleyenler; hatalı otomasyonlar ve ölü işler; son gelen/giden mesajlar; cevap bekleyenler; AI'daki ve insan bekleyen sohbetler; aktif otomasyonlar. Hepsi Centrifugo ile canlı. Sistem sağlığı ayrı sekmeye.
- **Kabul:** kampanya ilerlemesi ve yeni escalation sayfa yenilemeden görünür (E2E).

### P10 — Public API dokümantasyonu (Bölüm 15; kabul 21)
- Public uç nokta envanteri OpenAPI'deki güvenlik şemalarından otomatik çıkarılır (panel-only uç noktalar ayrı bir "internal" etiketle gizlenir).
- Her uç nokta için açıklama, yetki (scope), örnek istek/yanıt, hata kodları, rate limit; rate limit başlıkları (`RateLimit-*`) eklenir.
- Arayüz: Scalar (§4). Doküman `openapi.yaml`'dan üretilir; CI'da "spec ile kod uyumlu" kontrolü zaten var (oapi-codegen), buna "her public uç noktanın açıklaması ve örneği var" lint'i eklenir.
- **Kabul:** dokümandaki uç nokta listesi sunucunun gerçek rotalarıyla birebir (otomatik test).

---

## 4. Kararlar (2026-11-25, ürün sahibi)

Aşağıdaki önerilerin hepsi kabul edildi: (1) yetki izolasyonu 16C'de, firmalar arası izolasyon Faz 17'de; (2) API dokümanı Scalar; (3) izni bilinmeyen kişiler için varsayılan "izin ver + uyar".


1. **Tenant izolasyonu (16.2, kabul 22).** Bugün kurulumun kendisi tek firma. Öneri: 16C'de **yetki izolasyonu** (temsilci görünürlüğü, kanal sızıntısı R6, API key scope'ları, kuyruk işlerinde aktör bağlamı) tam yapılır ve testlenir; **firmalar arası izolasyon** (tenant_id + RLS + tenant başı AI anahtarları) Faz 17'de tüm tablolarla birlikte. 16C'de yazılan yeni tablolar ve işler Faz 17'ye hazır biçimde (tek noktadan kapsam) yazılır.
2. **API doküman arayüzü.** Öneri: **Scalar** — OpenAPI'den doğrudan üretim, arama, kod örnekleri, mobil uyumlu; JS paketi uygulamanın içinden servis edilir (CDN yok), `/docs/api` adresinde. Alternatif Fumadocs: Next.js + MDX, daha esnek ama ağır ve ayrı bakım.
3. **İzni bilinmeyen kişiler.** Mevcut kişilerin hepsi "izin durumu bilinmiyor" olarak başlayacak. Öneri: varsayılan **izin ver + uyarı** (firma ayarıyla "engelle"ye çevrilebilir); kesin engeller her durumda aşılamaz.
4. **Kapsam dışı bırakılacaklar** (veri/özellik Meta'da yok veya ayrı ürün): spam bildiren kişiye göre hedefleme (Meta vermiyor), Flow oluşturucu, katalog yönetimi.

## 5. Test stratejisi
- Birim: hata sınıflandırma, fiyat hesabı (saat/eşik/geçerlilik), zamanlayıcı paylaşımı, izin kuralları, tahmin.
- Entegrasyon (testcontainers + Meta mock + sahte AI sağlayıcı): her kabul kriteri en az bir test.
- Sözleşme: Meta istek JSON'ları (LTO, carousel, copy code) ve AI sağlayıcı istek/yanıt örnekleri.
- Olay testleri: yetki filtresi, sıra/tekrar, kopma sonrası senkron.
- Yetki testleri: her yeni uç nokta için rol matrisi.
- E2E: Meta mock'u yığına eklenir; devir, kampanya kitlesi, dışa aktarma, LTO, operasyon merkezi akışları.

## 6. Durum

| Paket | Durum |
|---|---|
| P0 Altyapı | ✅ tamamlandı (2026-11-25) — açık doğrulamalar: Meta `throughput` seviye adları ve `pricing_analytics` numara alanı gerçek hesapta teyit edilecek |
| P1 Abonelik ve izin | ✅ tamamlandı (2026-11-25) — kurallar ve kararlar: docs/CONSENT.md; açık: durum webhook'u ile gelen 131049 otomatik tekrar gönderilmez (P2 "ulaşılamayanlar" kitlesi) |
| P2 Gönderim motoru | ✅ tamamlandı (2026-11-25) — tasarım: ARCHITECTURE.md §6.3/§7 (numara başına GCRA şeritleri + ağırlıklı adil pay, `portfolio_reach` ile 24 saatlik tekil müşteri sayacı ve `paused_limit`, `campaign_pauses`, `previous_campaign` kitlesi, webhook 131049 tekrar gönderimi); açık: portföy birden çok WABA'yı kapsıyorsa yalnızca bağlı WABA sayılır |
| P3 Analitik ve rapor | ✅ tamamlandı (2026-11-26) — tanımlar ve kurallar: docs/CAMPAIGN_ANALYTICS.md (alıcı başı sonuç kolonları + tetikleyiciyle artımlı `campaign_counters` ve uzlaştırıcı, 7 günlük atama penceresi, quick reply eşlemesi, ön analiz ve `campaign_forecasts`, tahmin vs gerçek, alıcı tablosu/toplu işlem/`contact_ids` kitlesi, CSV/XLSX/PDF dışa aktarma işi, kırılımlar, kampanya koruması); AI sınıflandırıcısı P7 ile `ReplyClassifier`'a bağlandı; açık: kademeli fiyatlarda tahmin ayın ilk kademesiyle (üst sınır) yapılır, `free_entry_point` önceden bilinemediği için tahminde ücretli sayılır |
| P4 Şablon ve teklif | bekliyor |
| P5 AI sağlayıcı ve profil | ✅ tamamlandı (2026-11-25), bkz. docs/AI.md — açık doğrulamalar: gerçek hesaplarla yapılandırılmış çıktı şemasının (nullable tipler) Anthropic/Gemini tarafından kabulü, Gemini `toolUsePromptTokenCount` faturalaması, seed fiyatların yönetici onayı |
| P6 Sohbet zekâsı ve devir | ✅ tamamlandı (2026-11-25), bkz. docs/AI.md "Sohbet devri" — sorumlu/AI modu durum makinesi, iç notlar, AI cevap döngüsü (`ai:reply`, 3 sn debounce), escalation özetiyle, aktivite zaman çizelgesi ve otomasyon koşuları canlı. Açık: devir hedefi yalnızca havuz veya atanan kişi (round robin yok); "çevrimiçi" temsilci takibi yok, bildirim konuşmaları okuyabilen tüm aktif kullanıcılara gider; AI'ın `add_tag` vb. parametreli aksiyonları yalnızca kaydedilir (otomasyon ve alan güncelleme çalışır) |
| P7 AI müşteri zekâsı ve maliyet | ✅ tamamlandı (2026-11-26), bkz. docs/AI.md "Müşteri bilgisi toplama" ve "Maliyet ve kullanım" — bilinmeyen numarada profil oluşturma (`profile_status`, alan türleri, `contact_field_sources` ile kaynak/doğrulama, insan verisinin üzerine yazmama, hassas kategori yasağı, tamamlanınca devir), AI kullanımının tek kampanyaya atanması (insert anında + saatlik uzlaştırma), `GET /ai/usage/summary`, aylık bütçe (`ai_budget`), kampanya cevabı sınıflandırması (`ai:classify`). P3 ile birleştirildi: tek atama penceresi Kampanya koruması `attribution_days` (AI ayarındaki kopya kaldırıldı), sınıflandırma P3'ün `ReplyClassifier`'ı üzerinden çalışır ve sonucu alıcı satırına `reaction` (kaynak `ai`; manuel ve hızlı cevabın üzerine yazmaz) olarak yazar, `campaign_reply_classifications` iş kaydı olarak kalır; kampanyanın AI istek/token/maliyeti analiz merkezinin `ai_cost` alanında (ayrı `GET /campaigns/{id}/ai-usage` kaldırıldı). Açık: AI maliyeti (USD) ile Meta tahmini farklı para birimindeyse toplam gösterilmez (kur dönüşümü yok); kullanım sayfasında yalnızca hazır dönemler (7/30/90 gün, bu ay) |
| P8 Otomasyon ve presetler | ✅ tamamlandı (2026-11-26), bkz. docs/AUTOMATIONS.md — yeni tetikleyiciler (`window_closing` ve `no_reply` dakikalık tarayıcıyla, olay anahtarıyla pencere/sessizlik başına bir kez; `campaign_reply`, `reaction`, `ai_escalation`), AI adımları (`ai.Enabled` + profil `allowed_actions` kontrolü, kapalıyken `ai_disabled`, `ai_usage` purpose `automation` + run id), devir/iç not adımları, adım bazlı `retry {max, backoff}` (tükenince `retries_exhausted` + DLQ), `automation_sends` ile idempotent gönderim, 11 hazır otomasyon (gömülü, sürümlü; kurulum maddeleri dolmadan etkinleşmez) ve galeri. P3 ve P7 ile birleştirildi: tepki tetikleyicisi WhatsApp emojisinin yanında kampanya alıcısının tepki etiketiyle de çalışır (hızlı cevap eşlemesi, AI sınıflandırması, manuel etiket; kaynak `quick_reply`/`ai`/`manual`, etiketin yazıldığı işlemde), alıcı ve yön başına bir kez (`trigger.RecipientReactionKey`; kampanya mesajına emoji de aynı anahtarı kullanır); `campaign_reply` P3'ün atama kuralıyla (`attribution_days`, alıcının ilk cevabı) kampanyalar modülünden yükselir, 72 saat kuralı kaldırıldı; AI alan yazımı P7 kaynak deposundan geçer (`conversations.Service.SetFieldsWithSource`: `ai.MayAIWrite`, kaynak `ai` doğrulanmamış, `ai_usage_id` ve sohbetle; insan/import/API veya doğrulanmış değer korunur, adım günlüğünde `kept`), AI alan adımları `update_contact_fields` yerine profilin `may_update_fields` iznini ister (profil yoksa `ai_no_profile`), hassas alanlar kaydedilirken reddedilir. Açık: eşleme değişince geçmiş cevapların yeniden etiketlenmesi otomasyon başlatmaz; `ai_detect_intent` dallanması ardından gelen `var.intent` koşuluyla yapılır |
| P9 Operasyon merkezi | ✅ tamamlandı (2026-11-26), bkz. ARCHITECTURE.md §12 "Operasyon merkezi" — genel bakış sayfası operasyon merkezi oldu, sistem sağlığı "Sistem" sekmesinde; `GET /ops/overview` (yalnızca panel) bölümleri yetkiye göre süzer (kampanyalar, sohbetler görünürlük kuralıyla, saatlik mesaj grafiği, otomasyonlar, uyarılar); canlılık mevcut olaylar + `system:ops` üzerinde kimliksiz `ops.updated` ipuçları (veritabanı tetikleyicileri ve worker ölü iş kancası), bölüm bazlı yeniden okuma; hızlı aksiyonlar (durdur, önizlemeli devam, devral, bana ata). Açık: AI bütçesi aşımı ayrı ipucu üretmez; kampanya spam sinyali kampanya toplamıdır (zaman pencereli değil) |
| P10 API dokümantasyonu | ✅ tamamlandı (2026-11-25) — public spec `internal/platform/apidocs` ile `apiKey` kabul eden işlemlerden türetilir, `GET /api/v1/openapi.json` (anonim) ve `/docs/api` (Scalar, `@scalar/api-reference-react` 0.9.82, MIT, kendi origin'imizden; font/telemetri/AI/MCP kapalı); `RateLimit-*` başlıkları; envanter `docs/API.md` (`make api-docs`, CI `make api-docs-check`); lint + rota birebir testi (kabul 21). Not: Scalar ≥ 0.9.83 `@unhead/bundler` üzerinden lightningcss (MPL-2.0) getirdiği için lisans kontrolünü geçmiyor; yükseltme lisans kararı bekliyor |
