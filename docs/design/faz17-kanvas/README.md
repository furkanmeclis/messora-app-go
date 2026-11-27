# Faz 17 tasarım kanvası (yedek)

Bu klasör, claude.ai'deki "Messora SaaS — Faz 17 tasarım kanvası" Design artifact'ının 27.11.2026 tarihli kaynak yedeğidir (artifact silinmeden önce indirildi).

- `project/canvas.json`: kanvasın düzeni. İki sayfa var: "Faz 17 · Abonelik" ve "Marka kimliği". Ayrıca artboard konumları ve notlar.
- `project/*.dc.html`: her artboard'un kaynak kodu.
  - Abonelik sayfası: Main, Competitors, Pricing, Credits, Onboarding, Billing, PlatformTenants.
  - Marka kimliği sayfası: BrandOverview, BrandLogo, BrandPalette, BrandApplications.

Dosyalar Design Component biçimindedir ve Claude'un kanvas çalışma zamanında (`support.js`) çizilir; tarayıcıda tek başına açılınca düzgün görünmez. Yeniden düzenlemek için içerik yeni bir Design kanvasına yüklenebilir. Renkler, fiyatlar ve metinler ise doğrudan dosyalardan okunabilir.

İlgili belgeler:
- `docs/PHASE-17.md`: kapsam ve kararlar.
- `docs/design/rakip-fiyat-analizi.md`: kaynaklı rakip ve fiyat raporu.
