// OpenGraph / Twitter images, 1200×630, generated at build with satori + resvg.
import type { APIRoute } from 'astro';
import satori from 'satori';
import { Resvg } from '@resvg/resvg-js';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { allPages, type PageInfo } from '../../lib/pages';
import { ogFor } from '../../lib/site';

const fs = (p: string) => readFileSync(join(process.cwd(), 'node_modules/@fontsource', p));
const fonts = [
  { name: 'Jakarta', weight: 800 as const, style: 'normal' as const, data: fs('plus-jakarta-sans/files/plus-jakarta-sans-latin-800-normal.woff') },
  { name: 'JakartaExt', weight: 800 as const, style: 'normal' as const, data: fs('plus-jakarta-sans/files/plus-jakarta-sans-latin-ext-800-normal.woff') },
  { name: 'Inter', weight: 600 as const, style: 'normal' as const, data: fs('inter/files/inter-latin-600-normal.woff') },
  { name: 'InterExt', weight: 600 as const, style: 'normal' as const, data: fs('inter/files/inter-latin-ext-600-normal.woff') },
];

export async function getStaticPaths() {
  const pages = await allPages();
  return pages.map((p) => ({ params: { slug: ogFor(p.path).replace(/^\/og\//, '').replace(/\.png$/, '') }, props: { page: p } }));
}

const h = (type: string, style: Record<string, unknown>, children?: unknown) => ({ type, props: { style, children } });
const LOGO = 'data:image/svg+xml;base64,' + Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><path d="M14 6h36a10 10 0 0 1 10 10v24a10 10 0 0 1-10 10H26l-12 9v-9a10 10 0 0 1-10-10V16A10 10 0 0 1 14 6z" fill="#5B6BF0"/><path d="M19 39V24a5 5 0 0 1 10 0v15M29 24a5 5 0 0 1 10 0v15M39 24a5 5 0 0 1 6-4.9" fill="none" stroke="#FFFFFF" stroke-width="4.5" stroke-linecap="round"/><circle cx="47" cy="17" r="4" fill="#2DD4A7"/></svg>').toString('base64');
const BUBBLE = 'data:image/svg+xml;base64,' + Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><path d="M14 6h36a10 10 0 0 1 10 10v24a10 10 0 0 1-10 10H26l-12 9v-9a10 10 0 0 1-10-10V16A10 10 0 0 1 14 6z" fill="none" stroke="#3B4CCA" stroke-width="1.2" opacity=".55"/><circle cx="47" cy="17" r="4" fill="#2DD4A7" opacity=".9"/></svg>').toString('base64');

export const GET: APIRoute = async ({ props }) => {
  const page = props.page as PageInfo;
  const size = page.title.length > 70 ? 52 : page.title.length > 45 ? 60 : 68;
  const tree = h('div', { width: 1200, height: 630, display: 'flex', flexDirection: 'column', justifyContent: 'space-between', padding: '64px 72px', background: '#0B1220', color: '#F4F6FB', fontFamily: 'Inter, InterExt', position: 'relative' }, [
    { type: 'img', props: { src: BUBBLE, width: 460, height: 460, style: { position: 'absolute', right: -170, top: -40 } } },
    h('div', { display: 'flex', width: 900, height: 900, position: 'absolute', right: -380, top: -420, borderRadius: 450, background: 'radial-gradient(closest-side, rgba(59,76,202,0.45), rgba(11,18,32,0))' }),
    h('div', { display: 'flex', alignItems: 'center', gap: 16 }, [
      { type: 'img', props: { src: LOGO, width: 56, height: 56 } },
      h('div', { fontFamily: 'Jakarta, JakartaExt', fontSize: 40, letterSpacing: -1.5 }, 'messora'),
    ]),
    h('div', { display: 'flex', flexDirection: 'column', gap: 22, maxWidth: 900 }, [
      h('div', { display: 'flex', alignItems: 'center', gap: 14, fontSize: 24, color: '#AEB7F5', textTransform: 'uppercase', letterSpacing: 3 }, [
        h('div', { width: 34, height: 4, borderRadius: 2, background: '#2DD4A7' }), page.eyebrow,
      ]),
      h('div', { fontFamily: 'Jakarta, JakartaExt', fontSize: size, lineHeight: 1.08, letterSpacing: -2 }, page.title),
    ]),
    h('div', { display: 'flex', justifyContent: 'space-between', fontSize: 24, color: '#A3AECA' }, [
      h('div', {}, page.lang === 'tr' ? 'Konuşmak, satmanın en kısa yolu.' : 'Talking is the shortest way to sell.'),
      h('div', { color: '#F4F6FB' }, 'messora.tech'),
    ]),
  ]);
  const svg = await satori(tree as any, { width: 1200, height: 630, fonts });
  const png = new Resvg(svg, { fitTo: { mode: 'width', value: 1200 } }).render().asPng();
  return new Response(png, { headers: { 'Content-Type': 'image/png' } });
};
