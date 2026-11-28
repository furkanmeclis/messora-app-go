// Plans and limits (docs/PHASE-17.md §1.5, pricing analysis). TL + KDV.
import type { Lang } from './types';

export interface Plan {
  key: 'starter' | 'growth' | 'pro' | 'enterprise';
  name: Record<Lang, string>;
  monthly: number;          // TL + KDV per month
  yearly: number | null;    // TL + KDV per year (12 months for the price of 10)
  from?: boolean;           // "starting at"
  popular?: boolean;
  blurb: Record<Lang, string>;
}

export const plans: Plan[] = [
  { key: 'starter', name: { tr: 'Başlangıç', en: 'Starter' }, monthly: 1290, yearly: 12900,
    blurb: { tr: 'Tek numara, küçük ekip. Gelen kutusu, kampanya ve ilk AI ajanınız.', en: 'One number, a small team. Inbox, campaigns and your first AI agent.' } },
  { key: 'growth', name: { tr: 'Büyüme', en: 'Growth' }, monthly: 2990, yearly: 29900, popular: true,
    blurb: { tr: 'Ekibi büyüyen işletmeler için: daha çok kullanıcı, akış ve API.', en: 'For growing teams: more users, flows and API access.' } },
  { key: 'pro', name: { tr: 'Pro', en: 'Pro' }, monthly: 6490, yearly: 64900,
    blurb: { tr: 'Birden çok numara, sınırsız akış, yüksek API kotası.', en: 'Several numbers, unlimited flows, a high API quota.' } },
  { key: 'enterprise', name: { tr: 'Kurumsal', en: 'Enterprise' }, monthly: 14990, yearly: null, from: true,
    blurb: { tr: 'Özel limitler, kendi SMTP ve AI anahtarlarınız, sözleşmeli hizmet.', en: 'Custom limits, your own SMTP and AI keys, a contracted service.' } },
];

type Cell = string | boolean;
export interface LimitRow { label: Record<Lang, string>; values: Record<Lang, Cell[]>; hint?: Record<Lang, string> }

const same = (v: Cell[]): Record<Lang, Cell[]> => ({ tr: v, en: v });

export const limits: LimitRow[] = [
  { label: { tr: 'WhatsApp numarası', en: 'WhatsApp numbers' }, values: { tr: ['1', '1', '3', 'Özel'], en: ['1', '1', '3', 'Custom'] } },
  { label: { tr: 'Kullanıcı', en: 'Users' }, values: { tr: ['2', '5', '12', 'Özel'], en: ['2', '5', '12', 'Custom'] } },
  { label: { tr: 'AI ajanı', en: 'AI agents' }, values: { tr: ['1', '3', '10', 'Özel'], en: ['1', '3', '10', 'Custom'] } },
  { label: { tr: 'Aktif otomasyon akışı', en: 'Active automation flows' }, values: { tr: ['3', '20', 'Sınırsız', 'Sınırsız'], en: ['3', '20', 'Unlimited', 'Unlimited'] } },
  { label: { tr: 'Aylık AI kredisi', en: 'AI credits per month' }, values: { tr: ['300', '2.000', '8.000', 'Özel havuz'], en: ['300', '2,000', '8,000', 'Custom pool'] },
    hint: { tr: 'Standart modelde 1 kredi ≈ 1 AI yanıtı.', en: 'On the standard model 1 credit ≈ 1 AI reply.' } },
  { label: { tr: 'API çağrısı / ay', en: 'API calls per month' }, values: { tr: ['Yalnızca webhook', '100 bin', '1 milyon', 'Özel'], en: ['Webhooks only', '100k', '1 million', 'Custom'] } },
  { label: { tr: 'Kişi (alıcı)', en: 'Contacts' }, values: { tr: ['Sınırsız*', 'Sınırsız*', 'Sınırsız*', 'Sınırsız*'], en: ['Unlimited*', 'Unlimited*', 'Unlimited*', 'Unlimited*'] },
    hint: { tr: '* Adil kullanım koşuluyla.', en: '* Subject to fair use.' } },
  { label: { tr: 'Ekip gelen kutusu, notlar, hızlı yanıtlar', en: 'Team inbox, notes, quick replies' }, values: same([true, true, true, true]) },
  { label: { tr: 'Kampanyalar, izin ve ret yönetimi', en: 'Campaigns, consent and opt-out' }, values: same([true, true, true, true]) },
  { label: { tr: 'Şablon oluşturucu (carousel, süreli teklif, OTP, Flows)', en: 'Template builder (carousel, limited-time offer, OTP, Flows)' }, values: same([true, true, true, true]) },
  { label: { tr: '11 hazır otomasyon şablonu', en: '11 ready-made automation presets' }, values: same([true, true, true, true]) },
  { label: { tr: 'Gönderim öncesi maliyet ve tahmin', en: 'Cost and forecast before sending' }, values: same([true, true, true, true]) },
  { label: { tr: 'Dışa aktarım ve zamanlanmış raporlar', en: 'Exports and scheduled reports' }, values: same([true, true, true, true]) },
  { label: { tr: 'Kendi alt alan adınız veya alan adınız', en: 'Your own subdomain or domain' }, values: same([true, true, true, true]) },
  { label: { tr: 'İki adımlı doğrulama, roller, denetim kaydı', en: '2FA, roles, audit log' }, values: same([true, true, true, true]) },
  { label: { tr: 'Kendi SMTP sunucunuz', en: 'Your own SMTP server' }, values: same([false, false, false, true]) },
  { label: { tr: 'Kendi AI anahtarlarınız', en: 'Your own AI keys' }, values: same([false, false, false, true]) },
  { label: { tr: 'Destek', en: 'Support' }, values: { tr: ['E-posta', 'E-posta + canlı sohbet', 'Öncelikli + kurulum görüşmesi', 'Atanmış yönetici, SLA'], en: ['E-mail', 'E-mail + live chat', 'Priority + onboarding call', 'Account manager, SLA'] } },
];

export function tl(n: number, lang: Lang): string {
  return new Intl.NumberFormat(lang === 'tr' ? 'tr-TR' : 'en-GB').format(n);
}
