// After `astro build`: sitemaps with hreflang, robots.txt, favicons and app
// icons, internal link check, and gzip/brotli copies of text assets.
import { readdir, readFile, writeFile, mkdir, stat } from 'node:fs/promises';
import { join, relative } from 'node:path';
import { gzipSync, brotliCompressSync, constants } from 'node:zlib';
import { Resvg } from '@resvg/resvg-js';

const DIST = new URL('../dist/', import.meta.url).pathname;
const SITE = 'https://messora.tech';

async function walk(dir) {
  const out = [];
  for (const e of await readdir(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) out.push(...(await walk(p)));
    else out.push(p);
  }
  return out;
}

const files = await walk(DIST);
const htmlFiles = files.filter((f) => f.endsWith('.html'));

// ── Sitemaps ───────────────────────────────────────────────
const entries = [];
const known = new Set();
for (const f of htmlFiles) {
  const rel = '/' + relative(DIST, f).replace(/index\.html$/, '').replace(/\.html$/, '').replace(/\/$/, '');
  known.add(rel === '/' ? '/' : rel || '/');
  const html = await readFile(f, 'utf8');
  if (/<meta name="robots" content="noindex/.test(html)) continue;
  const canonical = html.match(/<link rel="canonical" href="([^"]+)"/)?.[1];
  if (!canonical) continue;
  const alts = [...html.matchAll(/<link rel="alternate" hreflang="([^"]+)" href="([^"]+)"/g)].map((m) => ({ lang: m[1], href: m[2] }));
  const mod = html.match(/article:modified_time" content="([^"]+)"/)?.[1] ?? html.match(/article:published_time" content="([^"]+)"/)?.[1];
  entries.push({ loc: canonical, alts, lastmod: (mod ? new Date(mod) : new Date()).toISOString().slice(0, 10), blog: /\/blog\//.test(canonical) });
}
entries.sort((a, b) => a.loc.localeCompare(b.loc));
const urlset = (list) => `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">\n${list.map((e) =>
  `  <url>\n    <loc>${e.loc}</loc>\n    <lastmod>${e.lastmod}</lastmod>\n${e.alts.map((a) => `    <xhtml:link rel="alternate" hreflang="${a.lang}" href="${a.href}"/>\n`).join('')}  </url>`).join('\n')}\n</urlset>\n`;
const today = new Date().toISOString().slice(0, 10);
await writeFile(join(DIST, 'sitemap-pages.xml'), urlset(entries.filter((e) => !e.blog)));
await writeFile(join(DIST, 'sitemap-blog.xml'), urlset(entries.filter((e) => e.blog)));
await writeFile(join(DIST, 'sitemap.xml'), `<?xml version="1.0" encoding="UTF-8"?>\n<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n  <sitemap><loc>${SITE}/sitemap-pages.xml</loc><lastmod>${today}</lastmod></sitemap>\n  <sitemap><loc>${SITE}/sitemap-blog.xml</loc><lastmod>${today}</lastmod></sitemap>\n</sitemapindex>\n`);
await writeFile(join(DIST, 'robots.txt'), `User-agent: *\nAllow: /\nDisallow: /lp/\nDisallow: /api/\n\nSitemap: ${SITE}/sitemap.xml\n`);
console.log(`sitemap: ${entries.length} indexable pages`);

// ── Icons ──────────────────────────────────────────────────
const mark = (bg, pad) => `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">${bg ? `<rect width="64" height="64" rx="${bg === 'sq' ? 0 : 14}" fill="#0B1220"/>` : ''}<g transform="translate(${pad} ${pad}) scale(${(64 - 2 * pad) / 64})"><path d="M14 6h36a10 10 0 0 1 10 10v24a10 10 0 0 1-10 10H26l-12 9v-9a10 10 0 0 1-10-10V16A10 10 0 0 1 14 6z" fill="${bg ? '#5B6BF0' : '#3B4CCA'}"/><path d="M19 39V24a5 5 0 0 1 10 0v15M29 24a5 5 0 0 1 10 0v15M39 24a5 5 0 0 1 6-4.9" fill="none" stroke="#FFFFFF" stroke-width="4.5" stroke-linecap="round"/><circle cx="47" cy="17" r="4" fill="#2DD4A7"/></g></svg>`;
const png = (svg, size) => new Resvg(svg, { fitTo: { mode: 'width', value: size } }).render().asPng();
await mkdir(join(DIST, 'icons'), { recursive: true });
await writeFile(join(DIST, 'favicon.svg'), mark(null, 0));
await writeFile(join(DIST, 'icons/icon-192.png'), png(mark('r', 8), 192));
await writeFile(join(DIST, 'icons/icon-512.png'), png(mark('r', 8), 512));
await writeFile(join(DIST, 'icons/maskable-512.png'), png(mark('sq', 14), 512));
await writeFile(join(DIST, 'icons/apple-touch-icon.png'), png(mark('sq', 10), 180));
const ico32 = png(mark(null, 0), 32);
const ico = Buffer.alloc(22);
ico.writeUInt16LE(0, 0); ico.writeUInt16LE(1, 2); ico.writeUInt16LE(1, 4);
ico.writeUInt8(32, 6); ico.writeUInt8(32, 7); ico.writeUInt8(0, 8); ico.writeUInt8(0, 9);
ico.writeUInt16LE(1, 10); ico.writeUInt16LE(32, 12); ico.writeUInt32LE(ico32.length, 14); ico.writeUInt32LE(22, 18);
await writeFile(join(DIST, 'favicon.ico'), Buffer.concat([ico, ico32]));
await writeFile(join(DIST, 'site.webmanifest'), JSON.stringify({
  name: 'Messora', short_name: 'Messora', start_url: '/', display: 'browser', background_color: '#0B1220', theme_color: '#0B1220',
  icons: [
    { src: '/icons/icon-192.png', sizes: '192x192', type: 'image/png' },
    { src: '/icons/icon-512.png', sizes: '512x512', type: 'image/png' },
    { src: '/icons/maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
  ],
}, null, 2));

// ── Internal link check ────────────────────────────────────
const assets = new Set((await walk(DIST)).map((f) => '/' + relative(DIST, f)));
const broken = new Map();
for (const f of htmlFiles) {
  const html = await readFile(f, 'utf8');
  for (const m of html.matchAll(/(?:href|src)="(\/[^"#?]*)/g)) {
    const p = m[1].replace(/\/$/, '') || '/';
    if (p.startsWith('/api/')) continue;
    if (known.has(p) || assets.has(p) || assets.has(p + '/index.html')) continue;
    const page = '/' + relative(DIST, f);
    if (!broken.has(p)) broken.set(p, page);
  }
}
if (broken.size) {
  console.error('Broken internal links:');
  for (const [p, page] of broken) console.error(`  ${p}  (first seen in ${page})`);
  process.exitCode = 1;
}

// ── Precompress ────────────────────────────────────────────
let n = 0;
for (const f of await walk(DIST)) {
  if (!/\.(html|css|js|svg|xml|txt|json|webmanifest|ico)$/.test(f)) continue;
  const buf = await readFile(f);
  if (buf.length < 1024) continue;
  await writeFile(f + '.gz', gzipSync(buf, { level: 9 }));
  await writeFile(f + '.br', brotliCompressSync(buf, { params: { [constants.BROTLI_PARAM_QUALITY]: 11 } }));
  n++;
}
console.log(`precompressed ${n} files`);
