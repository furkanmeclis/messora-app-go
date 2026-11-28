// Titles (without the " | Messora" suffix) and meta descriptions of the
// static pages. Also used for the generated OpenGraph images.
import type { Lang } from './types';
import type { StaticKey } from './routes';

export interface PageMeta { title: string; description: string; og?: string; eyebrow?: string }

export const meta: Record<StaticKey, Record<Lang, PageMeta>> = {
  home: {
    tr: { title: 'Messora — Resmi WhatsApp Business API platformu', og: 'WhatsApp’ta satış, destek ve kampanya tek panelde', eyebrow: 'Resmi WhatsApp Business API',
      description: 'Ekip gelen kutusu, izinli WhatsApp kampanyaları, yapay zekâ ajanları ve otomasyonlar. Meta ücretini göndermeden görün. 14 gün ücretsiz deneyin.' },
    en: { title: 'Messora — Official WhatsApp Business API platform', og: 'Sales, support and campaigns on WhatsApp, in one place', eyebrow: 'Official WhatsApp Business API',
      description: 'Team inbox, consent-aware WhatsApp campaigns, AI agents and automations. See the Meta fee before you send. Try it free for 14 days.' },
  },
  features: {
    tr: { title: 'Özellikler: WhatsApp gelen kutusu, kampanya, AI ve otomasyon', og: 'Messora’nın tüm özellikleri', eyebrow: 'Özellikler',
      description: 'Messora’nın özellikleri: ekip gelen kutusu, WhatsApp toplu mesaj kampanyaları, AI ajanları, otomasyon tuvali, şablon oluşturucu, analitik, API ve webhook.' },
    en: { title: 'Features: WhatsApp inbox, campaigns, AI and automation', og: 'Everything Messora does', eyebrow: 'Features',
      description: 'Messora features: team inbox, WhatsApp broadcast campaigns, AI agents, an automation canvas, template builder, analytics, a public API and webhooks.' },
  },
  solutions: {
    tr: { title: 'Sektörlere göre WhatsApp çözümleri', og: 'Sektörünüze göre WhatsApp kullanımı', eyebrow: 'Çözümler',
      description: 'E-ticaret, perakende, sağlık, eğitim, turizm, emlak ve hizmet işletmeleri için gerçekçi WhatsApp Business API akışları, şablon örnekleri ve izin notları.' },
    en: { title: 'WhatsApp solutions by industry', og: 'WhatsApp, the way your industry works', eyebrow: 'Solutions',
      description: 'Realistic WhatsApp Business API flows, template examples and consent notes for e-commerce, retail, healthcare, education, travel, real estate and services.' },
  },
  pricing: {
    tr: { title: 'Fiyatlar: WhatsApp Business API paketleri', og: 'Şeffaf fiyat, Meta ücreti ayrı ve görünür', eyebrow: 'Fiyatlar',
      description: 'Messora paketleri 1.290 TL + KDV/ay’dan başlar. Yıllıkta 12 ay 10 ay fiyatına. Meta mesaj ücretleri ayrıdır ve göndermeden önce gösterilir.' },
    en: { title: 'Pricing: WhatsApp Business API plans', og: 'Clear prices, Meta fees shown separately', eyebrow: 'Pricing',
      description: 'Messora plans start at 1,290 TL + VAT per month; yearly is 12 months for the price of 10. Meta message fees are separate and shown before you send.' },
  },
  security: {
    tr: { title: 'Güvenlik ve veri: KVKK, AB’de barındırma, şifreleme', og: 'Verileriniz AB’de, ayrık ve şifreli', eyebrow: 'Güvenlik',
      description: 'Messora’da her firmanın verisi satır bazlı güvenlikle ayrılır, sırlar şifrelenir, yedekler her gece şifreli alınır. Barındırma: AWS Frankfurt (AB).' },
    en: { title: 'Security and data: KVKK, GDPR, EU hosting, encryption', og: 'Your data in the EU, isolated and encrypted', eyebrow: 'Security',
      description: 'In Messora each company’s data is isolated with row-level security, secrets are encrypted and backups are taken nightly, encrypted. Hosted on AWS Frankfurt.' },
  },
  guide: {
    tr: { title: 'WhatsApp Business API nedir? Kapsamlı rehber', og: 'WhatsApp Business API nedir?', eyebrow: 'Rehber',
      description: 'WhatsApp Business API nedir, uygulamadan farkı ne, kimler kullanmalı, nasıl başvurulur ve ne kadar tutar? Türkiye’deki işletmeler için sade bir rehber.' },
    en: { title: 'What is the WhatsApp Business API? A complete guide', og: 'What is the WhatsApp Business API?', eyebrow: 'Guide',
      description: 'What the WhatsApp Business API is, how it differs from the app, who should use it, how to apply and what it costs — a plain guide for businesses.' },
  },
  resources: {
    tr: { title: 'Kaynaklar: WhatsApp Business API rehberleri', og: 'WhatsApp Business API rehberleri', eyebrow: 'Kaynaklar',
      description: 'WhatsApp Business API başvurusu, toplu mesajın yasal çerçevesi, Meta ücretleri, şablon onayı ve yapay zekâ ile müşteri hizmetleri üzerine rehberler.' },
    en: { title: 'Resources: WhatsApp Business API guides', og: 'WhatsApp Business API guides', eyebrow: 'Resources',
      description: 'Guides on applying for the WhatsApp Business API, Meta pricing in 2026 and choosing between the WhatsApp Business app and the API.' },
  },
  about: {
    tr: { title: 'Hakkımızda', og: 'Messora’yı neden yapıyoruz', eyebrow: 'Hakkımızda',
      description: 'Messora, Türkiye’deki işletmelerin WhatsApp’taki satış, destek ve kampanyalarını tek panelde toplamak için geliştirilen resmi WhatsApp Business API platformudur.' },
    en: { title: 'About us', og: 'Why we build Messora', eyebrow: 'About',
      description: 'Messora is an official WhatsApp Business API platform built to bring sales, support and campaigns on WhatsApp into one place for businesses in Türkiye.' },
  },
  contact: {
    tr: { title: 'İletişim', og: 'Bize yazın', eyebrow: 'İletişim',
      description: 'Messora ekibine ulaşın: satış, destek, fatura ve iş ortaklığı soruları için iletişim formu ve şirket bilgileri. İş günlerinde yanıt veriyoruz.' },
    en: { title: 'Contact', og: 'Get in touch', eyebrow: 'Contact',
      description: 'Contact the Messora team: a form for sales, support, billing and partnership questions, plus our company details. We reply on business days.' },
  },
  demo: {
    tr: { title: 'Demo isteyin', og: 'Messora’yı kendi senaryonuzla görün', eyebrow: 'Demo',
      description: 'Messora’yı kendi iş akışınız üzerinden görün: gelen kutusu, kampanya maliyeti, AI ajanı ve otomasyonlar. 30 dakikalık görüşme için formu doldurun.' },
    en: { title: 'Request a demo', og: 'See Messora with your own scenario', eyebrow: 'Demo',
      description: 'See Messora on your own workflow: the inbox, campaign cost preview, AI agents and automations. Fill in the form for a 30-minute call.' },
  },
  faq: {
    tr: { title: 'Sık sorulan sorular', og: 'Sık sorulan sorular', eyebrow: 'SSS',
      description: 'WhatsApp Business API, Meta ücretleri, deneme süresi, ödeme, veri güvenliği ve kurulum hakkında Messora’ya en sık sorulan sorular ve kısa yanıtları.' },
    en: { title: 'Frequently asked questions', og: 'Frequently asked questions', eyebrow: 'FAQ',
      description: 'Short answers to the questions we hear most about the WhatsApp Business API, Meta fees, the trial, payment, data security and getting set up.' },
  },
  partners: {
    tr: { title: 'İş ortaklığı programı', og: 'Messora iş ortaklığı', eyebrow: 'İş ortaklığı',
      description: 'Ajans, yazılım evi ya da danışmansanız müşterilerinize Messora ile resmi WhatsApp Business API kurun. İş ortaklığı başvurusu için formu doldurun.' },
    en: { title: 'Partner programme', og: 'Partner with Messora', eyebrow: 'Partners',
      description: 'Agencies, software houses and consultants: set up the official WhatsApp Business API for your clients with Messora. Apply with the partner form.' },
  },
  legal: {
    tr: { title: 'Yasal belgeler', og: 'Yasal belgeler', eyebrow: 'Yasal',
      description: 'Messora’nın KVKK aydınlatma metni, gizlilik ve çerez politikaları, hizmet sözleşmesi, mesafeli satış, iade koşulları, veri işleme sözleşmesi ve alt işleyenler.' },
    en: { title: 'Legal documents', og: 'Legal documents', eyebrow: 'Legal',
      description: 'Messora’s privacy notice, privacy and cookie policies, terms of service, distance sales, refund terms, data processing agreement and sub-processors.' },
  },
  dataRequest: {
    tr: { title: 'KVKK başvurusu ve veri silme talebi', og: 'KVKK başvurusu ve veri silme', eyebrow: 'Veri talepleri',
      description: 'Kişisel verilerinizle ilgili KVKK md.11 kapsamındaki taleplerinizi ve WhatsApp/Meta kullanıcı verisi silme isteğinizi bu formla iletin. 30 gün içinde yanıtlanır.' },
    en: { title: 'Data requests and data deletion', og: 'Data requests and deletion', eyebrow: 'Data requests',
      description: 'Use this form to exercise your rights over your personal data and to ask for deletion of your data, including WhatsApp/Meta user data. Answered within 30 days.' },
  },
  status: {
    tr: { title: 'Sistem durumu', og: 'Sistem durumu', eyebrow: 'Durum',
      description: 'Messora hizmetlerinin durumu. Planlı bakım ve kesintileri e-posta ile duyuruyoruz; herkese açık durum sayfası hazırlanıyor.' },
    en: { title: 'System status', og: 'System status', eyebrow: 'Status',
      description: 'Status of the Messora services. We announce planned maintenance and incidents by e-mail; a public status page is being prepared.' },
  },
};
