// Plans and prices: shape check, formatting and HTML for every place that
// shows plan data. Shared by the Astro build (pages are rendered from a
// build-time snapshot) and the Node server (which re-renders the same blocks
// from the live API, see server.mjs). No dependencies.
//
// Source: GET https://app.messora.tech/api/v1/plans
//   { updated_at, plans: [{ key, name:{tr,en}, description:{tr,en},
//     prices:{ monthly_try, yearly_try, vat_rate, currency, custom_pricing },
//     limits:{ phone_numbers, users, ai_agents, automation_flows, ai_credits_per_month,
//              api_calls_per_month, webhook_endpoints, storage_bytes, campaign_messages_per_second },
//     features:{ custom_smtp, byo_ai_keys, custom_domain, public_api, webhooks, ai_agents } }] }
// A null limit means unlimited (or custom on plans with custom pricing).
// Unknown limit and feature keys are ignored.

export const PLANS_API_URL = 'https://app.messora.tech/api/v1/plans';
const SITE = 'https://messora.tech';
const SIGNUP = 'https://app.messora.tech/signup';
const POPULAR = 'growth';
const LIMIT_KEYS = ['phone_numbers', 'users', 'ai_agents', 'automation_flows', 'ai_credits_per_month', 'api_calls_per_month', 'webhook_endpoints', 'storage_bytes', 'campaign_messages_per_second'];
const LIMIT_ALIASES = { active_flows: 'automation_flows', monthly_ai_credits: 'ai_credits_per_month', api_calls_month: 'api_calls_per_month', campaign_msgs_per_sec: 'campaign_messages_per_second' };
const FEATURE_KEYS = ['custom_smtp', 'byo_ai_keys', 'custom_domain', 'public_api', 'webhooks', 'ai_agents'];

const str = (v) => (typeof v === 'string' && v.trim() ? v.trim() : null);
const num = (v) => (typeof v === 'number' && Number.isFinite(v) && v >= 0 ? v : (typeof v === 'string' && /^\d+(\.\d+)?$/.test(v) ? Number(v) : undefined));

/** Validates an API response; returns normalised plans or null when the shape is unexpected. */
export function normalize(json) {
  const list = Array.isArray(json) ? json : Array.isArray(json?.plans) ? json.plans : Array.isArray(json?.data) ? json.data : null;
  if (!list || !list.length || list.length > 12) return null;
  const out = [];
  for (const p of list) {
    const key = str(p?.key);
    const tr = str(p?.name?.tr), en = str(p?.name?.en) ?? tr;
    const pr = p?.prices ?? {};
    const custom = pr.custom_pricing === true;
    const monthly = num(pr.monthly_try);
    if (!key || !/^[a-z0-9_-]{1,40}$/.test(key) || !tr) return null;
    if (monthly === undefined && !custom) return null;
    if ((pr.currency ?? 'TRY') !== 'TRY') return null;
    let vat = num(pr.vat_rate);
    if (vat === undefined) vat = 0.2;
    if (vat > 1) vat /= 100;
    const limits = {};
    for (const [k, v] of Object.entries(p?.limits ?? {})) {
      const kk = LIMIT_KEYS.includes(k) ? k : LIMIT_ALIASES[k];
      if (!kk) continue;
      if (v === null) limits[kk] = null;
      else if (num(v) !== undefined) limits[kk] = num(v);
    }
    const features = {};
    for (const k of FEATURE_KEYS) if (typeof p?.features?.[k] === 'boolean') features[k] = p.features[k];
    out.push({
      key, custom, vat,
      name: { tr, en },
      desc: { tr: str(p?.description?.tr) ?? '', en: str(p?.description?.en) ?? str(p?.description?.tr) ?? '' },
      monthly: monthly ?? null,
      yearly: num(pr.yearly_try) ?? null,
      limits, features,
    });
  }
  if (new Set(out.map((p) => p.key)).size !== out.length) return null;
  return out;
}

/** Back to the API shape (used for the build snapshot file). */
export function toApiShape(plans, updatedAt = new Date().toISOString()) {
  return {
    updated_at: updatedAt,
    plans: plans.map((p) => ({
      key: p.key, name: p.name, description: p.desc,
      prices: { monthly_try: p.monthly, yearly_try: p.yearly, vat_rate: p.vat, currency: 'TRY', custom_pricing: p.custom },
      limits: p.limits, features: p.features,
    })),
  };
}

export function hashPlans(plans) {
  const s = JSON.stringify(plans);
  let h = 5381;
  for (let i = 0; i < s.length; i++) h = ((h << 5) + h + s.charCodeAt(i)) >>> 0;
  return h.toString(36);
}

// ── Formatting ─────────────────────────────────────────────
const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
export const tl = (n, lang) => new Intl.NumberFormat(lang === 'tr' ? 'tr-TR' : 'en-GB').format(Math.round(n));
const tl2 = (n, lang) => new Intl.NumberFormat(lang === 'tr' ? 'tr-TR' : 'en-GB', { maximumFractionDigits: 0 }).format(n);
const L = (lang) => (a, b) => (lang === 'tr' ? a : b);
const priced = (plans) => plans.filter((p) => !p.custom && p.monthly);
const yearlyMonthly = (p) => (p.yearly ? Math.round(p.yearly / 12) : null);

function big(n, lang) {
  const T = L(lang);
  if (n >= 1e6) return `${tl2(n / 1e6, lang)} ${T('milyon', 'million')}`;
  if (n >= 1e3) return lang === 'tr' ? `${tl2(n / 1e3, lang)} bin` : `${tl2(n / 1e3, lang)}k`;
  return tl(n, lang);
}
function fmtLimit(p, k, lang) {
  const T = L(lang);
  const v = p.limits[k];
  if (v === undefined) return '—';
  if (k === 'api_calls_per_month' && (p.features.public_api === false || v === 0)) return p.features.webhooks === false ? '—' : T('Yalnızca webhook', 'Webhooks only');
  if (v === null) {
    if (k === 'automation_flows') return T('Sınırsız', 'Unlimited');
    if (k === 'ai_credits_per_month' && p.custom) return T('Özel havuz', 'Custom pool');
    return p.custom ? T('Özel', 'Custom') : T('Sınırsız', 'Unlimited');
  }
  if (k === 'api_calls_per_month') return big(v, lang);
  if (k === 'storage_bytes') return `${tl2(v / 1e9, lang)} GB`;
  return tl(v, lang);
}

const LIMIT_ROWS = [
  ['phone_numbers', 'WhatsApp numarası', 'WhatsApp numbers'],
  ['users', 'Kullanıcı', 'Users'],
  ['ai_agents', 'AI ajanı', 'AI agents'],
  ['automation_flows', 'Aktif otomasyon akışı', 'Active automation flows'],
  ['ai_credits_per_month', 'Aylık AI kredisi', 'AI credits per month', 'Standart modelde 1 kredi ≈ 1 AI yanıtı.', 'On the standard model 1 credit ≈ 1 AI reply.'],
  ['api_calls_per_month', 'API çağrısı / ay', 'API calls per month'],
  ['webhook_endpoints', 'Webhook adresi', 'Webhook endpoints'],
  ['storage_bytes', 'Dosya depolama', 'File storage'],
];
const ALL_ROWS = [
  ['Ekip gelen kutusu: atama, sahiplik, ekibe özel notlar', 'Team inbox: assignment, ownership, internal notes'],
  ['Kampanyalar, izin ve ret yönetimi', 'Campaigns, consent and opt-out'],
  ['Şablon oluşturucu (carousel, süreli teklif, OTP, Flows)', 'Template builder (carousel, limited-time offer, OTP, Flows)'],
  ['11 hazır otomasyon şablonu', '11 ready-made automation presets'],
  ['Gönderim öncesi maliyet ve tahmin', 'Cost and forecast before sending'],
  ['Dışa aktarım (CSV/Excel) ve PDF raporlar', 'Exports (CSV/Excel) and PDF reports'],
  ['İki adımlı doğrulama, roller, denetim kaydı', '2FA, roles, audit log'],
];
const FEATURE_ROWS = [
  ['custom_domain', 'Kendi alt alan adı ve alan adı (firma.messora.tech / panel.firmaniz.com)', 'Own subdomain and domain (company.messora.tech / panel.yourcompany.com)', 'Diğer paketler ve deneme ortak panelde: app.messora.tech', 'Other plans and the trial use the shared panel: app.messora.tech'],
  ['custom_smtp', 'Kendi SMTP sunucunuz', 'Your own SMTP server'],
  ['byo_ai_keys', 'Kendi AI anahtarlarınız', 'Your own AI keys'],
];
const SUPPORT = {
  starter: ['E-posta', 'E-mail'], growth: ['E-posta', 'E-mail'], pro: ['Öncelikli e-posta', 'Priority e-mail'], enterprise: ['E-posta, atanmış yönetici', 'E-mail, account manager'],
};

/** Short, translated plan highlights for the plan cards. */
export function keyPoints(p, lang) {
  const T = L(lang);
  const out = [];
  const l = p.limits;
  if (p.custom) {
    out.push(T('Özel numara, kullanıcı ve ajan sayısı', 'Custom numbers, users and agents'));
    if (p.features.custom_domain) out.push(T('Kendi adresiniz: firma.messora.tech veya kendi alan adınız', 'Your own address: company.messora.tech or your own domain'));
    if (p.features.custom_smtp) out.push(T('Kendi SMTP sunucunuz', 'Your own SMTP server'));
    if (p.features.byo_ai_keys) out.push(T('Kendi AI anahtarlarınız', 'Your own AI keys'));
    out.push(T('Sözleşmeli hizmet, atanmış yönetici', 'Contracted service, account manager'));
    return out;
  }
  if (l.phone_numbers !== undefined && l.users !== undefined) {
    const n = l.phone_numbers === null ? T('Sınırsız numara', 'Unlimited numbers') : `${tl(l.phone_numbers, lang)} ${T('numara', l.phone_numbers === 1 ? 'number' : 'numbers')}`;
    const u = l.users === null ? T('sınırsız kullanıcı', 'unlimited users') : `${tl(l.users, lang)} ${T('kullanıcı', l.users === 1 ? 'user' : 'users')}`;
    out.push(`${n}, ${u}`);
  }
  if (l.ai_agents !== undefined && p.features.ai_agents !== false) {
    const a = l.ai_agents === null ? T('Sınırsız AI ajanı', 'Unlimited AI agents') : `${tl(l.ai_agents, lang)} ${T('AI ajanı', l.ai_agents === 1 ? 'AI agent' : 'AI agents')}`;
    out.push(l.ai_credits_per_month ? `${a}, ${tl(l.ai_credits_per_month, lang)} ${T('AI kredisi/ay', 'AI credits/mo')}` : a);
  }
  if (l.automation_flows !== undefined) out.push(l.automation_flows === null ? T('Sınırsız otomasyon', 'Unlimited automations') : `${tl(l.automation_flows, lang)} ${T('aktif otomasyon', 'active automations')}`);
  if (p.features.public_api === false || l.api_calls_per_month === 0) { if (p.features.webhooks !== false) out.push(T('Webhook’lar', 'Webhooks')); }
  else if (l.api_calls_per_month !== undefined) out.push(l.api_calls_per_month === null ? T('API: sınırsız çağrı', 'API: unlimited calls') : `API: ${big(l.api_calls_per_month, lang)} ${T('çağrı/ay', 'calls/mo')}`);
  return out;
}

// ── Phrases used inside copy (the server swaps old → new) ──
export function phrases(plans, lang) {
  const T = L(lang);
  const pp = priced(plans);
  const from = pp.length ? Math.min(...pp.map((p) => p.monthly)) : null;
  const ratios = pp.filter((p) => p.yearly).map((p) => p.yearly / p.monthly);
  const tenFor12 = ratios.length && ratios.every((r) => Math.abs(r - 10) < 0.01);
  const save = ratios.length ? Math.round((1 - Math.min(...ratios) / 12) * 100) : 0;
  const vatPct = Math.round((pp[0]?.vat ?? 0.2) * 100);
  const ex = pp.find((p) => p.key === POPULAR) ?? pp[0];
  return {
    from: from ? T(`aylık ${tl(from, lang)} TL + KDV’den başlar`, `start at ${tl(from, lang)} TL + VAT per month`) : T('teklifle belirlenir', 'are quoted on request'),
    yearly: !ratios.length ? '' : tenFor12 ? T('Yıllık ödemede 12 ay, 10 ay fiyatına.', 'Pay yearly: 12 months for the price of 10.') : T(`Yıllık ödemede %${save}’e varan indirim.`, `Pay yearly and save up to ${save}%.`),
    yearlyChip: !ratios.length ? '' : tenFor12 ? T('2 ay bedava', '2 months free') : T(`%${save}’e varan indirim`, `save up to ${save}%`),
    vat: T(`%${vatPct} KDV hariçtir`, `exclude ${vatPct}% VAT`),
    vatExample: ex ? T(`Örneğin ${ex.name.tr} paketinin KDV dahil aylık fiyatı: ${tl(ex.monthly * (1 + ex.vat), lang)} TL.`, `For example, ${ex.name.en} is ${tl(ex.monthly * (1 + ex.vat), lang)} TL per month including VAT.`) : '',
  };
}

/** [old, new] text pairs for both languages; empty when nothing changed. */
export function phrasePairs(oldPlans, newPlans) {
  const out = [];
  for (const lang of ['tr', 'en']) {
    const a = phrases(oldPlans, lang), b = phrases(newPlans, lang);
    for (const k of Object.keys(a)) if (a[k] && a[k] !== b[k]) out.push([a[k], b[k]]);
  }
  return out;
}

// ── HTML blocks (wrapped in markers the server can find) ───
const ICON = (d, cls = 'ic') => `<svg class="${cls}" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">${d}</svg>`;
const CHECK = '<path d="m5 12.5 4.5 4.5L19 7.5"/>';
const ARROW = '<path d="M5 12h14M13.5 6.5 19 12l-5.5 5.5"/>';
export const slot = (name, html) => `<i hidden data-ps="${name}"></i>${html}<i hidden data-pe="${name}"></i>`;
export const SLOT_RE = /<i hidden data-ps="([a-z]+):(tr|en)"><\/i>[\s\S]*?<i hidden data-pe="\1:\2"><\/i>/g;

export function pricingCards(plans, lang, contactHref) {
  const T = L(lang);
  return `<div class="plans">${plans.map((p, i) => {
    const ym = yearlyMonthly(p);
    const from = p.custom ? `<small class="from">${T('başlangıç', 'from')}</small>` : '';
    const mPrice = p.monthly != null
      ? `<p class="price">${from}<span data-count="${p.monthly}">${tl(p.monthly, lang)}</span><small> TL</small></p>`
      : `<p class="price price-q">${T('Teklifle', 'On request')}</p>`;
    const mPer = p.custom ? T('+ KDV / ay, teklifle', '+ VAT / month, by quote') : T('+ KDV / ay', '+ VAT / month');
    const yPrice = ym ? `<p class="price">${tl(ym, lang)}<small> TL</small></p>` : mPrice.replace(/ data-count="\d+"/, '');
    const yPer = ym ? T(`+ KDV / ay · yıllık ${tl(p.yearly, lang)} TL`, `+ VAT / month · ${tl(p.yearly, lang)} TL yearly`) : (p.custom ? T('sözleşmeyle', 'by contract') : mPer);
    const pop = p.key === POPULAR;
    const cta = p.custom
      ? `<a class="btn btn-ghost" href="${esc(contactHref)}">${T('Teklif isteyin', 'Get a quote')}</a>`
      : `<a class="btn${pop ? '' : ' btn-ghost'}" href="${SIGNUP}" data-signup>${T('14 gün ücretsiz deneyin', 'Try free for 14 days')}</a>`;
    return `<div class="plan card rv${pop ? ' pop' : ''}" style="--rd:${i * 70}ms">${pop ? `<span class="badge">${T('Önerilen', 'Recommended')}</span>` : ''}<h2>${esc(p.name[lang])}</h2><p class="bl">${esc(p.desc[lang])}</p><div class="pbox"><div class="mo">${mPrice}<p class="per">${mPer}</p></div><div class="yr">${yPrice}<p class="per">${yPer}</p></div></div><ul class="checks">${keyPoints(p, lang).map((x) => `<li>${ICON(CHECK)}${esc(x)}</li>`).join('')}</ul>${cta}</div>`;
  }).join('')}</div>`;
}

export function vatNote(plans, lang) {
  const T = L(lang);
  const pp = priced(plans);
  const list = pp.map((p) => `${esc(p.name[lang])} ${tl(p.monthly * (1 + p.vat), lang)} TL`).join(' · ');
  return `<p class="note center">${list ? `${T('KDV dahil aylık', 'Monthly incl. VAT')}: ${list}. ` : ''}${T('Ödeme havale/EFT ile; kartla ödeme yakında (PayTR).', 'Payment by bank transfer; card payments coming soon (PayTR).')}</p>`;
}

export function compareTable(plans, lang) {
  const T = L(lang);
  const tick = `${ICON(CHECK)}<span class="sr-only">${T('Var', 'Included')}</span>`;
  const no = `<span class="no"><span aria-hidden="true">—</span><span class="sr-only">${T('Yok', 'Not included')}</span></span>`;
  const row = (label, cells, hint) => `<tr><th scope="row">${esc(label)}${hint ? `<span class="hint">${esc(hint)}</span>` : ''}</th>${cells.map((c) => `<td>${c}</td>`).join('')}</tr>`;
  const rows = [];
  for (const [k, tr, en, htr, hen] of LIMIT_ROWS) {
    if (!plans.some((p) => p.limits[k] !== undefined)) continue;
    rows.push(row(T(tr, en), plans.map((p) => esc(fmtLimit(p, k, lang))), htr ? T(htr, hen) : null));
  }
  rows.push(row(T('Kişi (alıcı)', 'Contacts'), plans.map(() => T('Sınırsız*', 'Unlimited*')), T('* Adil kullanım koşuluyla.', '* Subject to fair use.')));
  for (const [tr, en] of ALL_ROWS) rows.push(row(T(tr, en), plans.map(() => tick)));
  for (const [k, tr, en, htr, hen] of FEATURE_ROWS) {
    if (!plans.some((p) => typeof p.features[k] === 'boolean')) continue;
    rows.push(row(T(tr, en), plans.map((p) => (p.features[k] ? tick : no)), htr ? T(htr, hen) : null));
  }
  rows.push(row(T('Destek', 'Support'), plans.map((p) => esc((SUPPORT[p.key] ?? (p.custom ? SUPPORT.enterprise : SUPPORT.starter))[lang === 'tr' ? 0 : 1]))));
  return `<table class="tbl"><caption class="sr-only">${T('Paket karşılaştırması', 'Plan comparison')}</caption><thead><tr><th scope="col">${T('Özellik', 'Feature')}</th>${plans.map((p) => `<th scope="col"${p.key === POPULAR ? ' class="pop"' : ''}>${esc(p.name[lang])}</th>`).join('')}</tr></thead><tbody>${rows.join('')}</tbody></table>`;
}

export function teaser(plans, lang) {
  const T = L(lang);
  return `<div class="grid g4 plans-mini">${plans.map((p) => {
    const price = p.monthly != null
      ? `<p class="price">${p.custom ? `<small class="from">${T('başlangıç', 'from')}</small>` : ''}<span data-count="${p.monthly}">${tl(p.monthly, lang)}</span> <small>TL</small></p>`
      : `<p class="price">${T('Teklifle', 'On request')}</p>`;
    return `<div class="card rv${p.key === POPULAR ? ' pop' : ''}">${p.key === POPULAR ? `<span class="pop-tag">${T('Önerilen', 'Recommended')}</span>` : ''}<h3>${esc(p.name[lang])}</h3>${price}<p class="per">${T('+ KDV / ay', '+ VAT / month')}</p><p>${esc(p.desc[lang])}</p></div>`;
  }).join('')}</div>`;
}

// ── Structured data ────────────────────────────────────────
const pricingUrl = (lang) => `${SITE}${lang === 'tr' ? '/fiyatlar' : '/en/pricing'}`;
export function softwareLd(plans, lang) {
  const pp = plans.filter((p) => p.monthly);
  return {
    '@type': 'SoftwareApplication', '@id': `${SITE}/#app`, name: 'Messora',
    applicationCategory: 'BusinessApplication', applicationSubCategory: 'WhatsApp Business API platform', operatingSystem: 'Web', url: SITE, inLanguage: ['tr', 'en'],
    description: lang === 'tr'
      ? 'Resmi WhatsApp Business Cloud API üzerinde ekip gelen kutusu, izinli kampanyalar, yapay zekâ ajanları, otomasyonlar, analitik ve API.'
      : 'Team inbox, consent-aware campaigns, AI agents, automations, analytics and an API on the official WhatsApp Business Cloud API.',
    publisher: { '@id': `${SITE}/#org` },
    ...(pp.length ? { offers: { '@type': 'AggregateOffer', priceCurrency: 'TRY', lowPrice: Math.min(...pp.map((p) => p.monthly)), highPrice: Math.max(...priced(plans).map((p) => p.monthly), Math.min(...pp.map((p) => p.monthly))), offerCount: plans.length, url: pricingUrl(lang) } } : {}),
  };
}
export function productsLd(plans, lang) {
  return plans.filter((p) => p.monthly).map((p) => ({
    '@type': 'Product', name: `Messora ${p.name[lang]}`, description: p.desc[lang], brand: { '@type': 'Brand', name: 'Messora' },
    offers: {
      '@type': 'Offer', url: pricingUrl(lang), priceCurrency: 'TRY', price: p.monthly, availability: 'https://schema.org/InStock',
      priceSpecification: { '@type': 'UnitPriceSpecification', price: p.monthly, priceCurrency: 'TRY', unitCode: 'MON', valueAddedTaxIncluded: false, ...(p.custom ? { minPrice: p.monthly } : {}) },
    },
  }));
}
/** JSON-LD for a page kind ('home' | 'pricing'), as the text of a <script type="application/ld+json">. */
export function plansLdJson(plans, kind, lang) {
  const graph = [softwareLd(plans, lang), ...(kind === 'pricing' ? productsLd(plans, lang) : [])];
  return JSON.stringify({ '@context': 'https://schema.org', '@graph': graph }).replace(/</g, '\\u003c');
}
export const LD_RE = /<script type="application\/ld\+json" data-plans-ld="(home|pricing):(tr|en)">[\s\S]*?<\/script>/g;

/** Re-renders every plan block in a built page for new plan data. */
export function rerender(html, oldPlans, newPlans, contactHref) {
  let out = html.replace(SLOT_RE, (_m, name, lang) => {
    const f = { cards: () => pricingCards(newPlans, lang, contactHref(lang)), vat: () => vatNote(newPlans, lang), table: () => compareTable(newPlans, lang), teaser: () => teaser(newPlans, lang) }[name];
    return f ? slot(`${name}:${lang}`, f()) : _m;
  });
  out = out.replace(LD_RE, (_m, kind, lang) => `<script type="application/ld+json" data-plans-ld="${kind}:${lang}">${plansLdJson(newPlans, kind, lang)}</script>`);
  for (const [a, b] of phrasePairs(oldPlans, newPlans)) out = out.split(a).join(b);
  return out;
}
