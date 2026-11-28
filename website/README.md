# Messora website (messora.tech)

Marketing site: Turkish at `/`, English under `/en/`. Static pages built with
Astro, served by a small Node server (`server/`) that also handles the forms.

## Structure

```
src/
  config/company.ts      company / legal entity details (placeholders)
  data/                  routes, page meta, plan fallback, features, solutions, ad landing pages
  lib/plans.ts           build-time plan data (API → fallback), see "Plans and prices"
  content/
    blog/{tr,en}/*.md    articles (file name = URL slug)
    legal/{tr,en}/*.md   legal documents
    pages/{tr,en}/*.md   long-form pages (WhatsApp Business API guide)
  views/                 page templates shared by both languages
  pages/                 thin route files (TR and /en)
  pages/og/[slug].png.ts OpenGraph images (satori + resvg, at build)
  components/            header, footer, icons (icons.ts), product mock-ups, hero visual, forms
  styles/global.css      design tokens (colour, type scale, motion), components, page styles
  scripts/ui.ts          header states, mega menu, mobile sheet, reveals, spotlight, count-ups,
                         FAQ animation, sticky feature story, hero animation, reading progress
  scripts/site.ts        consent + Google tag, signup link tracking, forms
scripts/postbuild.mjs    sitemaps with hreflang, robots.txt, favicons, link check, gzip/brotli
server/server.mjs        static server, www → apex redirect, /api/forms/<type>, /api/config.js
server/forms-spec.mjs    form fields (used by the page markup and the server validation)
server/plans-core.mjs    plan data shape check, formatting and HTML blocks (build + server)
server/plans-live.mjs    live plans cache and page re-rendering
server/mail.mjs          notification and confirmation e-mails
```

## Develop

```
npm ci
npm run dev           # http://localhost:4321 (forms need the server, see below)
npm run build         # dist/ + postbuild (fails on broken internal links)
PORT=4321 RESEND_API_KEY=… FORMS_TO=… npm start
```

## Add a blog post

1. Create `src/content/blog/tr/<slug>.md` (the file name becomes `/blog/<slug>`).
2. Front matter:
   ```yaml
   ---
   title: "Başlık (H1)"
   seoTitle: "≤ 55 karakter <title>"
   description: "140–160 karakter meta açıklama"
   date: 2026-12-01
   updated: 2026-12-01
   translationKey: "unique-key"   # same key on the English version → hreflang pair
   tags: ["WhatsApp Business API"]
   faq:                           # optional; rendered at the end + FAQPage JSON-LD
     - q: "Soru?"
       a: "Yanıt."
   ---
   ```
3. Write the body with `##`/`###` headings (no `#`). Internal links use the
   paths in `src/data/routes.ts`. The OG image, sitemap entry and article
   JSON-LD are generated automatically.
4. Optional English version: `src/content/blog/en/<slug>.md` with the same `translationKey`.

## Plans and prices

Plans, prices and limits come from the app: `GET https://app.messora.tech/api/v1/plans`
(public, CORS open, 5-minute cache, ETag). Nothing about prices is hard-coded in pages.

- **Build:** `src/lib/plans.ts` fetches the API (6 s timeout) and checks the shape
  (`server/plans-core.mjs` → `normalize`). If the API is unreachable or the shape is
  unexpected, it uses `src/data/plans-fallback.json` (same shape as the API). The data
  the build used is written to `dist/plans-snapshot.json`.
- **Pages:** the pricing cards, the comparison table, the VAT note, the home teaser, the
  `SoftwareApplication`/`Product`/`Offer` JSON-LD and the price phrases in copy (meta
  description, "starts at …", yearly discount, FAQ VAT example, ad landing pages) are all
  rendered by `plans-core.mjs`, so prices are in the HTML.
- **Server:** `server/plans-live.mjs` fetches the API in the background (5 minutes,
  stale-while-revalidate, `If-None-Match`; one-minute retry after an error). When the live
  data differs from the snapshot, it re-renders those blocks in the affected pages
  (marked with `data-ps` / `data-plans-ld`) and caches the result with gzip/brotli. Requests
  never wait for the API; if it is down or returns an unexpected shape, the snapshot is served.
- **Rules:** labels and limit formatting are the site's own (TR/EN) in `plans-core.mjs`;
  unknown limit or feature keys are ignored. A `null` limit means unlimited, or "custom" on
  plans with `custom_pricing`. The "Recommended" badge goes to `growth`; support levels per
  plan are in `SUPPORT`. Plan structure changes (a new plan) appear live too, since the
  whole card list and table are re-rendered.
- **Local:** `PLANS_API_URL=file:src/data/plans-fallback.json npm run build` builds from the
  fixture; `PLANS_API_URL=off npm start` disables live fetching.

## Company details (placeholders)

All company fields live in `src/config/company.ts`. A field set to `null`
renders as a highlighted placeholder (e.g. "[Şirket unvanı]") everywhere: legal
texts (tokens like `{{unvan}}`), footer, contact and KVKK pages. Fill in:

`unvan`, `mersis`, `ticaretSicil`, `vergiDairesi`, `vergiNo`, `adres`, `sehir`,
`kep`, `telefon`, `temsilci`. `eposta` (merhaba@messora.tech) and `kvkkEposta`
(kvkk@messora.tech) are set but **need an inbound mailbox** before launch.

The legal texts must be reviewed by a lawyer before they are relied on.

## Environment

| Variable | Purpose |
|---|---|
| `RESEND_API_KEY` | Resend API key for form e-mails |
| `FORMS_TO` | Inbox that receives submissions (never shown on the site) |
| `FORMS_FROM` | Sender, default `Messora <no-reply@messora.tech>` |
| `GTAG_ID` | GA4 measurement ID (`G-…`); empty → no Google tag at all |
| `ADS_ID` | Google Ads ID (`AW-…`) |
| `ADS_LEAD_LABEL` | Conversion label for `generate_lead` (contact/demo/partner/landing forms) |
| `ADS_SIGNUP_LABEL` | Conversion label for `sign_up_click` (links to app.messora.tech/signup) |
| `GTAG_CONSENT_MODE` | `basic` (default: tag loads only after consent) or `advanced` |
| `PLANS_API_URL` | Plans API (build and server). Default `https://app.messora.tech/api/v1/plans`; `file:<path>` at build; `off` on the server |

Consent Mode v2 defaults everything to `denied`; the cookie banner appears only
when a tag ID is configured. Signup links keep `utm_*`/`gclid` parameters and use
the cross-domain linker for app.messora.tech.

## Forms

`POST /api/forms/{contact|demo|partner|lp|kvkk}` (JSON). Honeypot, time trap
(≥ 3 s), 5 submissions per IP per 10 minutes, Origin check. Sends the
submission to `FORMS_TO` (reply-to = requester) and a branded confirmation with
a reference number (`MSR-<type><yymmdd>-XXXXX`) to the requester. Logs contain
only type, reference and status — no personal data.

## Deploy

Dokploy compose app **website** (project Messora, production) builds
`website/docker-compose.yml` from branch `website` of the repo. Domains
`messora.tech` and `www.messora.tech` (Let's Encrypt) point to service
`website`, port 3000; the server answers `www` with a 301 to the apex. Push to
the branch and redeploy from Dokploy (API `compose.deploy`).

Ad landing pages (`/lp/*`) are `noindex` and excluded from the sitemap.
