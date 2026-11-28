# Messora website (messora.tech)

Marketing site: Turkish at `/`, English under `/en/`. Static pages built with
Astro, served by a small Node server (`server/`) that also handles the forms.

## Structure

```
src/
  config/company.ts      company / legal entity details (placeholders)
  data/                  routes, page meta, plans, features, solutions, ad landing pages
  content/
    blog/{tr,en}/*.md    articles (file name = URL slug)
    legal/{tr,en}/*.md   legal documents
    pages/{tr,en}/*.md   long-form pages (WhatsApp Business API guide)
  views/                 page templates shared by both languages
  pages/                 thin route files (TR and /en)
  pages/og/[slug].png.ts OpenGraph images (satori + resvg, at build)
  components/            header, footer, icons (icons.ts), product mock-ups, forms
  scripts/site.ts        menus, consent + Google tag, signup link tracking, forms
scripts/postbuild.mjs    sitemaps with hreflang, robots.txt, favicons, link check, gzip/brotli
server/server.mjs        static server, www → apex redirect, /api/forms/<type>, /api/config.js
server/forms-spec.mjs    form fields (used by the page markup and the server validation)
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
