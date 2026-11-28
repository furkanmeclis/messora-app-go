// Registry of every indexable page (path, title, eyebrow) — used for the
// OpenGraph images.
import { getCollection } from 'astro:content';
import type { Lang } from '../data/types';
import { staticRoutes, legalRoutes, blogBase, type StaticKey, type LegalKey } from '../data/routes';
import { meta } from '../data/meta';
import { features, solutions, fPath, sPath } from './site';
import { landings } from '../data/landings';

export interface PageInfo { path: string; title: string; eyebrow: string; lang: Lang }

export async function allPages(): Promise<PageInfo[]> {
  const out: PageInfo[] = [];
  for (const lang of ['tr', 'en'] as const) {
    for (const k of Object.keys(staticRoutes) as StaticKey[]) {
      const m = meta[k][lang];
      out.push({ path: staticRoutes[k][lang], title: m.og ?? m.title, eyebrow: m.eyebrow ?? 'Messora', lang });
    }
    for (const f of features(lang)) out.push({ path: fPath(lang, f.key), title: f.h1, eyebrow: f.eyebrow, lang });
    for (const s of solutions(lang)) out.push({ path: sPath(lang, s.key), title: s.h1, eyebrow: s.eyebrow, lang });
    const legal = await getCollection('legal', (e) => e.data.lang === lang && e.data.slug !== 'index');
    for (const d of legal) {
      const p = legalRoutes[d.data.slug as LegalKey]?.[lang];
      if (p) out.push({ path: p, title: d.data.title, eyebrow: lang === 'tr' ? 'Yasal' : 'Legal', lang });
    }
    const blog = await getCollection('blog', (e) => e.data.lang === lang);
    for (const b of blog) out.push({ path: `${blogBase[lang]}/${b.data.slug}`, title: b.data.title, eyebrow: b.data.tags[0] ?? (lang === 'tr' ? 'Rehber' : 'Guide'), lang });
  }
  for (const d of Object.values(landings)) out.push({ path: d.path, title: d.h1, eyebrow: d.eyebrow, lang: 'tr' });
  return out;
}
