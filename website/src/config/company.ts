// ─────────────────────────────────────────────────────────────────────────
// COMPANY DETAILS — the single place for every legal/company field.
//
// The legal entity is not settled yet. Every field set to `null` renders as a
// visible placeholder (e.g. "[Şirket unvanı]") on the site, in the legal
// texts and in e-mails. Fill the value in and rebuild; nothing else changes.
// ─────────────────────────────────────────────────────────────────────────
import type { Lang } from '../data/types';

type Field = { value: string | null; label: Record<Lang, string> };

export const company = {
  unvan: { value: null, label: { tr: 'Şirket unvanı', en: 'Company legal name' } },
  marka: { value: 'Messora', label: { tr: 'Marka', en: 'Brand' } },
  mersis: { value: null, label: { tr: 'MERSİS numarası', en: 'MERSIS number' } },
  ticaretSicil: { value: null, label: { tr: 'Ticaret sicil müdürlüğü ve sicil numarası', en: 'Trade registry office and number' } },
  vergiDairesi: { value: null, label: { tr: 'Vergi dairesi', en: 'Tax office' } },
  vergiNo: { value: null, label: { tr: 'Vergi numarası', en: 'Tax number' } },
  adres: { value: null, label: { tr: 'Merkez adresi', en: 'Registered address' } },
  sehir: { value: null, label: { tr: 'Merkezin bulunduğu il', en: 'City of the registered seat' } },
  kep: { value: null, label: { tr: 'KEP adresi', en: 'Registered e-mail (KEP) address' } },
  telefon: { value: null, label: { tr: 'Telefon', en: 'Phone' } },
  // Public addresses. They need an inbound mailbox before launch (see README).
  eposta: { value: 'merhaba@messora.tech', label: { tr: 'E-posta', en: 'E-mail' } },
  kvkkEposta: { value: 'kvkk@messora.tech', label: { tr: 'KVKK e-posta adresi', en: 'Privacy e-mail' } },
  temsilci: { value: null, label: { tr: 'Veri sorumlusu irtibat kişisi', en: 'Data controller contact person' } },
  web: { value: 'https://messora.tech', label: { tr: 'Web sitesi', en: 'Website' } },
  basvuruUrl: { value: null, label: { tr: 'Başvuru formu', en: 'Request form' } },
} satisfies Record<string, Field>;

export type CompanyKey = keyof typeof company;

/** Value for display: the configured value or the bracketed placeholder. */
export function companyText(key: CompanyKey, lang: Lang): string {
  if (key === 'basvuruUrl') return lang === 'tr' ? 'https://messora.tech/kvkk-basvuru' : 'https://messora.tech/en/data-request';
  const f = company[key] as Field;
  return f.value ?? `[${f.label[lang]}]`;
}

export function isPlaceholder(key: CompanyKey): boolean {
  if (key === 'basvuruUrl') return false;
  return (company[key] as Field).value === null;
}

/** HTML for Markdown token replacement: placeholders are marked visibly. */
export function companyHtml(key: CompanyKey, lang: Lang): string {
  const t = companyText(key, lang).replace(/&/g, '&amp;').replace(/</g, '&lt;');
  if (isPlaceholder(key)) return `<span class="ph">${t}</span>`;
  if (key === 'eposta' || key === 'kvkkEposta') return `<a href="mailto:${t}">${t}</a>`;
  if (key === 'web' || key === 'basvuruUrl') return `<a href="${t.replace('https://messora.tech', '') || '/'}">${t.replace('https://', '')}</a>`;
  return t;
}
