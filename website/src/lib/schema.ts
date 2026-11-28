import type { Faq, Lang } from '../data/types';
import { SITE, abs } from './site';

export const faqLd = (items: Faq[]) => ({
  '@type': 'FAQPage',
  mainEntity: items.map((f) => ({ '@type': 'Question', name: f.q, acceptedAnswer: { '@type': 'Answer', text: f.a.replace(/\[([^\]]+)\]\([^)]+\)/g, '$1') } })),
});

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
