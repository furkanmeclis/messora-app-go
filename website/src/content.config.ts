// Content collections. A small custom loader reads Markdown with YAML front
// matter, replaces the company tokens ({{unvan}} …, see src/config/company.ts)
// with the configured value or a visible placeholder, then renders it.
import { defineCollection } from 'astro:content';
import { z } from 'astro/zod';
import { readdir, readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { parse as parseYaml } from 'yaml';
import type { Loader } from 'astro/loaders';
import { company, companyHtml, type CompanyKey } from './config/company';

function markdownLoader(dir: string): Loader {
  return {
    name: 'messora-markdown',
    async load({ store, parseData, renderMarkdown, generateDigest, logger }) {
      store.clear();
      for (const lang of ['tr', 'en'] as const) {
        const base = join(process.cwd(), 'src/content', dir, lang);
        let files: string[] = [];
        try { files = (await readdir(base)).filter((f) => f.endsWith('.md')); } catch { continue; }
        for (const file of files) {
          const path = join(base, file);
          const raw = await readFile(path, 'utf8');
          const m = raw.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n?([\s\S]*)$/);
          if (!m) { logger.warn(`${dir}/${lang}/${file}: no front matter`); continue; }
          const front = parseYaml(m[1]) ?? {};
          const body = m[2].replace(/\{\{\s*([a-zA-Z]+)\s*\}\}/g, (all, key: string) =>
            key in company ? companyHtml(key as CompanyKey, lang) : all);
          const slug = file.replace(/\.md$/, '');
          const id = `${lang}/${slug}`;
          const words = body.replace(/<[^>]+>/g, ' ').split(/\s+/).filter(Boolean).length;
          const data = await parseData({ id, data: { ...front, lang, slug, words } });
          const rendered = await renderMarkdown(body, { fileURL: pathToFileURL(path) });
          store.set({ id, data, body, rendered, digest: generateDigest(raw), filePath: `src/content/${dir}/${lang}/${file}` });
        }
      }
    },
  };
}

const faq = z.array(z.object({ q: z.string(), a: z.string() })).optional();
const date = z.union([z.string(), z.date()]).transform((v) => new Date(v));

const legal = defineCollection({
  loader: markdownLoader('legal'),
  schema: z.object({
    title: z.string(), description: z.string(), updated: date, version: z.string().optional(),
    lang: z.enum(['tr', 'en']), slug: z.string(), words: z.number(),
  }),
});

const blog = defineCollection({
  loader: markdownLoader('blog'),
  schema: z.object({
    title: z.string(), seoTitle: z.string().optional(), description: z.string(),
    date, updated: date.optional(), translationKey: z.string().optional(),
    tags: z.array(z.string()).default([]), faq,
    lang: z.enum(['tr', 'en']), slug: z.string(), words: z.number(),
  }),
});

const pages = defineCollection({
  loader: markdownLoader('pages'),
  schema: z.object({
    title: z.string(), seoTitle: z.string().optional(), description: z.string(),
    updated: date.optional(), faq,
    lang: z.enum(['tr', 'en']), slug: z.string(), words: z.number(),
  }),
});

export const collections = { legal, blog, pages };
