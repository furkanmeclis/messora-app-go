// Shared content types. Feature and solution pages are rendered from these.
export type Lang = 'tr' | 'en';

export type FeatureKey = 'inbox' | 'campaigns' | 'ai' | 'automations' | 'templates' | 'analytics' | 'api';
export type SolutionKey = 'ecommerce' | 'retail' | 'health' | 'education' | 'travel' | 'realestate' | 'services';

/** Names of the icons in src/components/Icon.astro. */
export type IconName =
  | 'inbox' | 'megaphone' | 'sparkle' | 'flow' | 'template' | 'chart' | 'code'
  | 'shield' | 'lock' | 'key' | 'globe' | 'server' | 'database' | 'users' | 'user'
  | 'tag' | 'clock' | 'calendar' | 'check' | 'check-circle' | 'arrow-right' | 'phone'
  | 'cart' | 'store' | 'health' | 'education' | 'plane' | 'home' | 'wrench'
  | 'message' | 'send' | 'filter' | 'bell' | 'receipt' | 'coin' | 'download'
  | 'link' | 'webhook' | 'handoff' | 'note' | 'bolt' | 'eye' | 'mail' | 'building'
  | 'layers' | 'refresh' | 'pause' | 'list' | 'plus' | 'minus' | 'info' | 'x' | 'menu' | 'chevron-down';

export interface Faq { q: string; a: string }
export interface Block { title: string; body: string; points?: string[] }
export interface Step { title: string; body: string }

export interface Feature {
  key: FeatureKey;
  /** Localized URL slug (lowercase, ASCII, hyphens). */
  slug: string;
  /** Short label for navigation (1–3 words). */
  nav: string;
  icon: IconName;
  /** <title> without the brand suffix; ≤ 55 characters. */
  seoTitle: string;
  /** Meta description, 140–160 characters. */
  seoDescription: string;
  eyebrow: string;
  h1: string;
  lead: string;
  /** One or two sentences for cards on the hub and home page. */
  summary: string;
  /** Three short, factual proof points shown under the hero. */
  highlights: string[];
  /** 4–6 capability sections. */
  blocks: Block[];
  /** 3–4 steps: how it works. */
  steps: Step[];
  /** Which plans include it and the relevant limits (from the pricing table). */
  plans: string;
  faq: Faq[];
  related: FeatureKey[];
}

export interface UseCaseFlow {
  title: string;
  /** What starts it, e.g. "Sipariş kargoya verildiğinde". */
  trigger: string;
  /** 3–5 concrete steps. */
  steps: string[];
  /** What the business gets (qualitative — no invented numbers). */
  outcome: string;
  features: FeatureKey[];
}

export interface ExampleTemplate {
  name: string;
  category: 'marketing' | 'utility' | 'authentication';
  /** Template body with Meta-style variables {{1}}, {{2}}. */
  body: string;
  /** Optional quick-reply or URL button labels. */
  buttons?: string[];
}

export interface Solution {
  key: SolutionKey;
  slug: string;
  nav: string;
  icon: IconName;
  seoTitle: string;
  seoDescription: string;
  eyebrow: string;
  h1: string;
  lead: string;
  summary: string;
  /** Three typical problems of the sector. */
  pains: Block[];
  /** 3–4 realistic use-case flows. */
  flows: UseCaseFlow[];
  /** 2–3 example template messages. */
  templates: ExampleTemplate[];
  /** Sector-specific compliance / consent note (1 paragraph). */
  compliance: string;
  faq: Faq[];
  features: FeatureKey[];
}
