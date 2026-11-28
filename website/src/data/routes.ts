// Every static page with its Turkish and English path. Keys pair the
// languages for hreflang and the language switcher. Feature, solution,
// legal and blog paths are derived from their data in src/lib/pages.ts.
import type { Lang } from './types';

export const staticRoutes = {
  home: { tr: '/', en: '/en' },
  features: { tr: '/ozellikler', en: '/en/features' },
  solutions: { tr: '/cozumler', en: '/en/solutions' },
  pricing: { tr: '/fiyatlar', en: '/en/pricing' },
  security: { tr: '/guvenlik', en: '/en/security' },
  guide: { tr: '/whatsapp-business-api-nedir', en: '/en/what-is-whatsapp-business-api' },
  resources: { tr: '/kaynaklar', en: '/en/resources' },
  about: { tr: '/hakkimizda', en: '/en/about' },
  contact: { tr: '/iletisim', en: '/en/contact' },
  demo: { tr: '/demo', en: '/en/demo' },
  faq: { tr: '/sss', en: '/en/faq' },
  partners: { tr: '/is-ortakligi', en: '/en/partners' },
  legal: { tr: '/yasal', en: '/en/legal' },
  dataRequest: { tr: '/kvkk-basvuru', en: '/en/data-request' },
  status: { tr: '/durum', en: '/en/status' },
} as const satisfies Record<string, Record<Lang, string>>;

export type StaticKey = keyof typeof staticRoutes;

/** Legal documents: key → localized path. Markdown lives in src/content/legal/<lang>/<key>.md */
export const legalRoutes = {
  kvkk: { tr: '/yasal/kvkk-aydinlatma-metni', en: '/en/legal/privacy-notice' },
  privacy: { tr: '/yasal/gizlilik-politikasi', en: '/en/legal/privacy-policy' },
  cookies: { tr: '/yasal/cerez-politikasi', en: '/en/legal/cookie-policy' },
  terms: { tr: '/yasal/kullanim-kosullari', en: '/en/legal/terms-of-service' },
  'distance-sales': { tr: '/yasal/mesafeli-satis-sozlesmesi', en: '/en/legal/distance-sales-agreement' },
  'pre-information': { tr: '/yasal/on-bilgilendirme-formu', en: '/en/legal/pre-contract-information' },
  refund: { tr: '/yasal/iptal-iade-ve-cayma', en: '/en/legal/cancellation-and-refunds' },
  delivery: { tr: '/yasal/teslimat-ve-hizmetin-ifasi', en: '/en/legal/service-delivery' },
  dpa: { tr: '/yasal/veri-isleme-sozlesmesi', en: '/en/legal/data-processing-agreement' },
  subprocessors: { tr: '/yasal/alt-isleyenler', en: '/en/legal/subprocessors' },
} as const satisfies Record<string, Record<Lang, string>>;

export type LegalKey = keyof typeof legalRoutes;

export const featurePaths = {
  inbox: { tr: '/ozellikler/ekip-gelen-kutusu', en: '/en/features/team-inbox' },
  campaigns: { tr: '/ozellikler/whatsapp-kampanyalari', en: '/en/features/campaigns' },
  ai: { tr: '/ozellikler/yapay-zeka-ajanlari', en: '/en/features/ai-agents' },
  automations: { tr: '/ozellikler/otomasyonlar', en: '/en/features/automations' },
  templates: { tr: '/ozellikler/sablon-olusturucu', en: '/en/features/template-builder' },
  analytics: { tr: '/ozellikler/analitik-ve-raporlar', en: '/en/features/analytics-and-reports' },
  api: { tr: '/ozellikler/api-ve-webhook', en: '/en/features/api-and-webhooks' },
} as const;

export const solutionPaths = {
  ecommerce: { tr: '/cozumler/e-ticaret', en: '/en/solutions/ecommerce' },
  retail: { tr: '/cozumler/perakende', en: '/en/solutions/retail' },
  health: { tr: '/cozumler/saglik-ve-klinikler', en: '/en/solutions/healthcare-and-clinics' },
  education: { tr: '/cozumler/egitim', en: '/en/solutions/education' },
  travel: { tr: '/cozumler/turizm-ve-otel', en: '/en/solutions/travel-and-hotels' },
  realestate: { tr: '/cozumler/emlak', en: '/en/solutions/real-estate' },
  services: { tr: '/cozumler/hizmet-isletmeleri', en: '/en/solutions/service-businesses' },
} as const;

/** Blog posts live at /blog/<file-name> (TR) and /en/blog/<file-name> (EN). */
export const blogBase = { tr: '/blog', en: '/en/blog' } as const;

export const landingPaths = [
  '/lp/whatsapp-business-api',
  '/lp/whatsapp-toplu-mesaj',
  '/lp/whatsapp-chatbot',
  '/lp/whatsapp-crm',
] as const;

export const appUrls = {
  signup: 'https://app.messora.tech/signup',
  login: 'https://app.messora.tech/login',
};
