# Messora Google Ads (Arama Ağı) Lansman Planı

**Hazırlanma tarihi:** 28 Kasım 2026 · **Kapsam:** yalnızca Google Ads Arama (Search) kampanyaları, Türkiye, Türkçe, B2B KOBİ karar vericileri · **Ürün:** Messora, resmi WhatsApp Business Cloud API üzerinde çalışan ekip gelen kutusu, izinli kampanya, AI ajan, otomasyon, şablon oluşturucu, analitik ve public API/webhook paneli.

> **Okuma kılavuzu.** Kaynaklı bilgiler satır içi bağlantıyla verildi, tam liste `KAYNAKLAR.md` dosyasında. **"Tahmin"** etiketli her rakam bizim modelimizdir, ölçülmüş veri değildir. Türkiye'de WhatsApp API anahtar kelimeleri için kamuya açık, metodolojisi belli bir hacim ve TBM verisi **bulunamadı**. Bu yüzden anahtar kelime hacim ve TBM değerleri tahmindir. Hesap açıldığı gün Google Anahtar Kelime Planlayıcı ile değiştirilmelidir (§6.1). Fiyatlar + KDV'dir. Google bütçeleri ise Google'ın Türkiye "Yasal İşletim Maliyeti" ek ücreti ve KDV hariçtir (§12).

---

## 1. Yönetici özeti

- **Önerilen günlük bütçe: 1.500 TL/gün.** Bu, aylık en fazla yaklaşık **45.600 TL** ve 90 günde **136.500 TL** reklam harcaması demek. Yasal İşletim Maliyeti dahil toplam yaklaşık **142.600 TL**, KDV ayrıca. Alternatif senaryolar: **minimum test 750 TL/gün** (aylık ~22.800 TL) ve **büyüme 3.000 TL/gün** (aylık ~91.200 TL).
- **Baz senaryoda beklenen sonuç (tahmin):** 1.500 TL/gün ile ayda ~1.300 tıklama, ~52 deneme kaydı, ~10 demo talebi ve **~9 yeni ücretli müşteri** (aralık 2–27). Müşteri edinme maliyeti (CAC) ~5.400 TL. Ortalama müşterinin aylık brüt katkısı ~2.270 TL olduğu için geri ödeme süresi ~2,4 ay.
- **Kötü senaryo da göz önünde tutulmalı:** TBM 55 TL, tıklamadan denemeye %2 ve denemeden ücretliye %8 olursa CAC ~26.000 TL'ye çıkar ve geri ödeme ~11,5 ay olur. Plan bu yüzden 14, 30, 60 ve 90. günlerde ölçülebilir **karar kurallarına** bağlandı (§5). Bütçe ancak hedefler tutarsa artar.
- **Yapı:** 6 aktif kampanya var: Marka, WhatsApp Business API, Toplu Mesaj & Kampanya, Chatbot & AI, CRM & Ekip Gelen Kutusu, Sektör. Bunlara ek olarak duraklatılmış, opsiyonel bir Rakip kampanyası var. Toplam 16 reklam grubu, 82 anahtar kelime (sıralı/tam eşleme), kampanya başına 127 negatif kelime ve 32 RSA. Her grupta biri "WhatsApp" kelimesini içermeyen yedek reklam. Hepsi `google-ads-editor-import.csv` dosyasında.
- **Teklif stratejisi:** 1–2. haftalarda tavanlı Tıklamaları En Üst Düzeye Çıkarma ile başlanır. 3. haftadan itibaren Dönüşümleri En Üst Düzeye Çıkarma'ya geçilir. Kampanya başına ayda 30'dan fazla dönüşüm oluşunca Hedef EBM'ye (tCPA) geçilir.
- **Ölçüm, lansmanın ön şartı:** Dört dönüşüm kurulacak: `generate_lead` (demo/iletişim formu), `sign_up_click` (app.messora.tech/signup tıklaması, ikincil), uygulamadan `sign_up` (birincil) ve ödeme onayından `purchase`. Bunlara Gelişmiş Dönüşümler, Consent Mode v2 ve GA4 bağlantısı eşlik edecek. Denemenin ücretliye dönüşmesi daha sonra Data Manager üzerinden çevrimdışı olarak içe aktarılacak.
- **En büyük üç risk:**
  1. **"WhatsApp" markası.** Google, anahtar kelimede marka kullanımını kısıtlamıyor. Reklam metnindeki kullanım ise marka sahibi şikâyet ederse kısıtlanabilir. Bu yüzden her grupta marka geçmeyen yedek reklam hazır.
  2. **TBM belirsizliği.** Türk kaynaklarında yazılım TBM'si 11 TL ile 219 TL arasında değişiyor.
  3. **Rakip kampanyası.** Türkiye'de SMK m.7/3-(f), rakip markasının anahtar kelime olarak kullanılmasını marka hakkına tecavüz sayabiliyor. Bu nedenle kampanya duraklatılmış durumda ve hukuki görüş olmadan açılmamalı.

---

## 2. Hedefler ve KPI'lar

### 2.1 Birim ekonomisi varsayımları (tahmin)

| Varsayım | Değer | Dayanak |
|---|---|---|
| Plan karışımı (yeni müşteri) | %40 Başlangıç, %45 Büyüme, %15 Pro | Tahmin. Self-servis KOBİ ağırlığı |
| Ortalama aylık gelir (ARPA, + KDV hariç) | 0,40×1.290 + 0,45×2.990 + 0,15×6.490 ≈ **2.835 TL** | Plan fiyatları (`rakip-fiyat-analizi.md`) |
| Brüt marj (altyapı, AI, destek sonrası) | %80 → aylık brüt katkı ≈ **2.268 TL** | Tahmin. Mesaj Bakiyesi marjı dahil edilmedi (ihtiyatlı) |
| Aylık müşteri kaybı (churn) | kötü %7 · baz %5 · iyi %3,5 | Tahmin, KOBİ SaaS |
| Tıklama → deneme kaydı (`sign_up`) | kötü %2 · baz %4 · iyi %6 | SaaS'ta Google Arama medyan dönüşümü %5,1 ([Unbounce](https://unbounce.com/conversion-benchmark-report/saas-conversion-rate/)). Katı B2B SaaS ücretli arama için ~%1 ([Flint](https://www.flint.com/blog/landing-page-conversion-rate-statistics)). Türkiye "B2B Yazılım" dönüşüm oranı %3,1 ([Bravery](https://braverytechnology.com/tr/google-ads-istatistikleri/), metodolojisi yok) |
| Tıklama → demo talebi (`generate_lead`) | %0,4 · %0,8 · %1,2 | Tahmin |
| Deneme → ücretli (kart istemeyen deneme) | %8 · %12 · %18 | Kart istemeyen denemelerde %4–6 "iyi", %10–15 "çok iyi" ([ChartMogul](https://chartmogul.com/reports/saas-conversion-report/)). Opt-in denemede ortalama %18,2 (First Page Sage, [shno.co](https://www.shno.co/marketing-statistics/free-trial-conversion-statistics) üzerinden) |
| Demo → ücretli | %15 · %25 · %35 | Tahmin |
| Harmanlanmış TBM | 55 · 35 · 25 TL (büyüme senaryosunda +%20) | §6.1 |
| Google ek ücreti (Yasal İşletim Maliyeti) | %4,5 (2026), 1 Ocak 2027'den %2,25–2,5 | [roicool](https://www.roicool.com/en/blog/google-ads-costs-in-turkey), [Webtekno](https://webtekno.com/google-turkiye-2027-reklam-ek-ucret-orani-dusuyor-h228802.html) |

### 2.2 Hedef KPI'lar

| KPI | Hedef | Kırmızı çizgi (aksiyon) | Gerekçe |
|---|---|---|---|
| Deneme kaydı başı maliyet (`sign_up` CPA) | **≤ 1.000 TL** | > 2.000 TL (14 gün üst üste) | Baz model 914 TL. 6 aylık geri ödemeye izin veren üst sınır: 6 × 2.268 × %12 ≈ 1.630 TL |
| Demo talebi başı maliyet (`generate_lead` CPA) | **≤ 3.000 TL** | > 5.000 TL | 6 ay geri ödeme × %25 demo→ücretli ≈ 3.400 TL |
| Aktive deneme oranı (numara bağlanan / kayıt) | ≥ %35 | < %20 | Tahmin. Kalitesiz trafik göstergesi |
| Deneme → ücretli (kohort, 30 gün) | ≥ %12 | < %6 | Yukarıdaki varsayım |
| CAC (reklam + ek ücret) | ≤ 6.000 TL | > 13.600 TL (6 ay geri ödeme) | Brüt katkıya göre |
| Geri ödeme süresi | ≤ 3 ay | > 6 ay | |
| LTV/CAC | ≥ 3 | < 2 | Baz LTV = 2.268 / %5 ≈ 45.000 TL |
| TO (CTR), marka dışı | ≥ %4 | < %2 | Türkiye arama TO'su %3,8, "B2B Yazılım" %2,2 ([Bravery](https://braverytechnology.com/tr/google-ads-istatistikleri/)). Global ortalama %6,64 ([WordStream 2026](https://wordstream.com/blog/2026-google-ads-benchmarks)) |
| Arama gösterim payı (marka) | ≥ %90 | < %70 | |

---

## 3. Günlük bütçe önerisi: üç senaryo

Aylık tutarlar 30,4 günle hesaplandı (Google'ın aylık harcama sınırı günlük bütçe × 30,4). Google bir günde günlük bütçenin 2 katına kadar harcayabilir, ama ay toplamı bu sınırı aşmaz. 90 gün toplamı 91 günle hesaplandı. Tüm sonuç rakamları **tahmindir** ve aşağıdaki varsayım setinden üretildi (§2.1).

### 3.1 Bütçe ve maliyet

| Senaryo | Günlük | Aylık üst sınır | 90 gün | 90 gün + %4,5 ek ücret | Not |
|---|---|---|---|---|---|
| **A. Minimum test** | **750 TL** | 22.800 TL | 68.250 TL | ~71.300 TL | Yalnızca 4 kampanya: Marka, API, Toplu Mesaj, Chatbot. Hafta içi 08–21 |
| **B. Önerilen** | **1.500 TL** | 45.600 TL | 136.500 TL | ~142.600 TL | 6 kampanya, 7/24, gece ve hafta sonu düşük ağırlık |
| **C. Büyüme** | **3.000 TL** | 91.200 TL | 273.000 TL | ~285.300 TL | B'deki hedefler tuttuktan sonra. Rakip ve sektör genişlemesi dahil |

KDV: Fatura Google Ireland'dan gelirse KDV, sorumlu sıfatıyla 2 No.lu beyanname ile ödenir ve indirilir. Google Reklamcılık ve Pazarlama Ltd. Şti. faturasında ise KDV faturaya eklenir ([roicool](https://www.roicool.com/en/blog/google-ads-costs-in-turkey), [Vera Bilişim](https://verabilisim.com/google-reklam-kdv-2)). Her iki durumda da KDV mükellef için nakit akışı kalemidir, maliyet değildir. SMMM ile teyit edilmeli.

### 3.2 Beklenen sonuç aralıkları (aylık, tahmin)

| Senaryo / durum | TBM | Tık/gün | Tık/ay | Deneme/ay | Demo/ay | Yeni müşteri/ay | Deneme CPA | CAC | Geri ödeme | LTV/CAC | Yeni MRR |
|---|---|---|---|---|---|---|---|---|---|---|---|
| A · kötü | 55 | 14 | 415 | 8 | 1,7 | 0,9 | 2.874 | 26.125 | 11,5 ay | 1,2 | 2.586 |
| **A · baz** | 35 | 21 | 651 | 26 | 5,2 | **4,4** | 914 | 5.379 | 2,4 ay | 8,4 | 12.558 |
| A · iyi | 25 | 30 | 912 | 55 | 10,9 | 13,7 | 435 | 1.742 | 0,8 ay | 37 | 38.783 |
| B · kötü | 55 | 27 | 829 | 17 | 3,3 | 1,8 | 2.874 | 26.125 | 11,5 ay | 1,2 | 5.171 |
| **B · baz** | 35 | 43 | 1.303 | 52 | 10,4 | **8,9** | 914 | 5.379 | 2,4 ay | 8,4 | 25.116 |
| B · iyi | 25 | 60 | 1.824 | 109 | 21,9 | 27,4 | 435 | 1.742 | 0,8 ay | 37 | 77.566 |
| C · kötü | 66 | 45 | 1.382 | 28 | 5,5 | 3,0 | 3.448 | 31.350 | 13,8 ay | 1,0 | 8.618 |
| **C · baz** | 42 | 71 | 2.171 | 87 | 17,4 | **14,8** | 1.097 | 6.454 | 2,8 ay | 7,0 | 41.861 |
| C · iyi | 30 | 100 | 3.040 | 182 | 36,5 | 45,6 | 522 | 2.090 | 0,9 ay | 31 | 129.276 |

*CAC = (reklam harcaması × 1,045) / yeni müşteri. Geri ödeme = CAC / 2.268 TL. LTV = 2.268 TL / aylık churn. Yeni MRR = yeni müşteri × 2.835 TL (KDV hariç). Model betiği: `betikler/model.py`.*

### 3.3 Neden 1.500 TL/gün?

1. **Öğrenme için yeterli veri.** Akıllı teklif için kampanya başına ayda yaklaşık 30 dönüşüm, sektör pratiğinde iyi bir eşik olarak kabul ediliyor. Baz senaryoda 1.500 TL/gün ~52 deneme ve ~10 demo üretir. Yani dönüşümler 2–3 kampanyada yoğunlaştığında tCPA'ya geçilebilir. 750 TL/gün ile hesap toplamı ~26 denemede kalır, bu da tek bir kampanyada bile eşiğin altında demek. Minimum senaryo bu yüzden 4 kampanyayla sınırlı tutuldu.
2. **İstatistiksel okuma.** Bir anahtar kelimeyi durdurma kuralı "1,5 × hedef CPA harcayıp dönüşüm yok" (§5.3). 1.500 TL/gün ile ana gruplar bu eşiğe 2–3 haftada ulaşır. 750 TL/gün ile bu süre 5–6 haftaya uzar.
3. **Pazar hacmi tavanı.** Tahminimize göre çekirdek WhatsApp API ve CRM terimleri Türkiye'de birkaç bin ile on binler arasında aylık arama alıyor (§6.1, Ek A). 3.000 TL/gün büyük olasılıkla gösterim payı tavanına çarpar ve harcamayı daha az ilgili terimlere iter. Büyüme senaryosu bu yüzden ancak B'de "bütçe nedeniyle kaybedilen gösterim payı" > %20 ve CPA hedefte ise açılmalı.
4. **Risk sınırı.** Baz varsayımlarda B'nin 90 günlük yatırımı (~142.600 TL) ~27 yeni müşteri ve ~75.000 TL yeni MRR getirir. Kötü senaryoda bile 60. gün karar noktasında en fazla ~95.000 TL harcanmış olur.

### 3.4 Kampanyalara bütçe dağılımı (TL/gün)

| Kampanya | A (750) | **B (1.500)** | C (3.000) |
|---|---|---|---|
| Marka · Messora | 50 | **75** | 100 |
| WhatsApp Business API | 350 | **525** | 900 |
| Toplu Mesaj & Kampanya | 200 | **300** | 600 |
| Chatbot & AI | 150 | **225** | 450 |
| CRM & Ekip Gelen Kutusu | — (API'ye dahil) | **225** | 450 |
| Sektör | — | **150** | 300 |
| Rakip Alternatif (opsiyonel) | — | **0 (duraklatılmış)** | 200 |
| **Toplam** | **750** | **1.500** | **3.000** |

---

## 4. Hesap yapısı ve ayarlar

### 4.1 Kampanyalar ve reklam grupları

| Kampanya | Reklam grupları | Açılış sayfası |
|---|---|---|
| TR \| Search \| Marka \| Messora | Marka - Messora | `https://messora.tech/` |
| TR \| Search \| WA Business API | Business API - Genel · API - Fiyat ve Sağlayıcı · Kurumsal WhatsApp | `/lp/whatsapp-business-api`, fiyat grubu → `/fiyatlar` |
| TR \| Search \| Toplu Mesaj & Kampanya | Toplu Mesaj · Pazarlama ve Bildirim | `/lp/whatsapp-toplu-mesaj` |
| TR \| Search \| Chatbot & AI | Chatbot · Yapay Zeka Asistan | `/lp/whatsapp-chatbot` |
| TR \| Search \| CRM & Ekip Gelen Kutusu | WhatsApp CRM · Ekip Gelen Kutusu | `/lp/whatsapp-crm` |
| TR \| Search \| Sektör | E-ticaret · Sağlık ve Klinik · Turizm ve Otel · Eğitim · Emlak | `/cozumler/e-ticaret`, `/cozumler/saglik-ve-klinikler`, `/cozumler/turizm-ve-otel`, `/cozumler/egitim`, `/cozumler/emlak` |
| TR \| Search \| Rakip Alternatif (opsiyonel, **Paused**) | Rakip - Global Alternatif | `/fiyatlar` (ileride ayrı bir karşılaştırma sayfası) |

Yollar sitenin `website/src/data/routes.ts` dosyasındaki rotalarla eşleşiyor. **Site yayına girmeden reklam açılmamalı.** Google'ın hedef sayfa gereksinimleri çalışmayan veya "yapım aşamasında" sayfaları reddeder.

### 4.2 Eşleme türleri

- **Başlangıç:** sıralı eşleme ("…") ve en değerli terimlerde tam eşleme ([…]). **Geniş eşleme ve AI Max for Search lansmanda kapalı.** WhatsApp terimlerinin tüketici niyetiyle (indir, web, yedekleme, GB…) çok karıştığı bir alanda geniş eşleme bütçeyi hızla tüketir.
- **7. haftadan sonra:** tCPA stabil olan tek bir kampanyada (muhtemelen API), ayrı bir deney (Experiments) olarak geniş eşleme ve AI Max testi yapılır. Başarı kriteri: CPA en fazla +%15 artarken dönüşüm en az +%20 artmalı.
- **Sorgu yönlendirme:** Marka dışındaki tüm kampanyalarda `messora` negatif. API kampanyasında `toplu`, `chatbot`, `crm` negatif, böylece bu sorgular kendi kampanyalarına akar.

### 4.3 Teklif stratejisi (aşamalı)

| Aşama | Marka | Marka dışı | Geçiş koşulu |
|---|---|---|---|
| Hafta 1–2 (öğrenme) | Tıklamaları En Üst Düzeye Çıkarma, **maks. TBM tavanı 8 TL** | Tıklamaları En Üst Düzeye Çıkarma, **tavan 45 TL**. Toplu Mesaj ve Sektör'de 30 TL | Dönüşüm etiketleri doğrulandı ve hesapta ≥15 `sign_up` + `generate_lead` |
| Hafta 3–6 | Aynı (veya Hedef Gösterim Payı: üst %90, tavan 8 TL) | **Dönüşümleri En Üst Düzeye Çıkarma.** Birincil hedefler: `sign_up` + `generate_lead`. `sign_up_click` ikincil | Kampanyada son 30 günde ≥30 birincil dönüşüm ve CPA dalgalanması ±%25 içinde |
| Hafta 7–12 | Aynı | **Hedef EBM (tCPA)**, son 30 günlük gerçek CPA'nın 1,1–1,2 katıyla başlar. Ayarlar 2 haftada bir, en fazla %15 adımla yapılır | `purchase` içe aktarımı ayda ≥15 olursa dönüşüm değeri + tROAS testi düşünülür (büyük ihtimalle 90 gün sonrası) |

Düşük hacimde tek tek kampanyalar eşiğe ulaşamıyorsa marka dışı kampanyalar **ortak bir portföy teklif stratejisinde** toplanır.

### 4.4 Ağ, konum, dil, zaman, cihaz

- **Ağ:** Yalnızca Google Arama. **Arama Ağı iş ortakları KAPALI, Görüntülü Reklam Ağı genişletmesi KAPALI.** CSV'de `Networks = Google search` olarak ayarlı. İş ortağı ve Display seçeneklerini Editor'da ayrıca kontrol edin.
- **Konum:** Türkiye (geo ID 2792). Konum seçeneği **"Mevcudiyet: Hedeflenen konumda bulunan veya düzenli olarak bulunan kişiler"** olmalı. "İlgi" seçeneği kapalı kalmalı, yoksa yurt dışından Türkiye'yi arayanlar da hedeflenir. İstanbul, Ankara, İzmir, Bursa, Antalya, Kocaeli ve Konya **gözlem** için ayrı konum olarak eklenir. Teklif ayarı yapılmaz, veri toplandıktan sonra karar verilir.
- **Dil:** Türkçe (CSV'de `tr`). Not: Google dil hedeflemesinde kullanıcının dil ayarları gibi sinyalleri de kullanıyor. Türkiye'de arayüzü İngilizce olan çok sayıda iş kullanıcısı var. Anahtar kelimeler zaten Türkçe olduğu için **İngilizce eklemek düşük riskli bir genişlemedir**. 3. haftada değerlendirilir.
- **Reklam zamanlaması:** B senaryosunda 7/24 açık. Akıllı teklif saat ve gün etkisini kendisi öğrenir. Pzt–Cuma 08–20 dışını ve hafta sonunu gözlemleyin. Tıklama yaşı ile CPA farkı >%40 ise -%30 zaman ayarı yapın (yalnızca tıklama stratejisinde etkilidir). A senaryosunda Pzt–Cmt 08:00–21:00.
- **Cihaz:** Tümü açık. Türkiye'de tıklamaların ~%64'ü mobilden geliyor. Dönüşüm oranı ise masaüstünde %4,6, mobilde %2,8 ([Bravery](https://braverytechnology.com/tr/google-ads-istatistikleri/), metodolojisi belirsiz). Mobilde form ve kayıt akışı kısa tutulmalı. Mobil CPA, masaüstünün 1,5 katını 3 hafta üst üste aşarsa öğrenme aşamasında -%20 mobil ayarı uygulanır. Akıllı teklifte cihaz ayarı yalnızca -%100 olarak etkilidir.
- **Otomatik uygulanan öneriler:** Tamamı **KAPALI**. Özellikle "geniş eşleme ekle", "anahtar kelime ekle" ve "reklam metni oluştur".

### 4.5 Kitle (yalnızca gözlem)

Arama kampanyalarında kitleler **Gözlem** modunda eklenir, hedefleme daraltılmaz:

- Pazardaki segmentler: iş yazılımı, CRM yazılımı, pazarlama ve reklam hizmetleri, müşteri hizmetleri yazılımı (Google'daki Türkçe adlarıyla seçilmeli).
- Ayrıntılı demografi: işletme sahipleri / KOBİ (hesapta mevcutsa).
- Yeniden pazarlama: site ziyaretçileri (30/90 gün), fiyat sayfası ziyaretçileri, deneme başlatıp ödeme yapmayanlar.
- Hariç tutma: mevcut müşteriler. Bunun için Customer Match gerekir. Uygunluk koşulları (politika geçmişi, hesap yaşı) hesapta kontrol edilmeli.

---

## 5. 90 günlük aşamalı plan ve karar kuralları

### 5.1 Takvim

| Dönem | Odak | Yapılacaklar |
|---|---|---|
| **Hafta 0 (lansman öncesi)** | Altyapı | Hesap, ödeme ve doğrulama (§12). GTM/Google etiketi, Consent Mode v2, dört dönüşüm, GA4 bağlantısı (§10). Anahtar Kelime Planlayıcı ile Ek A tahminlerini güncelleme. CSV'yi Editor'a aktarma. Site ve LP'lerin yayında olduğunu doğrulama. Test dönüşümü atma (Tag Assistant). |
| **Hafta 1–2 (öğrenme)** | Veri ve temizlik | Tavanlı Tıklamaları En Üst Düzeye Çıkarma. **Her gün** arama terimleri raporu ve yeni negatifler. Politika ret/kısıtlamaları günlük izlenir ("WhatsApp" kısıtı gelirse §11.4). TO < %2 olan grupta başlık testi. |
| **Hafta 3–6 (optimizasyon)** | Dönüşüm maliyeti | Dönüşümleri En Üst Düzeye Çıkarma. Haftalık arama terimleri ve negatifler. RSA öğe raporu ("Düşük" etiketli başlıkları değiştir). LP A/B testi (form uzunluğu, CTA metni). Kötü performanslı anahtar kelimeleri durdur (§5.3). 30. gün ara değerlendirmesi. |
| **Hafta 7–12 (ölçek)** | Karlı büyüme | tCPA. Bütçeyi CPA hedefini tutan kampanyalara kaydır. Gösterim payı kaybı bütçeden geliyorsa artır. Geniş eşleme/AI Max deneyi (tek kampanya). Hukuki görüş olumluysa Rakip kampanyası testi. `purchase` çevrimdışı içe aktarımı. 60. ve 90. gün değerlendirmeleri. |

### 5.2 Bütçe karar kuralları

| Koşul (son 14 gün, kampanya veya hesap) | Karar |
|---|---|
| `sign_up` CPA ≤ 1.000 TL **ve** bütçe kaynaklı kayıp gösterim payı > %20 | Kampanya bütçesini **+%20** artır. Haftada en fazla bir kez, akıllı teklifte öğrenmeyi bozmamak için |
| CPA 1.000–1.500 TL | Bütçe sabit. Negatifler, metin ve LP iyileştirmesine odaklan |
| CPA 1.500–2.000 TL | Kampanya bütçesini **-%20** düşür, tCPA'yı sıkılaştırma (önce sorgu kalitesine bak) |
| CPA > 2.000 TL **veya** 1,5 × hedef harcayıp sıfır dönüşüm | Kampanyayı en iyi tek reklam grubuna indir ya da **duraklat**. Bütçeyi API kampanyasına aktar |
| Hesap genelinde **30. gün** `sign_up` CPA > 2.000 TL ve aktive deneme oranı < %20 | Senaryo A'ya (750 TL/gün) **in**. LP ve onboarding düzeltilmeden artırma yapma |
| **60. gün:** ilk kohortta deneme → ücretli ≥ %10 ve CAC ≤ 8.000 TL | Senaryo C'ye geçiş için ortaklık onayı iste |
| **90. gün:** CAC > 13.600 TL (6 ay geri ödemeyi aşıyor) | Bütçeyi A'ya indir veya kanalı yeniden tasarla |

### 5.3 Anahtar kelime ve reklam kuralları

- **Durdur:** Harcaması > 1,5 × hedef CPA (yaklaşık 1.500 TL) olup dönüşümü 0 olan anahtar kelime. Ya da 300'den fazla gösterim alıp TO'su < %1 olan anahtar kelime (marka hariç).
- **Negatife ekle:** İlgisiz niyetli her arama terimi. Kişisel kullanım, indirme, iş ilanı, casus yazılım, ücretsiz ve resmi olmayan araçlar, mod uygulamalar gibi. Sıralı veya tam eşleme kullanılır. Kampanyalar arası ortak liste "Messora Global Negatifler" olarak tutulur.
- **Tam eşlemeye terfi:** ≥3 dönüşüm getiren ve CPA'sı hedefin altında olan arama terimi kendi grubuna tam eşleme olarak eklenir.
- **RSA:** "Reklam gücü" en az "İyi" olmalı. Öğe raporunda 2 hafta "Düşük" kalan başlık değiştirilir. Aynı anda bir grupta en fazla 2 aktif RSA bulunur. B (marka geçmeyen) varyantı yalnızca kısıtlama olursa açılır.
- **Kalite Puanı:** < 5 olan anahtar kelimede önce açılış sayfası alaka düzeyi düzeltilir (başlıkta terim, sayfada H1 eşleşmesi), sonra teklif değiştirilir.

---

## 6. Anahtar kelimeler

### 6.1 TBM ve hacim tahminlerinin dayanağı

| Veri noktası | Değer | Kaynak | Not |
|---|---|---|---|
| Türkiye Arama Ağı ortalama TBM, 2026 1. çeyrek | 5,20 TL. TO %3,8, dönüşüm %3,4 | [Bravery Technology](https://braverytechnology.com/tr/google-ads-istatistikleri/) | Tüm sektörler. Metodoloji açıklanmıyor |
| "B2B Yazılım" ortalama TBM | 11,20 TL. TO %2,2, dönüşüm %3,1, EBM 361 TL | [Bravery Technology](https://braverytechnology.com/tr/google-ads-istatistikleri/) | Metodoloji açıklanmıyor, alt sınır olarak okunmalı |
| "SaaS, B2B Bulut Yazılım & Mobil Uygulama" medyan TBM | 82 TL, dönüşüm %5,9 | [212 Medya benchmark (GitHub)](https://github.com/fsakbas/turkiye-dijital-reklam-ve-roas-benchmark-2026) | Metodoloji açıklanmıyor |
| Keyword Planner örneği: "muhasebe programı" | ~9.400 arama/ay, teklif aralığı 43–219 TL, medyan 219 TL | [212 Medya](https://212medya.com.tr/blog/turkiyede-google-ads-tiklama-maliyetleri-2026/) | Yoğun rekabetli yazılım terimi. Üst sınır referansı |
| Türkiye TBM artışı | 2025'e göre +%17 | [PB Reklam](https://pbreklam.com/turkiye-google-ads-tbm-raporu-2026) | 200+ hesap, 2026 1. yarı |
| ABD B2B SaaS TBM | 8,50–14 $ | [Foundry CRO 2026](https://foundrycro.com/assets/2026-paid-media-benchmark-report-foundrycro.pdf) | Türkiye'ye taşınamaz, yalnızca bağlam |
| İspanya "whatsapp business api" | ~1K arama/ay, TBM 16,8 $'a kadar | [GitHub PR (Nightwatch verisi)](https://github.com/MMARTID/alhabla/pull/423) | Farklı pazar. Terimin ticari değerini gösteriyor |

**Tahminimiz:** WhatsApp API ve CRM terimlerinde açık artırmaya SMS ve telekom operatörleri (Netgsm, Kobikom, VatanSMS vb.), BSP'ler ve yerli paneller katılıyor. Bu nedenle bu terimlerin genel "B2B Yazılım" ortalamasının (11 TL) üstünde, "muhasebe programı" gibi olgun kategorilerin (219 TL) ise altında olması bekleniyor. Planlama aralığımız **harmanlanmış 25–55 TL, baz 35 TL**. "Toplu mesaj" ve "otomatik yanıt" gibi tüketici niyetiyle karışan terimlerde 5–30 TL, "API fiyatları/sağlayıcı" gibi satın alma niyetli terimlerde 20–70 TL. Hacimler Keyword Planner aralık biçiminde verildi: 10–100, 100–1K, 1K–10K. **Bu değerler doğrulanmadı.** Hesap açıldığında Keyword Planner'da (Türkiye, Türkçe, son 12 ay) her terim için "sayfa üstü teklif (düşük/yüksek aralık)" alınmalı ve Ek A güncellenmeli.

### 6.2 En önemli 10 anahtar kelime (tahmin)

| # | Anahtar kelime | Eşleme | Tahmini aylık arama | Tahmini TBM |
|---|---|---|---|---|
| 1 | whatsapp business api | sıralı + tam | 1K–10K | 20–60 TL |
| 2 | whatsapp api | sıralı + tam | 1K–10K | 15–50 TL |
| 3 | whatsapp business api fiyatları | sıralı + tam | 100–1K | 20–60 TL |
| 4 | whatsapp toplu mesaj | sıralı | 1K–10K | 8–30 TL |
| 5 | whatsapp toplu mesaj gönderme | sıralı | 1K–10K | 8–30 TL |
| 6 | whatsapp chatbot | sıralı + tam | 100–1K | 15–45 TL |
| 7 | whatsapp crm | sıralı + tam | 100–1K | 20–60 TL |
| 8 | whatsapp cloud api | sıralı | 100–1K | 15–45 TL |
| 9 | whatsapp bot | sıralı | 1K–10K | 8–30 TL |
| 10 | kurumsal whatsapp | sıralı | 100–1K | 15–45 TL |

Tam liste (82 satır) **Ek A**'da, CSV'de de aynı şekilde.

### 6.3 Negatif anahtar kelimeler

Kampanya düzeyinde, sıralı eşlemeyle (`Campaign Negative Phrase`) **127 terim** her kampanyaya eklendi (Ek C). Kategoriler:

- **Uygulama ve indirme:** indir, apk, yükle, güncelleme, web giriş, whatsapp web, masaüstü, pc, iphone, android…
- **Mod uygulamalar:** gb whatsapp, whatsapp plus, yowhatsapp, fm whatsapp, aero, mod, tema…
- **Kişisel kullanım ve destek:** yedekleme, silinen mesajlar, nasıl silinir, hesap silme, engelleme, son görülme, çevrimiçi, durum, çıkartma, çalışmıyor, doğrulama kodu…
- **Casus ve gri alan:** hack, casus, takip programı, kim baktı, dinleme, şifre kırma, başkasının, ban kaldırma, fake/sanal numara, spam…
  - Bu terimlerden reklam almak hem bütçe israfıdır hem de Google'ın **"Sahtekârlığa olanak tanıma"** politikasıyla çelişen bir trafik profili oluşturur ([Google](https://support.google.com/adspolicy/answer/6016086?hl=en)).
- **Geliştirici ve resmi olmayan araçlar:** github, python, php, npm, baileys, selenium, whatsapp-web.js, qr kod ile bot, chrome eklentisi…
- **İş ilanı ve diğer:** iş ilanı, kariyer, staj, maaş, ödev, pdf, logo, grup linkleri, sözler, hisse…

**"ücretsiz" kararı:** "ücretsiz" kelimesi yalnızca **Toplu Mesaj** kampanyasında negatif. Bu kampanyada "ücretsiz toplu mesaj" aramaları büyük oranda tüketici veya resmi olmayan araç niyeti taşıyor. API, Chatbot ve CRM kampanyalarında "ücretsiz deneme" ve "ücretsiz dene" gibi sorgular değerli olabileceği için negatif yapılmadı. Bu sorgular arama terimleri raporunda izlenmeli. Aynı mantıkla **"nedir"** ve **"nasıl"** genel negatif yapılmadı. "whatsapp business api nedir" sorgusu üst huni değerindedir, `/whatsapp-business-api-nedir` rehber sayfası bunun için var. Bu sorgularda CPA hedefin 2 katını geçerse negatife eklenir.

---

## 7. Reklam metinleri (RSA)

- **Yapı:** Her reklam grubunda **A** (ana) ve **B** (yedek) olmak üzere iki RSA var. A, gruba özgü 4–7 başlıkla başlar, kalan yerler ortak başlık havuzundan doldurulur (14 gün deneme, kart gerekmez, taahhüt yok, 1.290 TL + KDV'den, izin yönetimi, kendi AI anahtarı…). B'de **"WhatsApp" kelimesi hiç geçmez** ve duraklatılmış durumdadır.
- **Doğrulama:** 32 RSA, 480 başlık ve 128 açıklama betikle ölçüldü. En uzun başlık 30/30, en uzun açıklama 90/90 karakter. Tüm yollar ≤15 karakter ve yollarda marka adı yok. Ayrıntı **Ek B**'de.
- **Sabitleme (pinning):** Sabitleme Reklam Gücü'nü düşürür ve Google'ın kombinasyon öğrenmesini daraltır. Bu nedenle **yalnızca Marka grubunda "Messora | Resmi Site" başlığı 1. konuma sabitlendi.** Marka dışı gruplarda sabitleme yok. İstisna: Hukuk ekibi her gösterimde bir ibare (ör. "+ KDV") isterse ilgili başlık 2. konuma sabitlenir, ama aynı konuma 2–3 alternatif başlık eklenir.
- **İddia disiplini:** Yalnızca doğrulanabilir özellik iddiaları kullanıldı. "Banlanmaz", "%100 teslim", "Meta onaylı/partneri" ve "en ucuz" gibi ifadelerden kaçınıldı. Meta Tech Provider statüsü şu anda **kapalı/başvuru aşamasında** (`docs/TECH_PROVIDER.md`). Bu nedenle "Meta iş ortağı" ifadesi **onay gelmeden kullanılmamalı**.
- **Fiyat ibaresi:** Tüm fiyatlar "+ KDV" ile yazıldı. LP'de de aynı fiyat ve KDV bilgisi görünmeli. Reklam ile sayfa arasındaki fiyat tutarsızlığı "Yanıltıcı beyan" politikası kapsamında ret sebebidir.

Tam metinler: **Ek B**.

---

## 8. Öğeler (assets)

Hesap düzeyinde eklenir (Editor'da `<Account-level>`). Uzunluklar betikle doğrulandı.

### 8.1 Site bağlantıları (metin ≤25, açıklama satırları ≤35)

| Metin | Açıklama 1 | Açıklama 2 | URL |
|---|---|---|---|
| Fiyatlar ve Paketler | 1.290 TL + KDV'den başlayan planlar | Yıllıkta 12 ay 10 ay fiyatına | /fiyatlar |
| 14 Gün Ücretsiz Deneyin | Kart bilgisi gerekmez | Taahhüt yok, dilediğiniz an bırakın | https://app.messora.tech/signup |
| Demo Talep Edin | Paneli ekibimizle birlikte gezin | Canlı tanıtım planlayın | /demo |
| AI Ajanları | 7/24 yanıt, zor soruda insana devir | Kendi AI anahtarınızı bağlayın | /ozellikler/yapay-zeka-ajanlari |
| Ekip Gelen Kutusu | Tek numara, çok kullanıcı | Atama, etiket ve iç notlar | /ozellikler/ekip-gelen-kutusu |
| İzinli Kampanyalar | Onay kayıtları otomatik tutulur | Teslim ve okunma raporları | /ozellikler/whatsapp-kampanyalari |
| API ve Webhook | Kendi sistemlerinize bağlayın | Dokümanlı public API | /ozellikler/api-ve-webhook |
| Güvenlik ve KVKK | Rol ve yetki yönetimi | Veri işleme sözleşmesi hazır | /guvenlik |

Site bağlantısı metni için 25 karakter sınırı Google kaynaklı ([Google](https://support.google.com/google-ads/answer/2375416?hl=en)). 35 karakterlik açıklama satırı sınırı ise üçüncü taraf kaynaklı ([WordStream](https://www.wordstream.com/blog/google-ads-assets)). Deneme bağlantısı `app.` alt alan adına gidiyor. Google aynı ana alan adını kabul eder, yine de Editor ön izlemesinde kontrol edilmeli.

### 8.2 Açıklama metinleri (callout, ≤25)

14 Gün Ücretsiz Deneme · Kart Bilgisi Gerekmez · Taahhüt Yok · Türkçe Destek · TL Fiyat + KDV · İzin Yönetimi Dahil · Şablon Oluşturucu Dahil · Hazır Otomasyonlar · Kendi AI Anahtarınız · Public API ve Webhook · Havale/EFT ile Ödeme · Yıllıkta 2 Ay Bizden

Callout sınırı 25 karakter. Aynı anda en fazla 4 tanesi gösterilir ([Google](https://support.google.com/google-ads/answer/6079510?hl=en)). "Havale/EFT ile Ödeme" yalnızca kartlı ödeme gelene kadar kullanılmalı.

### 8.3 Yapılandırılmış snippet'ler

- **Başlık "Hizmetler" (Service catalog):** Ekip gelen kutusu, İzinli kampanyalar, AI ajanlar, Otomasyonlar, Şablon oluşturucu, Analitik, API ve webhook
- **Başlık "Türler" (Types):** Başlangıç, Büyüme, Pro, Kurumsal

Google en az 3, ideal olarak 4'ten fazla değer öneriyor ([Google](https://support.google.com/google-ads/answer/6280012?hl=en)). Türkçe dilde hangi başlıkların sunulduğu yardım sayfasında listelenmiyor. Editor ve arayüz açılır menüsünden kontrol edilmeli.

### 8.4 Fiyat öğeleri: şu an kullanılamıyor

Google'ın yardım sayfasına göre fiyat öğeleri **Türkçe dilinde desteklenmiyor**. Desteklenen 10 dil arasında Türkçe yok ve **TRY de para birimi listesinde değil** ([Google](https://support.google.com/google-ads/answer/7065415?hl=en)). Bu yüzden plan fiyatları başlık, açıklama, callout ve site bağlantısı içinde verildi. Destek gelirse hazır set (başlık/açıklama ≤25, en az 3 öğe, "aylık" birim):

| Başlık | Açıklama | Fiyat | Niteleyici |
|---|---|---|---|
| Başlangıç | 2 kullanıcı, 1 numara | 1.290 TL/ay | + KDV |
| Büyüme | 5 kullanıcı, 3 AI ajan | 2.990 TL/ay | + KDV |
| Pro | 12 kullanıcı, 3 numara | 6.490 TL/ay | + KDV |
| Kurumsal | Özel limit, SLA | 14.990 TL'den | Başlayan |

### 8.5 Potansiyel müşteri formu öğesi: lansmanda kullanılmıyor

Form öğesi Arama kampanyalarında kullanılabiliyor, fakat bazı koşulları var ([Google](https://support.google.com/google-ads/answer/9423234?hl=en), [Google politika](https://support.google.com/adspolicy/answer/9472930?hl=en)):

- iyi bir politika geçmişi
- uygun sektör
- gizlilik politikası bağlantısı
- dönüşüme yönelik teklif stratejisi

Doğrudan forma açılan reklam için ayrıca reklamveren doğrulaması gerekiyor. Desteklenen ülke listesi yayımlanmıyor. Türkiye'nin desteklenip desteklenmediğini **doğrulayamadık**. Öneri: 7. haftadan sonra, tCPA'ya geçildiğinde Demo grubu için test edin. Form verisi KVKK aydınlatma metni bağlantısı taşımalı.

### 8.6 Diğer öğeler

- **İşletme adı ve logo:** Reklamveren doğrulaması tamamlanınca eklenir.
- **Görsel öğeler:** Uygunsa ürün ekran görüntüleri eklenir. WhatsApp logosu kullanılmaz (§11).
- **Arama öğesi:** Satış hattı varsa yalnızca mesai saatlerinde.

---

## 9. Açılış sayfası eşlemesi ve sayfa gereksinimleri

| Niyet | Kampanya / grup | Sayfa | Birincil CTA | İkincil CTA |
|---|---|---|---|---|
| Marka | Marka | `/` | 14 gün dene → app.messora.tech/signup | Demo |
| API ve kurulum | API - Genel, Kurumsal | `/lp/whatsapp-business-api` | Dene | Demo, "Kurulum Hizmeti" |
| Fiyat karşılaştırma | API - Fiyat | `/fiyatlar` | Dene | Demo |
| Toplu mesaj | Toplu Mesaj, Pazarlama | `/lp/whatsapp-toplu-mesaj` | Dene | İzin yönetimi rehberi |
| Chatbot ve AI | Chatbot, Yapay Zeka | `/lp/whatsapp-chatbot` | Dene | Demo |
| CRM ve ekip | CRM, Ekip Gelen Kutusu | `/lp/whatsapp-crm` | Dene | Demo |
| Sektör | 5 grup | `/cozumler/*` | Dene | Demo |

**LP kontrol listesi (Google kalite ve politika):**

- H1, reklam grubunun ana terimini içermeli (ör. "WhatsApp Business API ile ekip gelen kutusu").
- Fiyat ve "+ KDV" bilgisi görünür olmalı, deneme koşulları (14 gün, kart yok) yazılı olmalı.
- Şirket unvanı, adres ve iletişim bilgisi, KVKK aydınlatma metni, çerez politikası ve kullanım koşulları bulunmalı. Bu sayfalar `/yasal/*` altında.
- Mobil hızı LCP < 2,5 sn olmalı.
- Form 3–5 alanla sınırlı olmalı: ad, iş e-postası, telefon, şirket, çalışan sayısı.
- "Toplu mesaj" sayfası **izinli gönderim** ve resmi API farkını açıkça anlatmalı. Bu, hem politika uyumu hem de "QR bot/spam" arayan kullanıcıyı eleyip kaliteyi artırmak için gerekli.
- Sayfada "WhatsApp" adı Meta kurallarına uygun yazılmalı (§11.2).

---

## 10. Dönüşüm ölçümü

### 10.1 Dönüşüm eylemleri

| Eylem | Nerede tetiklenir | Google Ads rolü | Değer | Sayım |
|---|---|---|---|---|
| `generate_lead` | messora.tech demo/iletişim formu başarıyla gönderildiğinde (sunucu 2xx yanıtından sonra, "teşekkürler" durumunda) | **Birincil** | 1.000 TL (tahmini değer, sonra güncellenir) | Bir |
| `sign_up_click` | messora.tech üzerinde app.messora.tech/signup'a giden CTA tıklaması | **İkincil** (yalnızca gözlem). Öğrenme ilk haftasında `sign_up` hiç yoksa geçici olarak birincil | 0 | Bir |
| `sign_up` | app.messora.tech'te hesap gerçekten oluştuğunda (sunucu onayı sonrası) | **Birincil** | 500 TL (tahmini) | Bir |
| `number_connected` (öneri) | Denemede WhatsApp numarası bağlanıp ilk mesaj gönderildiğinde | İkincil. Ileride birincil adayı | 0 | Bir |
| `purchase` | İlk ücretli fatura veya havale eşleşmesi onaylandığında. Data Manager üzerinden çevrimdışı içe aktarılır | Birincil (veri yeterince olunca) | Fatura tutarı, KDV hariç | Bir |

GA4'te `generate_lead`, `sign_up` ve `purchase` önerilen etkinlik adlarıdır. Google Ads'te dönüşümler, GA4'ten içe aktarmak yerine **Google Ads dönüşüm etiketiyle** (gtag/GTM) doğrudan kurulmalı. Bu, daha hızlı sinyal ve Gelişmiş Dönüşüm desteği sağlar. GA4 içe aktarımı yalnızca raporlama karşılaştırması için ikincil kalır.

### 10.2 Alan adları arası ölçüm

- messora.tech ve app.messora.tech **aynı kayıtlı alan adının alt alan adları**. Google etiketi çerezleri varsayılan olarak `.messora.tech` üst düzeyine yazar. Bu nedenle GA4 ve Google Ads (`_gcl_aw`) çerezleri iki tarafta da okunur. Yine de GA4 yönetiminde **Configure your domains** listesine iki alan adı da eklenmeli ve **yönlendirmelerde `_gl` parametresi korunmalı** ([Google GA4](https://support.google.com/analytics/answer/10071811?hl=en), [Google tag](https://developers.google.com/tag-platform/devguides/cross-domain)).
- Her iki tarafta da **aynı G- ölçüm kimliği** ve aynı Google Ads dönüşüm kimliği kullanılmalı. Kendi kendine yönlendirme ("referral") olmaması için `messora.tech` istenmeyen yönlendirmeler listesine eklenmeli.
- Kayıt formu `gclid`, `gbraid` ve `wbraid` değerlerini (URL'den veya `_gcl_aw` çerezinden) okuyup kiracı kaydına yazmalı. Bu, §10.4'teki çevrimdışı içe aktarımın temelidir.
- Firma alt alan adları (`<firma>.messora.tech`) ve özel alan adları ölçüme dahil edilmez. Reklam yolculuğu signup ile sona erer.

### 10.3 Gelişmiş Dönüşümler

`generate_lead` ve `sign_up` için Gelişmiş Dönüşümler açılmalı. Form gönderiminde normalleştirilmiş ve SHA-256 ile karma haline getirilmiş e-posta ve telefon `user_data` olarak iletilir. Google Nisan 2026'da web ve potansiyel müşteri Gelişmiş Dönüşümlerini tek ayarda birleştirdi ([Google](https://support.google.com/google-ads/answer/15713840?hl=en)). Veri, KVKK aydınlatma metninde "reklam ölçümü" amacıyla yurt dışına aktarım olarak açıklanmalı.

### 10.4 Çevrimdışı içe aktarım (deneme → ücretli)

Messora'da ödeme havale ile alınıyor ve manuel eşleştiriliyor. Bu nedenle tarayıcıda bir "satın alma" anı yok. Ödeme onaylandığında `purchase` sunucudan aktarılmalı. Bunun için anahtar olarak **GCLID + karma e-posta**, değer olarak fatura tutarı ve işlem kimliği olarak fatura no kullanılır. Google, eski çevrimdışı dönüşüm içe aktarımını "eski özellik" olarak tanımlıyor ve **potansiyel müşteriler için Gelişmiş Dönüşümler + Data Manager**'ı öneriyor. Google Ads API üzerinden çevrimdışı yükleme 15 Haziran 2026'dan itibaren Data Manager API'ye taşındı ([Google](https://support.google.com/google-ads/answer/14274408?hl=en), [Google](https://support.google.com/google-ads/answer/15479486?hl=en)). Zamanlama:

- **1. ay:** CSV ile manuel yükleme (haftalık).
- **2–3. ay:** Data Manager API entegrasyonu (günlük).

`purchase`, ayda 15'ten fazla olana kadar **ikincil** tutulur. Sonra değer tabanlı teklif için birincil yapılır.

### 10.5 Consent Mode v2 ve KVKK

- Google'ın AB Kullanıcı Rızası Politikası ve Consent Mode v2 zorunluluğu **AEA, Birleşik Krallık ve İsviçre** trafiği için geçerli. Türkiye bu kapsamda değil ([Google](https://support.google.com/google-ads/answer/13695607?hl=en), [Google](https://www.google.com/about/company/user-consent-policy-help.html)).
- Buna karşılık **KVKK Çerez Uygulamaları Hakkında Rehber**, zorunlu olmayan çerezler için çerez yerleştirilmeden **önce açık rıza** alınmasını bekliyor. Önceden işaretli kutular geçerli rıza sayılmıyor ([KVKK](https://kvkk.gov.tr/Icerik/7353/Cerez-Uygulamalari-Hakkinda-Rehber), [NTV](https://www.ntv.com.tr/teknoloji/kvkk-cerezler-icin-rehber-hazirladi,BdjApRquJEmK4KG6JZ-IpQ)).
- **Öneri:** Rıza yönetim aracı (CMP) + **Consent Mode v2 "advanced"** kurulmalı. Varsayılan olarak `ad_storage`, `analytics_storage`, `ad_user_data` ve `ad_personalization` değerleri `denied` olur. Kullanıcı kabul ettiğinde `update` gönderilir. Advanced modda etiket, rıza vermeyen kullanıcılar için çerezsiz ping gönderir. Bu, Google'ın dönüşüm modellemesini Basic moda göre belirgin biçimde iyileştirir.
- Türkiye trafiğinin büyük kısmı reddetse bile ölçüm tamamen kaybolmaz. AB'den gelen trafik az olsa da aynı kurulum uyumu sağlar.
- Sunucu tarafı `purchase` aktarımında, kullanıcının kayıt sırasında verdiği "pazarlama ölçümü" rızası dikkate alınmalı (hukuk görüşüyle).

### 10.6 GA4 bağlantısı

GA4 mülkü ile Google Ads hesabı bağlanır. Otomatik etiketleme açık olmalı. GA4'te "Google sinyalleri" ve "kitle paylaşımı" rıza ile çalışır. GA4 kitleleri (fiyat sayfası ziyaretçisi, deneme başlatan) Ads'e aktarılır. Raporlamada GA4 "Ücretli arama" kanalı ile Ads dönüşümleri arasındaki fark (atıf modeli farkı) beklenir. Karar verirken **Google Ads dönüşümleri** esas alınır.

---

## 11. Politika ve marka (trademark)

### 11.1 Google'ın marka politikası

- **Anahtar kelimeler kısıtlanmıyor.** Google, anahtar kelime olarak marka kullanımını kısıtlamıyor. Kısıtlama yalnızca reklam metnindeki kullanım için geçerli ([Google](https://support.google.com/adspolicy/answer/6118?hl=en)).
- **Reklam metninde iki durum kısıtlanıyor:** (a) markayı **doğrudan rakip** kullanıyorsa, (b) kullanım **kafa karıştırıcı, yanıltıcı veya aldatıcı** ise. Markanın sıradan anlamıyla **tanımlayıcı kullanımı** kısıtlanmıyor.
- **Bayi ve uyumlu ürün istisnası:** Sayfası esas olarak markalı ürünü veya **uyumlu ürünleri** satan reklamverenler kısıtlanmıyor.
- **Şikâyet süreci:** Kısıtlama, marka sahibinin şikâyetiyle başlıyor. Google, şikâyetleri URL'leriyle belirlenmiş reklamverenlere karşı ve marka hakkının gösterildiği ülkelerde kabul ediyor. Kısıtlama genellikle aynı ikinci düzey alan adına giden tüm reklamlara uygulanıyor. İtiraz edilebiliyor. Marka sahibi ise yetkilendirme formuyla izin verebiliyor.

**Messora için değerlendirme (tahmin, hukuki görüş değildir):** Messora, WhatsApp'ın rakibi değil. WhatsApp Business Platform'un resmi Cloud API'si üzerinde çalışan **tamamlayıcı ve uyumlu bir yazılım**. "WhatsApp Business API paneli" ve "WhatsApp için ekip gelen kutusu" ifadeleri tanımlayıcı ve uyumluluk bildiren kullanımlar. Bu nedenle kısıtlanma riski **düşük ile orta** arasında. Ancak Meta'nın Türkiye'de bu alanda şikâyet açıp açmadığını **doğrulayamadık**. Kısıtlama gelirse reklam genelde "Onaylandı (sınırlı)" durumuna düşer veya reddedilir ([freak.marketing](https://freak.marketing/post/google-ads-approved-limited-trademarks-ad-text), 3. taraf).

### 11.2 Meta / WhatsApp marka kuralları

Meta'nın WhatsApp marka kurallarına göre ([Meta](https://www.meta.com/brand/resources/whatsapp/whatsapp-brand/)):

| Kural | Reklama ve LP'ye etkisi |
|---|---|
| "W" ve "A" her zaman büyük yazılır. Kelime kısaltılmaz ve değiştirilmez ("Whatsapp", "WA", "Wp" kullanılmaz) | Tüm metinler "WhatsApp" olarak yazıldı. "WA Business API" yalnızca iç kampanya adında geçiyor, kullanıcı görmez |
| WhatsApp adı veya logosu, başka bir şirketin **ürün veya hizmet adının parçası** olarak kullanılmaz | Ürün adı "Messora". "Messora WhatsApp" veya "WhatsApp Messora" gibi bir ürün adı kullanılmıyor |
| Ortaklık, sponsorluk veya onay **ima edilmez** | "Meta/WhatsApp onaylı", "resmi partner" gibi ifadeler yok. "Resmi" sözcüğü yalnızca **API'yi** niteliyor ("resmi Cloud API") |
| WhatsApp, materyallerin **en belirgin unsuru yapılmaz** ve fiil olarak kullanılmaz | LP hero alanında marka Messora olmalı. "WhatsApp'layın" gibi kullanımlar yok |
| Alan adında ve kullanıcı adında kullanılmaz (ikinci düzey alan adı) | Alan adı `messora.tech`. `/lp/whatsapp-…` yolları alt dizindir, ikinci düzey alan adı değildir. Risk düşük, ama yeniden adlandırma seçeneği aşağıda |
| Yalnızca resmi marka varlıkları kullanılır, logo değiştirilmez | Reklamda WhatsApp logosu kullanılmıyor. LP'de entegrasyon logosu gerekiyorsa yalnızca resmi varlık, orijinal oranlarda kullanılmalı |

### 11.3 Türk hukuku: rakip ve marka anahtar kelimeleri

**SMK m.7/3-(f)** (m.29 atfıyla) işaretin internette "ticari etki doğuracak biçimde **anahtar sözcük**" olarak kullanılmasını marka hakkının ihlali sayabiliyor. Yargıtay'ın bazı kararlarında anahtar kelime kullanımı, reklam metninde marka geçmese bile haksız rekabet olarak değerlendirilmiş. Kaynaklar arasında yaklaşım farkı var ([Etkin Patent](https://etkinpatent.com/dijital-rekabet-ortaminda-marka-hakki-ve-arama-motoru-reklamciligi-yargitay-kararlari-isiginda-kapsamli-bir-inceleme/), [DG Legal](https://www.dg-legal.com/yazi/google-adwords-kapsaminda-marka-hakkina-ve-haksiz-rekabete-iliskin-hukuki-degerlendirme), [Emre Kurt](https://www.emrekurt.av.tr/blogs/2024/11/22/googleadwordsmarkaihlali/)). Sonuçları:

- **Rakip kampanyası** (WATI, respond.io, SleekFlow) **duraklatılmış** olarak teslim edildi. Yerli rakipler (Chatinbox, Wao, ApisWP, WAtakip, Netgsm, Kobikom) **listeye hiç alınmadı**. Rakip markaları reklam metninde **asla** kullanılmaz. Açmadan önce marka avukatından yazılı görüş alınmalı. Açılırsa "alternatif" niteliğinde sorgular ve Türkiye'de tescili olmayan global markalarla sınırlı kalmalı. Bu tescil durumu TÜRKPATENT'ten kontrol edilmeli.
- **"WhatsApp" anahtar kelimeleri:** Messora'nın kullanımı ürünün amacını ve uyumluluğunu belirtmek içindir (SMK m.12 kapsamında dürüst kullanım savunması, **hukuki görüşle teyit edilmeli**). Risk rakip kampanyasından çok daha düşük, ama sıfır değil.

### 11.4 Riskler ve önlemler

| Risk | Olasılık (tahmin) | Etki | Önlem |
|---|---|---|---|
| Reklam metninde "WhatsApp" kısıtlaması (Meta şikâyeti) | Düşük–orta | Ana reklamlar sınırlı gösterim alır veya reddedilir | Her grupta hazır **B varyantı** (duraklatılmış). Kısıtlama bildirimi gelir gelmez B açılır, A durdurulur. Uyumlu ürün gerekçesiyle itiraz edilir. Anahtar kelimeler etkilenmez |
| Yol ve URL'de "whatsapp" | Düşük | Kısıtlama URL temelli de uygulanabilir | Görünen yollarda marka kullanılmadı. Gerekirse LP'ler `/lp/is-mesajlasma-api` gibi nötr yollara 301 ile taşınabilir |
| "Toplu mesaj" trafiğinin spam/gri niyetli olması | Orta | Bütçe israfı, düşük deneme kalitesi | Sıkı negatif listesi, "izinli gönderim" vurgusu, `number_connected` kalite metriği |
| Hesap askıya alma / doğrulama gecikmesi | Düşük | Lansman gecikmesi | Doğrulamayı lansmandan **en az 2 hafta önce** başlatmak (§12) |
| Yanıltıcı beyan (fiyat/deneme tutarsızlığı) | Düşük | Reklam reddi | LP ve reklam fiyatları aynı, "+ KDV" her yerde |
| TBM'nin tahminden yüksek çıkması | Orta | CPA artışı | Tavanlı başlangıç, §5.2 karar kuralları |

### 11.5 Marka geçmeyen yedek metin örnekleri

Tam setler Ek B'de, "B-markasız" başlığı altında. Örnekler:

- **Başlıklar:** "Resmi Cloud API Altyapısı", "Kurumsal Mesajlaşma Paneli", "Tek Numara, Çok Kullanıcı", "Chatbot ve AI Ajan", "Resmi API ile Toplu Mesaj"
- **Açıklama:** "Resmi Cloud API altyapısıyla ekip gelen kutusu, kampanya, AI ajan ve API tek yerde."

### 11.6 Diğer ilgili Google politikaları

- **Yanıltıcı beyan:** Şeffaf fiyat ve deneme koşulları, şirket kimliği görünür olmalı.
- **Hedef sayfa gereksinimleri:** Çalışan, hızlı ve yayında bir sayfa gerekir.
- **Güvenilmez iddialar:** "Ban garantisi" gibi ifadeler kullanılmaz.
- **Sahtekârlığa olanak tanıma:** Casus ve takip yazılımı çağrışımı yapılmaz ([Google](https://support.google.com/adspolicy/answer/6016086?hl=en)).
- **Reklam ağının kötüye kullanımı:** Spam veya kötü amaçlı yazılım içerik barındırılmaz ([Google](https://support.google.com/adspolicy/answer/6020954?hl=en)).
- **Veri toplama:** Formlar HTTPS üzerinden çalışmalı ve gizlilik politikası bağlantısı içermeli.
- **Yerel hukuk:** Google, hedeflenen ülkenin yerel mevzuatına uyulmasını bekliyor ([Google](https://support.google.com/adspolicy/answer/6023676)). Ticari Reklam ve Haksız Ticari Uygulamalar Yönetmeliği'nde dijital reklam ve hedefleme kurallarıyla ilgili değişiklikler var ([CMS](https://cms.law/en/tur/legal-updates/turkiye-launches-new-era-of-digital-advertising-with-amendments-to-the-commercial-advertising-and-unfair-commercial-practices-regulation)).

---

## 12. Hesap kurulumu kontrol listesi ve sahiplik

### 12.1 Kurulum adımları

1. **Yönetici hesabı (MCC):** Messora adına açılır. Reklam hesabı bunun altında oluşturulur. Ortak ve ajans erişimi kullanıcı düzeyinde verilir.
2. **Reklam hesabı:** Para birimi **TRY**, saat dilimi **(GMT+03:00) İstanbul**. Para birimi hesap oluşturulduktan sonra değiştirilemiyor, bu yüzden ilk seferde doğru seçilmeli.
3. **Ödeme profili:** Türü "Kuruluş". Yasal unvan, vergi dairesi, VKN ve fatura adresi girilir. Ödeme ayarı olarak sonradan ödeme (otomatik, kart) veya ön ödeme seçilir. Aylık faturalandırma (kredi limiti) yalnızca uygun müşteriler içindir ([Google](https://support.google.com/google-ads/answer/1704418?hl=tr)). Türkiye'de geçerli ödeme yöntemleri [payments.google.com/paymentsinfofinder](https://payments.google.com/paymentsinfofinder) üzerinden kontrol edilmeli.
4. **Fatura kesen Google şirketi:** Varsayılan Google Ireland Ltd'dir. Bu durumda alıcı KDV'yi sorumlu sıfatıyla 2 No.lu beyanname ile öder. "Etkilenen Ürünler" bildirimi alındıysa karşı taraf Google Reklamcılık ve Pazarlama Ltd. Şti. olur ve KDV faturaya eklenir ([roicool](https://www.roicool.com/en/blog/google-ads-costs-in-turkey)). Bu durum SMMM ile netleştirilmeli.
5. **Yasal İşletim Maliyeti:** Türkiye'de yayınlanan reklamlara ek ücret uygulanıyor: 2026'da **%4,5**. Oran 1 Ocak 2027'den itibaren **%2,25** (Google e-postası) veya **%2,5** (destek dokümanı) olacak, kaynaklar çelişiyor ([Webtekno](https://webtekno.com/google-turkiye-2027-reklam-ek-ucret-orani-dusuyor-h228802.html), [CNBC-e](https://www.cnbce.com/haberler/googledan-turkiyedeki-reklam-verenlere-indirim-2027de-yari-yariya-dusebilir-h41591)). Bu ücret, Google'ın ödediği %5'lik Dijital Hizmet Vergisi'nin karşılığıdır. DHV oranı 2026'da %5, 2027'de %2,5 ([Alomaliye](https://www.alomaliye.com/2025/12/25/dijital-hizmet-vergisi-oranlari-tablosu-ve-aciklamalar/)).
6. **Reklamveren doğrulaması:** "İşletmeniz hakkında" soruları için 30 gün, istenen doğrulamalar için 30 gün daha süre veriliyor. Kimlik doğrulaması, işletme faaliyetleri doğrulaması veya ikisi birden istenebiliyor. Doğrulama tamamlanmazsa hesap duraklatılıyor. Google inceleme için 5–7 iş günü öngörüyor. Ödeme profili değişirse doğrulama yeniden istenebiliyor ([Google](https://support.google.com/adspolicy/answer/15577076?hl=en), [Google](https://support.google.com/adspolicy/answer/9703665)). Gerekecek belgeler: yetkili kişinin kimliği, ticaret sicil gazetesi veya vergi levhası, imza sirküleri, adres belgesi. **Doğrulanan ad, "Reklamı veren" bilgisi olarak kullanıcıya gösterilir** (Ads Transparency Center). Bu yüzden ödeme profilinin kimin adına açıldığı önemli (§12.2).
7. **Etiketler ve ölçüm:** GTM konteyneri, Google etiketi, Consent Mode v2 ve CMP, dört dönüşüm, Gelişmiş Dönüşümler, GA4 bağlantısı, otomatik etiketleme. Tag Assistant ile test dönüşümü yapılır.
8. **İçe aktarma:** Google Ads Editor'da Hesap → İçe aktar → Dosyadan, `google-ads-editor-import.csv` seçilir. Ön izlemede sütun eşlemesi kontrol edilir. Yayınlamadan önce Editor'daki "Değişiklikleri kontrol et" uyarıları temizlenir. Hesap düzeyi öğeler (§8) ve ortak negatif liste manuel eklenir.
9. **Lansman günü:** Kampanyalar açılır. İlk 48 saat politika durumu, harcama hızı ve arama terimleri izlenir.

### 12.2 Kim neye sahip olur? (sözleşme için öneri)

| Varlık | Sahip | Ortak/diğer taraf | Not |
|---|---|---|---|
| Google Ads yönetici hesabı (MCC) ve reklam hesabı | **Messora** (yönetici erişimi) | Ortak: standart veya yönetici kullanıcı | Marka ve geçmiş verisi Messora'da kalır. Ortaklık bitse de hesap taşınmaz |
| **Ödeme profili / faturalandırma hesabı** | **Bütçeyi taahhüt eden ortak şirket** (ortağın unvanı ve VKN'si) | Messora: ödeme profilinde "yalnızca görüntüleme" | Faturalar ortağın adına kesilir ve gider ortağın defterine girer. **Dikkat:** Doğrulanan reklamveren adı ödeme profilinden gelir. Reklamlarda "Reklamı veren: [ortak unvanı]" görünür. Reklamlarda Messora unvanının görünmesi isteniyorsa ödeme profili Messora'da olmalı, ortak harcamayı sözleşmeyle Messora'ya fatura karşılığı yansıtmalı. Bu karar sözleşmeden önce verilmeli |
| Google Ads faturaları ve KDV/YİM belgeleri | Ödeme profili sahibi | Diğer taraf: aylık PDF kopyası | Aylık mutabakat tablosu |
| GA4 mülkü, GTM konteyneri, Search Console | **Messora** | Ortak: okuma erişimi | Site ve uygulama Messora'nın |
| Dönüşüm verisi ve çevrimdışı içe aktarma (müşteri verisi) | **Messora** (veri sorumlusu) | — | KVKK gereği müşteri verisi Messora'da kalır. Ortağa yalnızca toplu raporlar gider |
| Reklam metinleri, anahtar kelime listesi, LP'ler | **Messora** (fikri mülkiyet) | Ortak: onay ve öneri hakkı | |
| Kampanya yönetimi (günlük operasyon) | Sözleşmeyle belirlenir (Messora ekibi veya ajans) | | |

---

## 13. Raporlama ritmi ve gösterge paneli

| Sıklık | İçerik | Kime |
|---|---|---|
| Günlük (ilk 14 gün), sonra iş günlerinde | Harcama hızı, politika durumu, yeni arama terimleri ve negatifler, etiket sağlığı | Kampanya yöneticisi |
| Haftalık | Kampanya ve grup bazında harcama, tıklama, TO, ort. TBM, dönüşüm (tür bazında), CPA, gösterim payı (bütçe/sıralama kaybı), yeni negatifler, test durumu | Messora + ortak (kısa e-posta) |
| İki haftada bir | Karar kuralları (§5.2) uygulaması: bütçe kaydırma, teklif değişikliği | Messora + ortak (15 dk görüşme) |
| Aylık | Deneme kohortu: aktivasyon, deneme→ücretli. CAC, geri ödeme, ads kaynaklı yeni MRR, fatura mutabakatı (harcama + YİM + KDV) | Ortaklık toplantısı |
| 30 / 60 / 90. gün | Senaryo kararı (A/B/C), bütçe revizyonu | Ortaklık onayı |

**Gösterge paneli (Looker Studio; kaynaklar Google Ads + GA4 + uygulama veritabanı özeti):**

1. Harcama (bütçe ve YİM dahil)
2. Tıklama, TO, ort. TBM
3. Arama gösterim payı ve bütçe/sıralama kaybı
4. `generate_lead`, `sign_up`, `number_connected`, `purchase` adetleri ve CPA'ları
5. Deneme→ücretli (kohort)
6. CAC, geri ödeme süresi, LTV/CAC
7. Kampanya ve anahtar kelime bazında ilk 10 ve son 10
8. Cihaz, saat ve şehir kırılımı
9. Politika ve kısıtlama durumu

---

## 14. Doğrulanamayanlar ve açık sorular

1. **Anahtar kelime hacimleri ve TBM'ler:** Türkiye'ye özgü kamuya açık bir veri bulunamadı. Tüm değerler tahmin. Keyword Planner ile değiştirilmeli.
2. **Türk benchmark kaynaklarının metodolojisi:** 11,20 TL, 82 TL ve 219 TL değerleri ajans bloglarından geliyor. Örneklem ve yöntem açıklanmıyor.
3. **Meta'nın Google'a "WhatsApp" marka şikâyeti:** Türkiye'de kayıtlı bir şikâyet olup olmadığı bilinmiyor. Kısıtlama olasılığı ancak yayına girince görülecek.
4. **Fiyat öğeleri:** Google sayfasına göre Türkçe ve TRY desteklenmiyor. Hesapta tekrar kontrol edilmeli.
5. **Yapılandırılmış snippet'ler ve potansiyel müşteri formları:** Türkçe başlık listesi ve formların Türkiye'de kullanılabilirliği doğrulanmadı.
6. **Editor CSV sütunları:** `Criterion Type` değerleri (`Exact`, `Phrase`, `Campaign Negative Phrase`), `Final URL suffix`, `Budget type` ve `Headline 1 position` başlıkları Google'ın sütun listesi ve Editor dışa aktarım biçimiyle uyumlu. Yine de Google'ın sayfası maç türü değerlerini tek tek listelemiyor. İçe aktarma ön izlemesinde kontrol edilmeli (bkz. CSV notları).
7. **Site bağlantısı açıklama sınırı (35):** Bu değer üçüncü taraf kaynaktan alındı.
8. **Yasal İşletim Maliyeti 2027 oranı:** %2,25 mi %2,5 mi olacağı kaynaklar arasında çelişkili.
9. **Deneme → ücretli oranları:** Messora'nın kendi verisi yok. Global benchmark'lar %4 ile %25 arasında geniş bir aralık veriyor.
10. **Hukuk:** SMK m.7/3-(f) ve m.12 yorumu, KVKK çerez rızası ve yurt dışı aktarım konuları avukat ve SMMM görüşüyle teyit edilmeli.

---

## Ekler

### Ek A. Anahtar kelimeler (tahmini hacim ve TBM)

Hacim, Google Anahtar Kelime Planlayıcı'nın aralık biçiminde verilmiş **tahminimizdir** (aylık, Türkiye, Türkçe). TBM aralığı TL, **tahminimizdir**; gerekçe ve kaynaklar §6.1'de. İlk hafta Planlayıcı'dan gerçek değerlerle değiştirilmelidir.

| Kampanya | Reklam grubu | Anahtar kelime | Eşleme | Tahmini aylık arama | Tahmini TBM (TL) |
|---|---|---|---|---|---|
| Messora | Marka - Messora | messora | Tam [ ] | 0-50 | 1–5 |
| Messora | Marka - Messora | messora | Sıralı " " | 0-50 | 1–5 |
| Messora | Marka - Messora | messora fiyat | Sıralı " " | 0-10 | 1–5 |
| Messora | Marka - Messora | messora whatsapp | Sıralı " " | 0-10 | 1–5 |
| WA Business API | Business API - Genel | whatsapp business api | Sıralı " " | 1K-10K | 20–60 |
| WA Business API | Business API - Genel | whatsapp business api | Tam [ ] | 1K-10K | 20–60 |
| WA Business API | Business API - Genel | whatsapp api | Sıralı " " | 1K-10K | 15–50 |
| WA Business API | Business API - Genel | whatsapp api | Tam [ ] | 1K-10K | 15–50 |
| WA Business API | Business API - Genel | whatsapp cloud api | Sıralı " " | 100-1K | 15–45 |
| WA Business API | Business API - Genel | whatsapp business api kurulumu | Sıralı " " | 100-1K | 20–60 |
| WA Business API | Business API - Genel | whatsapp api entegrasyonu | Sıralı " " | 100-1K | 20–60 |
| WA Business API | Business API - Genel | resmi whatsapp api | Sıralı " " | 100-1K | 15–50 |
| WA Business API | Business API - Genel | whatsapp business platform | Sıralı " " | 100-1K | 10–40 |
| WA Business API | Business API - Genel | whatsapp business api türkiye | Sıralı " " | 10-100 | 20–60 |
| WA Business API | API - Fiyat ve Sağlayıcı | whatsapp business api fiyatları | Sıralı " " | 100-1K | 20–60 |
| WA Business API | API - Fiyat ve Sağlayıcı | whatsapp business api fiyatları | Tam [ ] | 100-1K | 20–60 |
| WA Business API | API - Fiyat ve Sağlayıcı | whatsapp api fiyat | Sıralı " " | 100-1K | 20–60 |
| WA Business API | API - Fiyat ve Sağlayıcı | whatsapp business api ücretleri | Sıralı " " | 100-1K | 20–60 |
| WA Business API | API - Fiyat ve Sağlayıcı | whatsapp business api sağlayıcı | Sıralı " " | 10-100 | 25–70 |
| WA Business API | API - Fiyat ve Sağlayıcı | whatsapp api firmaları | Sıralı " " | 10-100 | 25–70 |
| WA Business API | API - Fiyat ve Sağlayıcı | whatsapp api paneli | Sıralı " " | 10-100 | 20–60 |
| WA Business API | Kurumsal WhatsApp | kurumsal whatsapp | Sıralı " " | 100-1K | 15–45 |
| WA Business API | Kurumsal WhatsApp | kurumsal whatsapp hattı | Sıralı " " | 10-100 | 15–45 |
| WA Business API | Kurumsal WhatsApp | whatsapp kurumsal hesap | Sıralı " " | 100-1K | 10–40 |
| WA Business API | Kurumsal WhatsApp | onaylı whatsapp hesabı | Sıralı " " | 10-100 | 10–35 |
| Toplu Mesaj & Kampanya | Toplu Mesaj | whatsapp toplu mesaj | Sıralı " " | 1K-10K | 8–30 |
| Toplu Mesaj & Kampanya | Toplu Mesaj | whatsapp toplu mesaj gönderme | Sıralı " " | 1K-10K | 8–30 |
| Toplu Mesaj & Kampanya | Toplu Mesaj | whatsapp toplu mesaj programı | Sıralı " " | 100-1K | 10–35 |
| Toplu Mesaj & Kampanya | Toplu Mesaj | whatsapp toplu mesaj yazılımı | Sıralı " " | 10-100 | 10–40 |
| Toplu Mesaj & Kampanya | Toplu Mesaj | whatsapp toplu mesaj firması | Sıralı " " | 10-100 | 10–40 |
| Toplu Mesaj & Kampanya | Toplu Mesaj | whatsapp toplu mesaj gönderme programı | Tam [ ] | 100-1K | 10–35 |
| Toplu Mesaj & Kampanya | Pazarlama ve Bildirim | whatsapp pazarlama | Sıralı " " | 100-1K | 10–40 |
| Toplu Mesaj & Kampanya | Pazarlama ve Bildirim | whatsapp kampanya mesajı | Sıralı " " | 100-1K | 10–35 |
| Toplu Mesaj & Kampanya | Pazarlama ve Bildirim | whatsapp duyuru gönderme | Sıralı " " | 10-100 | 8–30 |
| Toplu Mesaj & Kampanya | Pazarlama ve Bildirim | whatsapp bildirim mesajı | Sıralı " " | 100-1K | 10–35 |
| Toplu Mesaj & Kampanya | Pazarlama ve Bildirim | whatsapp mesaj paketleri | Sıralı " " | 100-1K | 10–35 |
| Toplu Mesaj & Kampanya | Pazarlama ve Bildirim | whatsapp reklam mesajı gönderme | Sıralı " " | 10-100 | 8–30 |
| Chatbot & AI | Chatbot | whatsapp chatbot | Sıralı " " | 100-1K | 15–45 |
| Chatbot & AI | Chatbot | whatsapp chatbot | Tam [ ] | 100-1K | 15–45 |
| Chatbot & AI | Chatbot | whatsapp bot | Sıralı " " | 1K-10K | 8–30 |
| Chatbot & AI | Chatbot | whatsapp otomatik yanıt | Sıralı " " | 1K-10K | 5–20 |
| Chatbot & AI | Chatbot | whatsapp otomasyon | Sıralı " " | 100-1K | 15–45 |
| Chatbot & AI | Chatbot | whatsapp bot yazılımı | Sıralı " " | 10-100 | 15–45 |
| Chatbot & AI | Chatbot | whatsapp otomatik mesaj sistemi | Sıralı " " | 10-100 | 10–35 |
| Chatbot & AI | Yapay Zeka Asistan | whatsapp yapay zeka | Sıralı " " | 100-1K | 10–40 |
| Chatbot & AI | Yapay Zeka Asistan | whatsapp ai asistan | Sıralı " " | 10-100 | 10–40 |
| Chatbot & AI | Yapay Zeka Asistan | yapay zeka müşteri hizmetleri | Sıralı " " | 100-1K | 15–50 |
| Chatbot & AI | Yapay Zeka Asistan | yapay zeka müşteri temsilcisi | Sıralı " " | 10-100 | 15–50 |
| Chatbot & AI | Yapay Zeka Asistan | chatgpt whatsapp entegrasyonu | Sıralı " " | 100-1K | 8–30 |
| CRM & Ekip Gelen Kutusu | WhatsApp CRM | whatsapp crm | Sıralı " " | 100-1K | 20–60 |
| CRM & Ekip Gelen Kutusu | WhatsApp CRM | whatsapp crm | Tam [ ] | 100-1K | 20–60 |
| CRM & Ekip Gelen Kutusu | WhatsApp CRM | whatsapp crm programı | Sıralı " " | 10-100 | 20–60 |
| CRM & Ekip Gelen Kutusu | WhatsApp CRM | whatsapp müşteri yönetimi | Sıralı " " | 10-100 | 15–50 |
| CRM & Ekip Gelen Kutusu | WhatsApp CRM | whatsapp satış takibi | Sıralı " " | 10-100 | 15–50 |
| CRM & Ekip Gelen Kutusu | Ekip Gelen Kutusu | whatsapp çoklu kullanıcı | Sıralı " " | 100-1K | 10–40 |
| CRM & Ekip Gelen Kutusu | Ekip Gelen Kutusu | whatsapp business çoklu kullanıcı | Sıralı " " | 100-1K | 10–40 |
| CRM & Ekip Gelen Kutusu | Ekip Gelen Kutusu | whatsapp ekip gelen kutusu | Sıralı " " | 10-100 | 15–50 |
| CRM & Ekip Gelen Kutusu | Ekip Gelen Kutusu | whatsapp müşteri hizmetleri yazılımı | Sıralı " " | 10-100 | 20–60 |
| CRM & Ekip Gelen Kutusu | Ekip Gelen Kutusu | whatsapp destek paneli | Sıralı " " | 10-100 | 15–50 |
| Sektör | Sektör - E-ticaret | e-ticaret whatsapp entegrasyonu | Sıralı " " | 10-100 | 15–50 |
| Sektör | Sektör - E-ticaret | whatsapp sipariş bildirimi | Sıralı " " | 10-100 | 10–40 |
| Sektör | Sektör - E-ticaret | whatsapp kargo bildirimi | Sıralı " " | 10-100 | 10–40 |
| Sektör | Sektör - E-ticaret | whatsapp sepet hatırlatma | Sıralı " " | 10-100 | 10–40 |
| Sektör | Sektör - Sağlık ve Klinik | whatsapp randevu hatırlatma | Sıralı " " | 10-100 | 10–40 |
| Sektör | Sektör - Sağlık ve Klinik | klinik whatsapp | Sıralı " " | 10-100 | 10–40 |
| Sektör | Sektör - Sağlık ve Klinik | whatsapp randevu sistemi | Sıralı " " | 100-1K | 10–40 |
| Sektör | Sektör - Sağlık ve Klinik | randevu hatırlatma mesajı | Sıralı " " | 100-1K | 8–30 |
| Sektör | Sektör - Turizm ve Otel | otel whatsapp | Sıralı " " | 10-100 | 10–40 |
| Sektör | Sektör - Turizm ve Otel | whatsapp rezervasyon | Sıralı " " | 100-1K | 10–40 |
| Sektör | Sektör - Turizm ve Otel | acente whatsapp | Sıralı " " | 10-100 | 10–40 |
| Sektör | Sektör - Eğitim | kurs whatsapp duyuru | Sıralı " " | 10-100 | 8–30 |
| Sektör | Sektör - Eğitim | okul whatsapp bilgilendirme | Sıralı " " | 10-100 | 8–30 |
| Sektör | Sektör - Eğitim | öğrenci bilgilendirme mesajı | Sıralı " " | 10-100 | 8–30 |
| Sektör | Sektör - Emlak | emlak whatsapp | Sıralı " " | 10-100 | 10–40 |
| Sektör | Sektör - Emlak | emlak crm whatsapp | Sıralı " " | 10-100 | 15–50 |
| Sektör | Sektör - Emlak | emlak müşteri takip yazılımı | Sıralı " " | 100-1K | 15–50 |
| Rakip Alternatif (opsiyonel) | Rakip - Global Alternatif | wati alternatifi | Sıralı " " | 0-50 | 15–50 |
| Rakip Alternatif (opsiyonel) | Rakip - Global Alternatif | respond io alternatifi | Sıralı " " | 0-50 | 15–50 |
| Rakip Alternatif (opsiyonel) | Rakip - Global Alternatif | sleekflow alternatifi | Sıralı " " | 0-50 | 15–50 |
| Rakip Alternatif (opsiyonel) | Rakip - Global Alternatif | wati | Tam [ ] | 100-1K | 10–40 |
| Rakip Alternatif (opsiyonel) | Rakip - Global Alternatif | respond io | Tam [ ] | 100-1K | 10–40 |
| Rakip Alternatif (opsiyonel) | Rakip - Global Alternatif | sleekflow | Tam [ ] | 10-100 | 10–40 |

### Ek B. RSA metinleri (karakter sayılarıyla)

Her reklam grubunda iki RSA var: **A** (ana, açık) ve **B** ("WhatsApp" geçmeyen yedek, duraklatılmış). Sayılar betikle ölçüldü (Python `len`, Unicode karakter). Sınırlar: başlık ≤30, açıklama ≤90, yol ≤15.

#### Messora › Marka - Messora — A-ana

Son URL: `https://messora.tech/` · Yol: `/deneme`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Messora | Resmi Site | 20 |
| H2 | WhatsApp için Ekip Paneli | 25 |
| H3 | Messora ile Tanışın | 19 |
| H4 | Messora Fiyatları | 17 |
| H5 | Messora'yı 14 Gün Deneyin | 25 |
| H6 | Messora Demo Talep Edin | 23 |
| H7 | 14 Gün Ücretsiz Deneyin | 23 |
| H8 | Kredi Kartı Gerekmez | 20 |
| H9 | Taahhüt Yok, Aylık Plan | 23 |
| H10 | Aylık 1.290 TL + KDV'den | 24 |
| H11 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H12 | Türkçe Destek, TL Fiyat | 23 |
| H13 | İzin (Onay) Yönetimi Dahil | 26 |
| H14 | Kendi AI Anahtarınızı Getirin | 29 |
| H15 | Kurulum Sihirbazı ile Başlayın | 30 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Messora: resmi WhatsApp Business API için ekip gelen kutusu, kampanya ve AI ajan paneli. | 88 |
| D2 | Messora'yı 14 gün ücretsiz deneyin. Kart bilgisi yok, taahhüt yok, Türkçe destek. | 81 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Messora › Marka - Messora — B-markasız (Paused)

Son URL: `https://messora.tech/` · Yol: `/deneme`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Messora | Resmi Site | 20 |
| H2 | Messora ile Tanışın | 19 |
| H3 | Messora Fiyatları | 17 |
| H4 | Messora'yı 14 Gün Deneyin | 25 |
| H5 | Messora Demo Talep Edin | 23 |
| H6 | Messora: Ekip Gelen Kutusu | 26 |
| H7 | 14 Gün Ücretsiz Deneyin | 23 |
| H8 | Kredi Kartı Gerekmez | 20 |
| H9 | Taahhüt Yok, Aylık Plan | 23 |
| H10 | Aylık 1.290 TL + KDV'den | 24 |
| H11 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H12 | Türkçe Destek, TL Fiyat | 23 |
| H13 | İzin (Onay) Yönetimi Dahil | 26 |
| H14 | Kendi AI Anahtarınızı Getirin | 29 |
| H15 | Kurulum Sihirbazı ile Başlayın | 30 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Messora: ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. | 87 |
| D2 | Messora'yı 14 gün ücretsiz deneyin. Kart bilgisi yok, taahhüt yok, Türkçe destek. | 81 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### WA Business API › Business API - Genel — A-ana

Son URL: `https://messora.tech/lp/whatsapp-business-api` · Yol: `/cloud-api/deneme`

| # | Başlık | Kar. |
|---|---|---|
| H1 | WhatsApp Business API Paneli | 28 |
| H2 | WhatsApp Cloud API ile Başla | 28 |
| H3 | Resmi WhatsApp Cloud API | 24 |
| H4 | WhatsApp API: Ekip ve AI | 24 |
| H5 | WhatsApp API Kurulum Sihirbazı | 30 |
| H6 | Resmi API, QR Bot Değil | 23 |
| H7 | Meta Cloud API Altyapısı | 24 |
| H8 | 14 Gün Ücretsiz Deneyin | 23 |
| H9 | Kredi Kartı Gerekmez | 20 |
| H10 | Taahhüt Yok, Aylık Plan | 23 |
| H11 | Aylık 1.290 TL + KDV'den | 24 |
| H12 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H13 | Türkçe Destek, TL Fiyat | 23 |
| H14 | İzin (Onay) Yönetimi Dahil | 26 |
| H15 | Kendi AI Anahtarınızı Getirin | 29 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Resmi WhatsApp Cloud API ile ekip gelen kutusu, kampanya, AI ajan ve API tek yerde. | 83 |
| D2 | Numaranızı adım adım bağlayın: işletme doğrulaması, şablon onayı ve ilk kampanya rehberli. | 90 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### WA Business API › Business API - Genel — B-markasız (Paused)

Son URL: `https://messora.tech/lp/whatsapp-business-api` · Yol: `/cloud-api/deneme`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Resmi Cloud API Altyapısı | 25 |
| H2 | Kurumsal Mesajlaşma Paneli | 26 |
| H3 | Resmi API, QR Bot Değil | 23 |
| H4 | Ekip ve AI Ajan Tek Panelde | 27 |
| H5 | Meta Cloud API Altyapısı | 24 |
| H6 | API Kurulum Sihirbazı | 21 |
| H7 | 14 Gün Ücretsiz Deneyin | 23 |
| H8 | Kredi Kartı Gerekmez | 20 |
| H9 | Taahhüt Yok, Aylık Plan | 23 |
| H10 | Aylık 1.290 TL + KDV'den | 24 |
| H11 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H12 | Türkçe Destek, TL Fiyat | 23 |
| H13 | İzin (Onay) Yönetimi Dahil | 26 |
| H14 | Kendi AI Anahtarınızı Getirin | 29 |
| H15 | Kurulum Sihirbazı ile Başlayın | 30 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Resmi Cloud API altyapısıyla ekip gelen kutusu, kampanya, AI ajan ve API tek yerde. | 83 |
| D2 | Numaranızı adım adım bağlayın: işletme doğrulaması, şablon onayı ve ilk kampanya rehberli. | 90 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### WA Business API › API - Fiyat ve Sağlayıcı — A-ana

Son URL: `https://messora.tech/fiyatlar` · Yol: `/fiyatlar`

| # | Başlık | Kar. |
|---|---|---|
| H1 | WhatsApp API Fiyatları | 22 |
| H2 | Şeffaf WhatsApp API Fiyatı | 26 |
| H3 | WhatsApp API: 1.290 TL'den | 26 |
| H4 | Paketleri Karşılaştırın | 23 |
| H5 | Başlangıç 1.290 TL + KDV | 24 |
| H6 | Büyüme 2.990 TL + KDV | 21 |
| H7 | Pro 6.490 TL + KDV | 18 |
| H8 | 14 Gün Ücretsiz Deneyin | 23 |
| H9 | Kredi Kartı Gerekmez | 20 |
| H10 | Taahhüt Yok, Aylık Plan | 23 |
| H11 | Aylık 1.290 TL + KDV'den | 24 |
| H12 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H13 | Türkçe Destek, TL Fiyat | 23 |
| H14 | İzin (Onay) Yönetimi Dahil | 26 |
| H15 | Kendi AI Anahtarınızı Getirin | 29 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Başlangıç 1.290, Büyüme 2.990, Pro 6.490 TL + KDV/ay. Yıllıkta 12 ay 10 ay fiyatına. | 84 |
| D2 | WhatsApp API sağlayıcınızı taahhüt olmadan seçin: 14 gün deneyin, beğenirseniz devam edin. | 90 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### WA Business API › API - Fiyat ve Sağlayıcı — B-markasız (Paused)

Son URL: `https://messora.tech/fiyatlar` · Yol: `/fiyatlar`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Şeffaf Plan Fiyatları | 21 |
| H2 | Paketleri Karşılaştırın | 23 |
| H3 | Başlangıç 1.290 TL + KDV | 24 |
| H4 | Büyüme 2.990 TL + KDV | 21 |
| H5 | Pro 6.490 TL + KDV | 18 |
| H6 | Gizli Kurulum Ücreti Yok | 24 |
| H7 | 14 Gün Ücretsiz Deneyin | 23 |
| H8 | Kredi Kartı Gerekmez | 20 |
| H9 | Taahhüt Yok, Aylık Plan | 23 |
| H10 | Aylık 1.290 TL + KDV'den | 24 |
| H11 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H12 | Türkçe Destek, TL Fiyat | 23 |
| H13 | İzin (Onay) Yönetimi Dahil | 26 |
| H14 | Kendi AI Anahtarınızı Getirin | 29 |
| H15 | Kurulum Sihirbazı ile Başlayın | 30 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Başlangıç 1.290, Büyüme 2.990, Pro 6.490 TL + KDV/ay. Yıllıkta 12 ay 10 ay fiyatına. | 84 |
| D2 | Sağlayıcınızı taahhüt olmadan seçin: 14 gün ücretsiz deneyin, beğenirseniz devam edin. | 86 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### WA Business API › Kurumsal WhatsApp — A-ana

Son URL: `https://messora.tech/lp/whatsapp-business-api` · Yol: `/kurumsal`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Kurumsal WhatsApp Paneli | 24 |
| H2 | Kurumsal WhatsApp Hattı | 23 |
| H3 | WhatsApp: Tek Numara, Çok Ekip | 30 |
| H4 | Kurumsal Mesajlaşma Paneli | 26 |
| H5 | Resmi Cloud API Altyapısı | 25 |
| H6 | Rol ve Yetki Yönetimi | 21 |
| H7 | 14 Gün Ücretsiz Deneyin | 23 |
| H8 | Kredi Kartı Gerekmez | 20 |
| H9 | Taahhüt Yok, Aylık Plan | 23 |
| H10 | Aylık 1.290 TL + KDV'den | 24 |
| H11 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H12 | Türkçe Destek, TL Fiyat | 23 |
| H13 | İzin (Onay) Yönetimi Dahil | 26 |
| H14 | Kendi AI Anahtarınızı Getirin | 29 |
| H15 | Kurulum Sihirbazı ile Başlayın | 30 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Kurumsal WhatsApp hattınızı ekibinizle paylaşın: atama, etiket, not ve rol yetkileri. | 85 |
| D2 | Resmi Cloud API ile çalışır. Mesaj geçmişi, raporlar ve API erişimi tek panelde. | 80 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### WA Business API › Kurumsal WhatsApp — B-markasız (Paused)

Son URL: `https://messora.tech/lp/whatsapp-business-api` · Yol: `/kurumsal`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Kurumsal Mesajlaşma Paneli | 26 |
| H2 | Tek Numara, Çok Kullanıcı | 25 |
| H3 | Resmi Cloud API Altyapısı | 25 |
| H4 | Rol ve Yetki Yönetimi | 21 |
| H5 | Ekip Performans Raporları | 25 |
| H6 | 14 Gün Ücretsiz Deneyin | 23 |
| H7 | Kredi Kartı Gerekmez | 20 |
| H8 | Taahhüt Yok, Aylık Plan | 23 |
| H9 | Aylık 1.290 TL + KDV'den | 24 |
| H10 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H11 | Türkçe Destek, TL Fiyat | 23 |
| H12 | İzin (Onay) Yönetimi Dahil | 26 |
| H13 | Kendi AI Anahtarınızı Getirin | 29 |
| H14 | Kurulum Sihirbazı ile Başlayın | 30 |
| H15 | KOBİ'ler İçin Tasarlandı | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Kurumsal hattınızı ekibinizle paylaşın: konuşma atama, etiket, not ve rol yetkileri. | 84 |
| D2 | Resmi Cloud API ile çalışır. Mesaj geçmişi, raporlar ve API erişimi tek panelde. | 80 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Toplu Mesaj & Kampanya › Toplu Mesaj — A-ana

Son URL: `https://messora.tech/lp/whatsapp-toplu-mesaj` · Yol: `/toplu-mesaj/izinli`

| # | Başlık | Kar. |
|---|---|---|
| H1 | WhatsApp Toplu Mesaj (Resmi) | 28 |
| H2 | İzinli WhatsApp Kampanyaları | 28 |
| H3 | Onaylı Şablonla Toplu Gönderim | 30 |
| H4 | Resmi API ile Toplu Mesaj | 25 |
| H5 | Teslim ve Okunma Raporu | 23 |
| H6 | Segmentli Kampanya Gönderimi | 28 |
| H7 | İzin Kaydı Otomatik Tutulur | 27 |
| H8 | 14 Gün Ücretsiz Deneyin | 23 |
| H9 | Kredi Kartı Gerekmez | 20 |
| H10 | Taahhüt Yok, Aylık Plan | 23 |
| H11 | Aylık 1.290 TL + KDV'den | 24 |
| H12 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H13 | Türkçe Destek, TL Fiyat | 23 |
| H14 | İzin (Onay) Yönetimi Dahil | 26 |
| H15 | Kendi AI Anahtarınızı Getirin | 29 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Resmi API ile izinli toplu WhatsApp kampanyası: segment, zamanlama, teslim raporu. | 82 |
| D2 | İzin kayıtları otomatik tutulur, ret talebi anında işlenir. Kalite puanınızı koruyun. | 85 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Toplu Mesaj & Kampanya › Toplu Mesaj — B-markasız (Paused)

Son URL: `https://messora.tech/lp/whatsapp-toplu-mesaj` · Yol: `/toplu-mesaj/izinli`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Resmi API ile Toplu Mesaj | 25 |
| H2 | İzinli Kampanya Gönderimi | 25 |
| H3 | Teslim ve Okunma Raporu | 23 |
| H4 | Segmentli Kampanya Gönderimi | 28 |
| H5 | İzin Kaydı Otomatik Tutulur | 27 |
| H6 | Onaylı Şablonlarla Gönderim | 27 |
| H7 | 14 Gün Ücretsiz Deneyin | 23 |
| H8 | Kredi Kartı Gerekmez | 20 |
| H9 | Taahhüt Yok, Aylık Plan | 23 |
| H10 | Aylık 1.290 TL + KDV'den | 24 |
| H11 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H12 | Türkçe Destek, TL Fiyat | 23 |
| H13 | İzin (Onay) Yönetimi Dahil | 26 |
| H14 | Kendi AI Anahtarınızı Getirin | 29 |
| H15 | Kurulum Sihirbazı ile Başlayın | 30 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Resmi API ile izinli toplu kampanya: segment, zamanlama, teslim ve okunma raporu. | 81 |
| D2 | İzin kayıtları otomatik tutulur, ret talebi anında işlenir. Kalite puanınızı koruyun. | 85 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Toplu Mesaj & Kampanya › Pazarlama ve Bildirim — A-ana

Son URL: `https://messora.tech/lp/whatsapp-toplu-mesaj` · Yol: `/kampanya`

| # | Başlık | Kar. |
|---|---|---|
| H1 | WhatsApp ile Pazarlama | 22 |
| H2 | WhatsApp Kampanya Mesajları | 27 |
| H3 | SMS Yerine WhatsApp Kampanya | 28 |
| H4 | Görselli ve Butonlu Mesajlar | 28 |
| H5 | Kampanya Analitiği ve A/B | 25 |
| H6 | İzinli Kampanya Gönderimi | 25 |
| H7 | 14 Gün Ücretsiz Deneyin | 23 |
| H8 | Kredi Kartı Gerekmez | 20 |
| H9 | Taahhüt Yok, Aylık Plan | 23 |
| H10 | Aylık 1.290 TL + KDV'den | 24 |
| H11 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H12 | Türkçe Destek, TL Fiyat | 23 |
| H13 | İzin (Onay) Yönetimi Dahil | 26 |
| H14 | Kendi AI Anahtarınızı Getirin | 29 |
| H15 | Kurulum Sihirbazı ile Başlayın | 30 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Görselli, butonlu ve carousel şablonlarla kampanya gönderin; yanıtlar ekibinize düşsün. | 87 |
| D2 | Teslim, okunma ve yanıt oranlarını kampanya bazında ölçün, segmentleri karşılaştırın. | 85 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Toplu Mesaj & Kampanya › Pazarlama ve Bildirim — B-markasız (Paused)

Son URL: `https://messora.tech/lp/whatsapp-toplu-mesaj` · Yol: `/kampanya`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Görselli ve Butonlu Mesajlar | 28 |
| H2 | Kampanya Analitiği ve A/B | 25 |
| H3 | İzinli Kampanya Gönderimi | 25 |
| H4 | Onaylı Şablonlarla Gönderim | 27 |
| H5 | Sepet ve Sipariş Bildirimi | 26 |
| H6 | 14 Gün Ücretsiz Deneyin | 23 |
| H7 | Kredi Kartı Gerekmez | 20 |
| H8 | Taahhüt Yok, Aylık Plan | 23 |
| H9 | Aylık 1.290 TL + KDV'den | 24 |
| H10 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H11 | Türkçe Destek, TL Fiyat | 23 |
| H12 | İzin (Onay) Yönetimi Dahil | 26 |
| H13 | Kendi AI Anahtarınızı Getirin | 29 |
| H14 | Kurulum Sihirbazı ile Başlayın | 30 |
| H15 | KOBİ'ler İçin Tasarlandı | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Görselli, butonlu ve carousel şablonlarla kampanya gönderin; yanıtlar ekibinize düşsün. | 87 |
| D2 | Teslim, okunma ve yanıt oranlarını kampanya bazında ölçün, segmentleri karşılaştırın. | 85 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Chatbot & AI › Chatbot — A-ana

Son URL: `https://messora.tech/lp/whatsapp-chatbot` · Yol: `/chatbot/ai-ajan`

| # | Başlık | Kar. |
|---|---|---|
| H1 | WhatsApp Chatbot ve AI Ajan | 27 |
| H2 | 7/24 Yanıt Veren AI Ajan | 24 |
| H3 | Zor Soruda İnsana Devir | 23 |
| H4 | WhatsApp Otomasyonları | 22 |
| H5 | Kodsuz Akış Tasarımcısı | 23 |
| H6 | Kendi Bilgi Tabanınızla AI | 26 |
| H7 | 14 Gün Ücretsiz Deneyin | 23 |
| H8 | Kredi Kartı Gerekmez | 20 |
| H9 | Taahhüt Yok, Aylık Plan | 23 |
| H10 | Aylık 1.290 TL + KDV'den | 24 |
| H11 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H12 | Türkçe Destek, TL Fiyat | 23 |
| H13 | İzin (Onay) Yönetimi Dahil | 26 |
| H14 | Kendi AI Anahtarınızı Getirin | 29 |
| H15 | Kurulum Sihirbazı ile Başlayın | 30 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | AI ajan sık soruları 7/24 yanıtlar, gerektiğinde konuşmayı ekibinize devreder. | 78 |
| D2 | Kodsuz akış tasarımcısı ve hazır şablonlarla WhatsApp otomasyonlarınızı kurun. | 78 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Chatbot & AI › Chatbot — B-markasız (Paused)

Son URL: `https://messora.tech/lp/whatsapp-chatbot` · Yol: `/chatbot/ai-ajan`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Chatbot ve AI Ajan | 18 |
| H2 | 7/24 Yanıt Veren AI Ajan | 24 |
| H3 | Zor Soruda İnsana Devir | 23 |
| H4 | Kodsuz Akış Tasarımcısı | 23 |
| H5 | Kendi Bilgi Tabanınızla AI | 26 |
| H6 | 14 Gün Ücretsiz Deneyin | 23 |
| H7 | Kredi Kartı Gerekmez | 20 |
| H8 | Taahhüt Yok, Aylık Plan | 23 |
| H9 | Aylık 1.290 TL + KDV'den | 24 |
| H10 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H11 | Türkçe Destek, TL Fiyat | 23 |
| H12 | İzin (Onay) Yönetimi Dahil | 26 |
| H13 | Kendi AI Anahtarınızı Getirin | 29 |
| H14 | Kurulum Sihirbazı ile Başlayın | 30 |
| H15 | KOBİ'ler İçin Tasarlandı | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | AI ajan sık soruları 7/24 yanıtlar, gerektiğinde konuşmayı ekibinize devreder. | 78 |
| D2 | Kodsuz akış tasarımcısı ve hazır şablonlarla otomasyonlarınızı adım adım kurun. | 79 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Chatbot & AI › Yapay Zeka Asistan — A-ana

Son URL: `https://messora.tech/lp/whatsapp-chatbot` · Yol: `/yapay-zeka`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Yapay Zeka Müşteri Temsilcisi | 29 |
| H2 | WhatsApp'ta AI Asistan | 22 |
| H3 | Kendi AI Anahtarınızla Çalışın | 30 |
| H4 | AI Ajan + İnsan Devri | 21 |
| H5 | Sık Soruları AI Yanıtlasın | 26 |
| H6 | 14 Gün Ücretsiz Deneyin | 23 |
| H7 | Kredi Kartı Gerekmez | 20 |
| H8 | Taahhüt Yok, Aylık Plan | 23 |
| H9 | Aylık 1.290 TL + KDV'den | 24 |
| H10 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H11 | Türkçe Destek, TL Fiyat | 23 |
| H12 | İzin (Onay) Yönetimi Dahil | 26 |
| H13 | Kendi AI Anahtarınızı Getirin | 29 |
| H14 | Kurulum Sihirbazı ile Başlayın | 30 |
| H15 | KOBİ'ler İçin Tasarlandı | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Kendi AI sağlayıcı anahtarınızı bağlayın; bu kullanımda Messora AI farkı almaz. | 79 |
| D2 | AI ajan bilgi tabanınızdan yanıtlar, emin olmadığında konuşmayı temsilciye aktarır. | 83 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Chatbot & AI › Yapay Zeka Asistan — B-markasız (Paused)

Son URL: `https://messora.tech/lp/whatsapp-chatbot` · Yol: `/yapay-zeka`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Yapay Zeka Müşteri Temsilcisi | 29 |
| H2 | Kendi AI Anahtarınızla Çalışın | 30 |
| H3 | AI Ajan + İnsan Devri | 21 |
| H4 | Sık Soruları AI Yanıtlasın | 26 |
| H5 | Türkçe Yanıt Veren AI Ajan | 26 |
| H6 | 14 Gün Ücretsiz Deneyin | 23 |
| H7 | Kredi Kartı Gerekmez | 20 |
| H8 | Taahhüt Yok, Aylık Plan | 23 |
| H9 | Aylık 1.290 TL + KDV'den | 24 |
| H10 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H11 | Türkçe Destek, TL Fiyat | 23 |
| H12 | İzin (Onay) Yönetimi Dahil | 26 |
| H13 | Kendi AI Anahtarınızı Getirin | 29 |
| H14 | Kurulum Sihirbazı ile Başlayın | 30 |
| H15 | KOBİ'ler İçin Tasarlandı | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Kendi AI sağlayıcı anahtarınızı bağlayın; BYO kullanımda Messora AI farkı almaz. | 80 |
| D2 | AI ajan bilgi tabanınızdan yanıtlar, emin olmadığında konuşmayı temsilciye aktarır. | 83 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### CRM & Ekip Gelen Kutusu › WhatsApp CRM — A-ana

Son URL: `https://messora.tech/lp/whatsapp-crm` · Yol: `/crm/ekip`

| # | Başlık | Kar. |
|---|---|---|
| H1 | WhatsApp CRM ve Gelen Kutusu | 28 |
| H2 | Müşteri Geçmişi Tek Ekranda | 27 |
| H3 | Etiket, Not ve Atama | 20 |
| H4 | WhatsApp Satış Takibi | 21 |
| H5 | Ekip Performans Raporları | 25 |
| H6 | 14 Gün Ücretsiz Deneyin | 23 |
| H7 | Kredi Kartı Gerekmez | 20 |
| H8 | Taahhüt Yok, Aylık Plan | 23 |
| H9 | Aylık 1.290 TL + KDV'den | 24 |
| H10 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H11 | Türkçe Destek, TL Fiyat | 23 |
| H12 | İzin (Onay) Yönetimi Dahil | 26 |
| H13 | Kendi AI Anahtarınızı Getirin | 29 |
| H14 | Kurulum Sihirbazı ile Başlayın | 30 |
| H15 | KOBİ'ler İçin Tasarlandı | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Her müşterinin konuşma geçmişi, etiketleri ve notları tek ekranda. Ekibinizle paylaşın. | 87 |
| D2 | Konuşmaları temsilcilere atayın, yanıt sürelerini ölçün, satış fırsatlarını kaçırmayın. | 87 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### CRM & Ekip Gelen Kutusu › WhatsApp CRM — B-markasız (Paused)

Son URL: `https://messora.tech/lp/whatsapp-crm` · Yol: `/crm/ekip`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Mesajlaşma CRM'i | 16 |
| H2 | Müşteri Geçmişi Tek Ekranda | 27 |
| H3 | Etiket, Not ve Atama | 20 |
| H4 | Ekip Performans Raporları | 25 |
| H5 | Satış Takibi Tek Panelde | 24 |
| H6 | 14 Gün Ücretsiz Deneyin | 23 |
| H7 | Kredi Kartı Gerekmez | 20 |
| H8 | Taahhüt Yok, Aylık Plan | 23 |
| H9 | Aylık 1.290 TL + KDV'den | 24 |
| H10 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H11 | Türkçe Destek, TL Fiyat | 23 |
| H12 | İzin (Onay) Yönetimi Dahil | 26 |
| H13 | Kendi AI Anahtarınızı Getirin | 29 |
| H14 | Kurulum Sihirbazı ile Başlayın | 30 |
| H15 | KOBİ'ler İçin Tasarlandı | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Her müşterinin konuşma geçmişi, etiketleri ve notları tek ekranda. Ekibinizle paylaşın. | 87 |
| D2 | Konuşmaları temsilcilere atayın, yanıt sürelerini ölçün, satış fırsatlarını kaçırmayın. | 87 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### CRM & Ekip Gelen Kutusu › Ekip Gelen Kutusu — A-ana

Son URL: `https://messora.tech/lp/whatsapp-crm` · Yol: `/gelen-kutusu`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Tek Numara, Çok Kullanıcı | 25 |
| H2 | WhatsApp Ekip Gelen Kutusu | 26 |
| H3 | Konuşma Atama ve Etiket | 23 |
| H4 | Yanıt Süresi Raporları | 22 |
| H5 | WhatsApp Destek Paneli | 22 |
| H6 | 14 Gün Ücretsiz Deneyin | 23 |
| H7 | Kredi Kartı Gerekmez | 20 |
| H8 | Taahhüt Yok, Aylık Plan | 23 |
| H9 | Aylık 1.290 TL + KDV'den | 24 |
| H10 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H11 | Türkçe Destek, TL Fiyat | 23 |
| H12 | İzin (Onay) Yönetimi Dahil | 26 |
| H13 | Kendi AI Anahtarınızı Getirin | 29 |
| H14 | Kurulum Sihirbazı ile Başlayın | 30 |
| H15 | KOBİ'ler İçin Tasarlandı | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Tek WhatsApp numarasını tüm ekip kullansın: atama, etiket, iç not ve yetki yönetimi. | 84 |
| D2 | Telefon paylaşmaya son. Temsilciler kendi hesabıyla girer, yöneticiler raporları izler. | 87 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### CRM & Ekip Gelen Kutusu › Ekip Gelen Kutusu — B-markasız (Paused)

Son URL: `https://messora.tech/lp/whatsapp-crm` · Yol: `/gelen-kutusu`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Tek Numara, Çok Kullanıcı | 25 |
| H2 | Konuşma Atama ve Etiket | 23 |
| H3 | Yanıt Süresi Raporları | 22 |
| H4 | Ortak Gelen Kutusu | 18 |
| H5 | Rol ve Yetki Yönetimi | 21 |
| H6 | 14 Gün Ücretsiz Deneyin | 23 |
| H7 | Kredi Kartı Gerekmez | 20 |
| H8 | Taahhüt Yok, Aylık Plan | 23 |
| H9 | Aylık 1.290 TL + KDV'den | 24 |
| H10 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H11 | Türkçe Destek, TL Fiyat | 23 |
| H12 | İzin (Onay) Yönetimi Dahil | 26 |
| H13 | Kendi AI Anahtarınızı Getirin | 29 |
| H14 | Kurulum Sihirbazı ile Başlayın | 30 |
| H15 | KOBİ'ler İçin Tasarlandı | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Tek numarayı tüm ekip kullansın: konuşma atama, etiket, iç not ve yetki yönetimi. | 81 |
| D2 | Telefon paylaşmaya son. Temsilciler kendi hesabıyla girer, yöneticiler raporları izler. | 87 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Sektör › Sektör - E-ticaret — A-ana

Son URL: `https://messora.tech/cozumler/e-ticaret` · Yol: `/e-ticaret`

| # | Başlık | Kar. |
|---|---|---|
| H1 | E-ticaret için WhatsApp | 23 |
| H2 | Sipariş ve Kargo Bildirimi | 26 |
| H3 | Sepet Hatırlatma Mesajı | 23 |
| H4 | Satış Sonrası Destek Paneli | 27 |
| H5 | 14 Gün Ücretsiz Deneyin | 23 |
| H6 | Kredi Kartı Gerekmez | 20 |
| H7 | Taahhüt Yok, Aylık Plan | 23 |
| H8 | Aylık 1.290 TL + KDV'den | 24 |
| H9 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H10 | Türkçe Destek, TL Fiyat | 23 |
| H11 | İzin (Onay) Yönetimi Dahil | 26 |
| H12 | Kendi AI Anahtarınızı Getirin | 29 |
| H13 | Kurulum Sihirbazı ile Başlayın | 30 |
| H14 | KOBİ'ler İçin Tasarlandı | 24 |
| H15 | Kampanya Analitiği Dahil | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Sipariş, kargo ve sepet hatırlatma mesajları otomatik gitsin; soruları ekip yanıtlasın. | 87 |
| D2 | API ve webhook ile mağazanızı bağlayın. 14 gün ücretsiz deneyin, taahhüt yok. | 77 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Sektör › Sektör - E-ticaret — B-markasız (Paused)

Son URL: `https://messora.tech/cozumler/e-ticaret` · Yol: `/e-ticaret`

| # | Başlık | Kar. |
|---|---|---|
| H1 | E-ticaret Mesajlaşma Paneli | 27 |
| H2 | Sipariş ve Kargo Bildirimi | 26 |
| H3 | Sepet Hatırlatma Mesajı | 23 |
| H4 | Satış Sonrası Destek Paneli | 27 |
| H5 | 14 Gün Ücretsiz Deneyin | 23 |
| H6 | Kredi Kartı Gerekmez | 20 |
| H7 | Taahhüt Yok, Aylık Plan | 23 |
| H8 | Aylık 1.290 TL + KDV'den | 24 |
| H9 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H10 | Türkçe Destek, TL Fiyat | 23 |
| H11 | İzin (Onay) Yönetimi Dahil | 26 |
| H12 | Kendi AI Anahtarınızı Getirin | 29 |
| H13 | Kurulum Sihirbazı ile Başlayın | 30 |
| H14 | KOBİ'ler İçin Tasarlandı | 24 |
| H15 | Kampanya Analitiği Dahil | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Sipariş, kargo ve sepet hatırlatma mesajları otomatik gitsin; soruları ekip yanıtlasın. | 87 |
| D2 | API ve webhook ile mağazanızı bağlayın. 14 gün ücretsiz deneyin, taahhüt yok. | 77 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Sektör › Sektör - Sağlık ve Klinik — A-ana

Son URL: `https://messora.tech/cozumler/saglik-ve-klinikler` · Yol: `/klinik`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Klinikler için WhatsApp | 23 |
| H2 | Randevu Hatırlatma Mesajı | 25 |
| H3 | Randevu Teyidi Butonla | 22 |
| H4 | Hasta Sorularına Hızlı Yanıt | 28 |
| H5 | 14 Gün Ücretsiz Deneyin | 23 |
| H6 | Kredi Kartı Gerekmez | 20 |
| H7 | Taahhüt Yok, Aylık Plan | 23 |
| H8 | Aylık 1.290 TL + KDV'den | 24 |
| H9 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H10 | Türkçe Destek, TL Fiyat | 23 |
| H11 | İzin (Onay) Yönetimi Dahil | 26 |
| H12 | Kendi AI Anahtarınızı Getirin | 29 |
| H13 | Kurulum Sihirbazı ile Başlayın | 30 |
| H14 | KOBİ'ler İçin Tasarlandı | 24 |
| H15 | Kampanya Analitiği Dahil | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Randevu hatırlatma ve teyit mesajlarını otomatik gönderin, gelmeme oranını düşürün. | 83 |
| D2 | Ekip gelen kutusu ve AI ajan ile soruları hızla yanıtlayın. 14 gün ücretsiz deneyin. | 84 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Sektör › Sektör - Sağlık ve Klinik — B-markasız (Paused)

Son URL: `https://messora.tech/cozumler/saglik-ve-klinikler` · Yol: `/klinik`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Klinik Mesajlaşma Paneli | 24 |
| H2 | Randevu Hatırlatma Mesajı | 25 |
| H3 | Randevu Teyidi Butonla | 22 |
| H4 | Danışan Sorularına Hızlı Yanıt | 30 |
| H5 | 14 Gün Ücretsiz Deneyin | 23 |
| H6 | Kredi Kartı Gerekmez | 20 |
| H7 | Taahhüt Yok, Aylık Plan | 23 |
| H8 | Aylık 1.290 TL + KDV'den | 24 |
| H9 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H10 | Türkçe Destek, TL Fiyat | 23 |
| H11 | İzin (Onay) Yönetimi Dahil | 26 |
| H12 | Kendi AI Anahtarınızı Getirin | 29 |
| H13 | Kurulum Sihirbazı ile Başlayın | 30 |
| H14 | KOBİ'ler İçin Tasarlandı | 24 |
| H15 | Kampanya Analitiği Dahil | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Randevu hatırlatma ve teyit mesajlarını otomatik gönderin, gelmeme oranını düşürün. | 83 |
| D2 | Ekip gelen kutusu ve AI ajan ile soruları hızla yanıtlayın. 14 gün ücretsiz deneyin. | 84 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Sektör › Sektör - Turizm ve Otel — A-ana

Son URL: `https://messora.tech/cozumler/turizm-ve-otel` · Yol: `/otel`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Oteller için WhatsApp | 21 |
| H2 | Rezervasyon Soruları Tek Yerde | 30 |
| H3 | Check-in Bilgisi Otomatik | 25 |
| H4 | Misafire 7/24 AI Yanıt | 22 |
| H5 | 14 Gün Ücretsiz Deneyin | 23 |
| H6 | Kredi Kartı Gerekmez | 20 |
| H7 | Taahhüt Yok, Aylık Plan | 23 |
| H8 | Aylık 1.290 TL + KDV'den | 24 |
| H9 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H10 | Türkçe Destek, TL Fiyat | 23 |
| H11 | İzin (Onay) Yönetimi Dahil | 26 |
| H12 | Kendi AI Anahtarınızı Getirin | 29 |
| H13 | Kurulum Sihirbazı ile Başlayın | 30 |
| H14 | KOBİ'ler İçin Tasarlandı | 24 |
| H15 | Kampanya Analitiği Dahil | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Rezervasyon sorularını ekip ve AI ajan yanıtlasın; check-in bilgisi otomatik gitsin. | 84 |
| D2 | Sezonda yoğunluğa hazır: konuşma atama, hazır yanıtlar ve raporlar. 14 gün deneyin. | 83 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Sektör › Sektör - Turizm ve Otel — B-markasız (Paused)

Son URL: `https://messora.tech/cozumler/turizm-ve-otel` · Yol: `/otel`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Otel Mesajlaşma Paneli | 22 |
| H2 | Rezervasyon Soruları Tek Yerde | 30 |
| H3 | Check-in Bilgisi Otomatik | 25 |
| H4 | Misafire 7/24 AI Yanıt | 22 |
| H5 | 14 Gün Ücretsiz Deneyin | 23 |
| H6 | Kredi Kartı Gerekmez | 20 |
| H7 | Taahhüt Yok, Aylık Plan | 23 |
| H8 | Aylık 1.290 TL + KDV'den | 24 |
| H9 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H10 | Türkçe Destek, TL Fiyat | 23 |
| H11 | İzin (Onay) Yönetimi Dahil | 26 |
| H12 | Kendi AI Anahtarınızı Getirin | 29 |
| H13 | Kurulum Sihirbazı ile Başlayın | 30 |
| H14 | KOBİ'ler İçin Tasarlandı | 24 |
| H15 | Kampanya Analitiği Dahil | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Rezervasyon sorularını ekip ve AI ajan yanıtlasın; check-in bilgisi otomatik gitsin. | 84 |
| D2 | Sezonda yoğunluğa hazır: konuşma atama, hazır yanıtlar ve raporlar. 14 gün deneyin. | 83 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Sektör › Sektör - Eğitim — A-ana

Son URL: `https://messora.tech/cozumler/egitim` · Yol: `/egitim`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Kurslar için WhatsApp | 21 |
| H2 | Veli ve Öğrenci Bilgilendirme | 29 |
| H3 | Kayıt Sorularına Hızlı Yanıt | 28 |
| H4 | Ders ve Ödeme Hatırlatma | 24 |
| H5 | 14 Gün Ücretsiz Deneyin | 23 |
| H6 | Kredi Kartı Gerekmez | 20 |
| H7 | Taahhüt Yok, Aylık Plan | 23 |
| H8 | Aylık 1.290 TL + KDV'den | 24 |
| H9 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H10 | Türkçe Destek, TL Fiyat | 23 |
| H11 | İzin (Onay) Yönetimi Dahil | 26 |
| H12 | Kendi AI Anahtarınızı Getirin | 29 |
| H13 | Kurulum Sihirbazı ile Başlayın | 30 |
| H14 | KOBİ'ler İçin Tasarlandı | 24 |
| H15 | Kampanya Analitiği Dahil | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Kayıt sorularını ekip ve AI ajan yanıtlasın; ders ve ödeme hatırlatmaları otomatik gitsin. | 90 |
| D2 | İzinli duyuru listeleri, şablon oluşturucu ve raporlar. 14 gün ücretsiz deneyin. | 80 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Sektör › Sektör - Eğitim — B-markasız (Paused)

Son URL: `https://messora.tech/cozumler/egitim` · Yol: `/egitim`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Kurs Mesajlaşma Paneli | 22 |
| H2 | Veli ve Öğrenci Bilgilendirme | 29 |
| H3 | Kayıt Sorularına Hızlı Yanıt | 28 |
| H4 | Ders ve Ödeme Hatırlatma | 24 |
| H5 | 14 Gün Ücretsiz Deneyin | 23 |
| H6 | Kredi Kartı Gerekmez | 20 |
| H7 | Taahhüt Yok, Aylık Plan | 23 |
| H8 | Aylık 1.290 TL + KDV'den | 24 |
| H9 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H10 | Türkçe Destek, TL Fiyat | 23 |
| H11 | İzin (Onay) Yönetimi Dahil | 26 |
| H12 | Kendi AI Anahtarınızı Getirin | 29 |
| H13 | Kurulum Sihirbazı ile Başlayın | 30 |
| H14 | KOBİ'ler İçin Tasarlandı | 24 |
| H15 | Kampanya Analitiği Dahil | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | Kayıt sorularını ekip ve AI ajan yanıtlasın; ders ve ödeme hatırlatmaları otomatik gitsin. | 90 |
| D2 | İzinli duyuru listeleri, şablon oluşturucu ve raporlar. 14 gün ücretsiz deneyin. | 80 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Sektör › Sektör - Emlak — A-ana

Son URL: `https://messora.tech/cozumler/emlak` · Yol: `/emlak`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Emlakçılar için WhatsApp | 24 |
| H2 | İlan Sorularını Kaçırmayın | 26 |
| H3 | Müşteri Takibi Tek Panelde | 26 |
| H4 | Danışmanlara Otomatik Atama | 27 |
| H5 | 14 Gün Ücretsiz Deneyin | 23 |
| H6 | Kredi Kartı Gerekmez | 20 |
| H7 | Taahhüt Yok, Aylık Plan | 23 |
| H8 | Aylık 1.290 TL + KDV'den | 24 |
| H9 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H10 | Türkçe Destek, TL Fiyat | 23 |
| H11 | İzin (Onay) Yönetimi Dahil | 26 |
| H12 | Kendi AI Anahtarınızı Getirin | 29 |
| H13 | Kurulum Sihirbazı ile Başlayın | 30 |
| H14 | KOBİ'ler İçin Tasarlandı | 24 |
| H15 | Kampanya Analitiği Dahil | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | İlan sorularını danışmanlara otomatik atayın, müşteri geçmişini tek ekranda izleyin. | 84 |
| D2 | Ekip gelen kutusu, etiketler ve hatırlatmalar. 14 gün ücretsiz deneyin, taahhüt yok. | 84 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Sektör › Sektör - Emlak — B-markasız (Paused)

Son URL: `https://messora.tech/cozumler/emlak` · Yol: `/emlak`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Emlak Mesajlaşma Paneli | 23 |
| H2 | İlan Sorularını Kaçırmayın | 26 |
| H3 | Müşteri Takibi Tek Panelde | 26 |
| H4 | Danışmanlara Otomatik Atama | 27 |
| H5 | 14 Gün Ücretsiz Deneyin | 23 |
| H6 | Kredi Kartı Gerekmez | 20 |
| H7 | Taahhüt Yok, Aylık Plan | 23 |
| H8 | Aylık 1.290 TL + KDV'den | 24 |
| H9 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H10 | Türkçe Destek, TL Fiyat | 23 |
| H11 | İzin (Onay) Yönetimi Dahil | 26 |
| H12 | Kendi AI Anahtarınızı Getirin | 29 |
| H13 | Kurulum Sihirbazı ile Başlayın | 30 |
| H14 | KOBİ'ler İçin Tasarlandı | 24 |
| H15 | Kampanya Analitiği Dahil | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | İlan sorularını danışmanlara otomatik atayın, müşteri geçmişini tek ekranda izleyin. | 84 |
| D2 | Ekip gelen kutusu, etiketler ve hatırlatmalar. 14 gün ücretsiz deneyin, taahhüt yok. | 84 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Rakip Alternatif (opsiyonel) › Rakip - Global Alternatif — A-ana

Son URL: `https://messora.tech/fiyatlar` · Yol: `/alternatif/fiyatlar`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Yerli ve TL Fiyatlı Alternatif | 30 |
| H2 | Dolar Değil, TL Fiyat | 21 |
| H3 | Taahhütsüz Alternatif | 21 |
| H4 | Türkçe Destekli Alternatif | 26 |
| H5 | Planlar 1.290 TL + KDV'den | 26 |
| H6 | 14 Gün Ücretsiz Deneyin | 23 |
| H7 | Kredi Kartı Gerekmez | 20 |
| H8 | Taahhüt Yok, Aylık Plan | 23 |
| H9 | Aylık 1.290 TL + KDV'den | 24 |
| H10 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H11 | Türkçe Destek, TL Fiyat | 23 |
| H12 | İzin (Onay) Yönetimi Dahil | 26 |
| H13 | Kendi AI Anahtarınızı Getirin | 29 |
| H14 | Kurulum Sihirbazı ile Başlayın | 30 |
| H15 | KOBİ'ler İçin Tasarlandı | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | TL fiyat, Türkçe destek ve taahhütsüz plan. Ekip gelen kutusu, kampanya ve AI ajan dahil. | 89 |
| D2 | 14 gün ücretsiz deneyin, mevcut aracınızla karşılaştırın. Kurulum sihirbazı ile başlayın. | 89 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

#### Rakip Alternatif (opsiyonel) › Rakip - Global Alternatif — B-markasız (Paused)

Son URL: `https://messora.tech/fiyatlar` · Yol: `/alternatif/fiyatlar`

| # | Başlık | Kar. |
|---|---|---|
| H1 | Yerli ve TL Fiyatlı Alternatif | 30 |
| H2 | Dolar Değil, TL Fiyat | 21 |
| H3 | Türkçe Destekli Alternatif | 26 |
| H4 | Planlar 1.290 TL + KDV'den | 26 |
| H5 | 14 Gün Ücretsiz Deneyin | 23 |
| H6 | Kredi Kartı Gerekmez | 20 |
| H7 | Taahhüt Yok, Aylık Plan | 23 |
| H8 | Aylık 1.290 TL + KDV'den | 24 |
| H9 | Yıllık Ödemede 2 Ay Bizden | 26 |
| H10 | Türkçe Destek, TL Fiyat | 23 |
| H11 | İzin (Onay) Yönetimi Dahil | 26 |
| H12 | Kendi AI Anahtarınızı Getirin | 29 |
| H13 | Kurulum Sihirbazı ile Başlayın | 30 |
| H14 | KOBİ'ler İçin Tasarlandı | 24 |
| H15 | Kampanya Analitiği Dahil | 24 |

| # | Açıklama | Kar. |
|---|---|---|
| D1 | TL fiyat, Türkçe destek ve taahhütsüz plan. Ekip gelen kutusu, kampanya ve AI ajan dahil. | 89 |
| D2 | 14 gün ücretsiz deneyin, mevcut aracınızla karşılaştırın. Kurulum sihirbazı ile başlayın. | 89 |
| D3 | Ekip gelen kutusu, izinli kampanyalar, AI ajanlar ve otomasyonlar tek panelde. Deneyin. | 87 |
| D4 | 14 gün ücretsiz deneme, kart bilgisi yok, taahhüt yok. Planlar aylık 1.290 TL + KDV'den. | 88 |

### Ek C. Negatif anahtar kelimeler (her kampanyaya, kampanya düzeyi sıralı eşleme)

indir, indirme, apk, yükle, güncelleme, güncelle, son sürüm, web giriş, whatsapp web, web whatsapp, masaüstü, pc, bilgisayara, iphone, android, ios, tablet, apple watch, gb whatsapp, gbwhatsapp, whatsapp plus, yowhatsapp, fm whatsapp, aero, mod, tema, theme, yedekleme, yedek, geri yükleme, silinen mesajlar, silinen, nasıl silinir, hesap silme, hesap dondurma, engelleme, engelleyen, engelli mi, son görülme, çevrimiçi, online gizleme, gizleme, mavi tik, profil fotoğrafı, durum, çıkartma, sticker, emoji, gif, duvar kağıdı, zil sesi, bildirim sesi, sesli mesaj, görüntülü arama, kamera, dil değiştirme, karanlık mod, yazı tipi, çalışmıyor, çöktü, açılmıyor, hata, sorunu, kod gelmiyor, doğrulama kodu, numara değiştirme, hack, hackleme, hacklemek, casus, takip programı, takip etme, kim baktı, dinleme, izleme programı, şifre kırma, başkasının, sevgilinin, eşinin, mesajları okuma, klonlama, ban kaldırma, ban açma, banlandı, yasaklandı, fake numara, sanal numara, numarasız, yabancı numara, spam, github, python, php, npm, nodejs, baileys, selenium, whatsapp-web.js, venom, qr kod ile bot, chrome eklentisi, iş ilanı, iş ilanları, eleman, kariyer, staj, maaş, ödev, tez, pdf, sunum, logo, png, vektör, grup linki, grup linkleri, gruplar, grup kur, sohbet grubu, arkadaşlık, flört, sözler, mesajları komik, hisse, meta hisse, kurucusu, tarihi

**Kampanyaya özel ek negatifler:**

- Messora: giriş, login, app messora
- WA Business API: toplu, chatbot, crm
- Toplu Mesaj & Kampanya: ücretsiz, bedava, sender, eklenti, numara listesi, numara satın
- Chatbot & AI: numarası, numara
- CRM & Ekip Gelen Kutusu: cihaz, iki telefon, birden fazla telefon
- Marka dışındaki tüm kampanyalar: messora (marka trafiği marka kampanyasında kalsın)

### Ek D. Google Ads Editor CSV içe aktarma notları

- **Dosya:** `google-ads-editor-import.csv`, UTF-8, virgülle ayrılmış, 1.056 satır. Tüm varlık türleri tek dosyada. Editor tek dosyada karma satırları kabul etmezse aynı veri `csv-parcalar/` altında 6 dosyaya bölünmüş halde de var. Sırayla içe aktarın: 1-kampanyalar → 2-konumlar → 3-reklam-gruplari → 4-anahtar-kelimeler → 5-negatifler → 6-rsa-reklamlar.
- **Sütun adları:** Google'ın "CSV file columns" listesine göre seçildi ([Google](https://support.google.com/google-ads/editor/answer/57747?hl=en)). Kullanılan sütunlar: `Campaign`, `Campaign Type` (Search), `Campaign Status`, `Networks` (Google search), `Languages` (tr), `Budget`, `Budget type` (Daily), `Bid Strategy Type` (Maximize clicks), `Final URL suffix` (UTM), `Location` / `Location ID` (Turkey / 2792), `Ad Group`, `Ad Group Status`, `Keyword`, `Criterion Type` (Exact / Phrase / Campaign Negative Phrase), `Status`, `Ad type` (Responsive search ad), `Final URL`, `Path 1`, `Path 2`, `Headline 1–15`, `Headline 1 position`, `Description 1–4`. Editor, başlığı tanımadığı sütunlar için ön izlemede eşleme menüsü sunar ([Google](https://support.google.com/google-ads/editor/answer/30564?hl=en)).
- **Editor'da elle yapılacaklar:**
  - maks. TBM tavanı (Marka 8 TL, diğerleri 45 TL, Toplu Mesaj ve Sektör 30 TL)
  - konum seçeneği "Mevcudiyet"
  - Arama Ağı iş ortaklarının ve Display'in kapalı olduğunun kontrolü
  - hesap düzeyi öğeler (§8)
  - ortak negatif liste (isteğe bağlı, kampanya negatifleri zaten var)
  - reklam zamanlaması ve gözlem kitleleri
- **Durumlar:** Rakip kampanyası `Paused`. Her gruptaki B (markasız) RSA `Paused`. Diğer her şey `Enabled`. Editor'da yayınlamadan önce hesabın ödeme ve doğrulama adımları tamamlanmış olmalı.
- **Doğrulama:** `betikler/build_ads.py` CSV'yi üretirken her başlığı (≤30), açıklamayı (≤90) ve yolu (≤15) ölçer. Sınırı aşan bir metin varsa dosya üretilmez. Son çalıştırmanın çıktısı: `RSAs=32 headlines=480 (max 30/30) descriptions=128 (max 90/90)`. Ayrıca negatiflerin kendi kampanyasındaki bir anahtar kelimeyi engellemediği betikle kontrol edildi (0 çakışma).
