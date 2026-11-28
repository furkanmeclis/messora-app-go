import type { Faq, Lang } from '../data/types';
import { plans } from '../data/plans';
import { SITE, abs } from './site';

export const faqLd = (items: Faq[]) => ({
  '@type': 'FAQPage',
  mainEntity: items.map((f) => ({ '@type': 'Question', name: f.q, acceptedAnswer: { '@type': 'Answer', text: f.a.replace(/\[([^\]]+)\]\([^)]+\)/g, '$1') } })),
});

export const softwareLd = (lang: Lang) => ({
  '@type': 'SoftwareApplication',
  '@id': `${SITE}/#app`,
  name: 'Messora',
  applicationCategory: 'BusinessApplication',
  applicationSubCategory: 'WhatsApp Business API platform',
  operatingSystem: 'Web',
  url: SITE,
  inLanguage: ['tr', 'en'],
  description: lang === 'tr'
    ? 'Resmi WhatsApp Business Cloud API üzerinde ekip gelen kutusu, izinli kampanyalar, yapay zekâ ajanları, otomasyonlar, analitik ve API.'
    : 'Team inbox, consent-aware campaigns, AI agents, automations, analytics and an API on the official WhatsApp Business Cloud API.',
  publisher: { '@id': `${SITE}/#org` },
  offers: {
    '@type': 'AggregateOffer', priceCurrency: 'TRY', lowPrice: 1290, highPrice: 6490, offerCount: 4,
    url: abs(lang === 'tr' ? '/fiyatlar' : '/en/pricing'),
  },
});

export const productsLd = (lang: Lang) => plans.map((p) => ({
  '@type': 'Product',
  name: `Messora ${p.name[lang]}`,
  description: p.blurb[lang],
  brand: { '@type': 'Brand', name: 'Messora' },
  offers: {
    '@type': 'Offer',
    url: abs(lang === 'tr' ? '/fiyatlar' : '/en/pricing'),
    priceCurrency: 'TRY',
    price: p.monthly,
    availability: 'https://schema.org/InStock',
    priceSpecification: {
      '@type': 'UnitPriceSpecification', price: p.monthly, priceCurrency: 'TRY', unitCode: 'MON', valueAddedTaxIncluded: false,
      ...(p.from ? { minPrice: p.monthly } : {}),
    },
  },
}));

export const articleLd = (o: { lang: Lang; title: string; description: string; path: string; published: Date; modified?: Date; image: string; words: number }) => ({
  '@type': 'Article',
  headline: o.title,
  description: o.description,
  inLanguage: o.lang === 'tr' ? 'tr-TR' : 'en',
  mainEntityOfPage: abs(o.path),
  image: abs(o.image),
  datePublished: o.published.toISOString(),
  dateModified: (o.modified ?? o.published).toISOString(),
  wordCount: o.words,
  author: { '@type': 'Organization', name: 'Messora', url: SITE },
  publisher: { '@id': `${SITE}/#org` },
});
