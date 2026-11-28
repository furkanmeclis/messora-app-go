// Site behaviour: menus, reveal-on-scroll, consent + Google tag, signup link
// tracking (UTM pass-through, cross-domain linker) and forms.
type Cfg = { gtag?: string; ads?: string; leadLabel?: string; signupLabel?: string; mode?: 'basic' | 'advanced' };
type Consent = { a: boolean; m: boolean; v: 1; t: number };

declare global {
  interface Window { dataLayer: unknown[]; gtag: (...args: unknown[]) => void; __MSR_CFG?: Cfg }
}

const cfg: Cfg = window.__MSR_CFG || {};
const tagIds = [cfg.gtag, cfg.ads].filter((x): x is string => !!x && /^(G|AW)-[A-Z0-9]+$/.test(x));
const lang = document.documentElement.lang === 'en' ? 'en' : 'tr';

/* ── Menus ─────────────────────────────────────────────── */
const megas = document.querySelectorAll<HTMLButtonElement>('[data-mega]');
function closeMegas(except?: HTMLButtonElement) {
  megas.forEach((b) => {
    if (b === except) return;
    b.setAttribute('aria-expanded', 'false');
    document.getElementById(b.getAttribute('aria-controls')!)?.classList.remove('open');
  });
}
megas.forEach((b) => {
  b.addEventListener('click', (e) => {
    e.stopPropagation();
    const open = b.getAttribute('aria-expanded') !== 'true';
    closeMegas(b);
    b.setAttribute('aria-expanded', String(open));
    document.getElementById(b.getAttribute('aria-controls')!)?.classList.toggle('open', open);
  });
});
document.addEventListener('click', (e) => {
  if (!(e.target as Element).closest('.mega')) closeMegas();
});
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    const open = [...megas].find((b) => b.getAttribute('aria-expanded') === 'true');
    closeMegas();
    open?.focus();
  }
});
const burger = document.querySelector<HTMLButtonElement>('[data-burger]');
burger?.addEventListener('click', () => {
  const open = burger.getAttribute('aria-expanded') !== 'true';
  burger.setAttribute('aria-expanded', String(open));
  document.getElementById('mnav')?.classList.toggle('open', open);
  document.body.style.overflow = open ? 'hidden' : '';
});

/* ── Reveal on scroll ───────────────────────────────────── */
const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
const rv = document.querySelectorAll('.rv');
if (!reduce && 'IntersectionObserver' in window) {
  const io = new IntersectionObserver((es) => es.forEach((en) => {
    if (en.isIntersecting) { en.target.classList.add('in'); io.unobserve(en.target); }
  }), { rootMargin: '0px 0px -8% 0px' });
  rv.forEach((el) => io.observe(el));
} else rv.forEach((el) => el.classList.add('in'));

/* ── Consent + Google tag (Consent Mode v2) ─────────────── */
const CK = 'msr_consent';
function readConsent(): Consent | null {
  try {
    const m = document.cookie.match(/(?:^|; )msr_consent=([^;]+)/);
    return m ? (JSON.parse(decodeURIComponent(m[1])) as Consent) : null;
  } catch { return null; }
}
function writeConsent(c: Consent) {
  const secure = location.protocol === 'https:' ? '; Secure' : '';
  document.cookie = `${CK}=${encodeURIComponent(JSON.stringify(c))}; Max-Age=${60 * 60 * 24 * 365}; Path=/; SameSite=Lax${secure}`;
}
let tagLoaded = false;
function loadTag() {
  if (tagLoaded || !tagIds.length) return;
  tagLoaded = true;
  const s = document.createElement('script');
  s.async = true;
  s.src = `https://www.googletagmanager.com/gtag/js?id=${tagIds[0]}`;
  document.head.appendChild(s);
  window.gtag('js', new Date());
  window.gtag('set', 'linker', { domains: ['app.messora.tech'], accept_incoming: true });
  tagIds.forEach((id) => window.gtag('config', id, { linker: { domains: ['app.messora.tech'] } }));
}
function applyConsent(c: Consent | null) {
  const a = c?.a ? 'granted' : 'denied';
  const m = c?.m ? 'granted' : 'denied';
  window.gtag('consent', 'update', { analytics_storage: a, ad_storage: m, ad_user_data: m, ad_personalization: m });
  if (cfg.mode === 'advanced' || c?.a || c?.m) loadTag();
}
const cc = document.getElementById('cc');
const ccA = document.getElementById('cc-a') as HTMLInputElement | null;
const ccM = document.getElementById('cc-m') as HTMLInputElement | null;
function showBanner(more = false) {
  if (!cc) return;
  const c = readConsent();
  if (ccA) ccA.checked = !!c?.a;
  if (ccM) ccM.checked = !!c?.m;
  cc.classList.add('show');
  cc.classList.toggle('more', more);
  document.getElementById('cc-more')!.hidden = more;
  document.getElementById('cc-save')!.hidden = !more;
  (cc.querySelector('.btn') as HTMLElement | null)?.focus({ preventScroll: true });
}
function decide(a: boolean, m: boolean) {
  const c: Consent = { a, m, v: 1, t: Date.now() };
  writeConsent(c);
  applyConsent(c);
  cc?.classList.remove('show', 'more');
}
cc?.addEventListener('click', (e) => {
  const act = (e.target as HTMLElement).closest<HTMLElement>('[data-cc]')?.dataset.cc;
  if (act === 'accept') decide(true, true);
  else if (act === 'reject') decide(false, false);
  else if (act === 'save') decide(!!ccA?.checked, !!ccM?.checked);
  else if (act === 'more') showBanner(true);
});
document.querySelectorAll('[data-cc-open]').forEach((b) => b.addEventListener('click', () => showBanner(true)));
const existing = readConsent();
if (existing) applyConsent(existing);
else if (tagIds.length) {
  if (cfg.mode === 'advanced') loadTag();
  showBanner();
}

/* ── Signup links: UTM pass-through + conversion event ──── */
const KEEP = ['utm_source', 'utm_medium', 'utm_campaign', 'utm_term', 'utm_content', 'gclid', 'gbraid', 'wbraid'];
const params = new URLSearchParams(location.search);
let attrib: Record<string, string> = {};
try { attrib = JSON.parse(sessionStorage.getItem('msr_attr') || '{}'); } catch { /* storage blocked */ }
KEEP.forEach((k) => { const v = params.get(k); if (v) attrib[k] = v.slice(0, 200); });
try { sessionStorage.setItem('msr_attr', JSON.stringify(attrib)); } catch { /* storage blocked */ }
document.querySelectorAll<HTMLAnchorElement>('a[href^="https://app.messora.tech/signup"]').forEach((a) => {
  const u = new URL(a.href);
  Object.entries(attrib).forEach(([k, v]) => u.searchParams.set(k, v));
  if (!u.searchParams.has('lang')) u.searchParams.set('lang', lang);
  a.href = u.toString();
  a.addEventListener('click', () => {
    window.gtag('event', 'sign_up_click', { link_url: a.href, page_path: location.pathname });
    if (cfg.ads && cfg.signupLabel) window.gtag('event', 'conversion', { send_to: `${cfg.ads}/${cfg.signupLabel}` });
  });
});

/* ── Forms ─────────────────────────────────────────────── */
document.querySelectorAll<HTMLFormElement>('form[data-form]').forEach((form) => {
  const started = Date.now();
  const status = form.querySelector<HTMLElement>('.form-status')!;
  const btn = form.querySelector<HTMLButtonElement>('button[type=submit]')!;
  const label = btn.textContent;
  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    let firstBad: HTMLElement | null = null;
    form.querySelectorAll<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>('input, textarea, select').forEach((el) => {
      if (el.closest('.hp')) return;
      const bad = !el.checkValidity();
      el.setAttribute('aria-invalid', String(bad));
      if (bad && !firstBad) firstBad = el;
    });
    if (firstBad) { (firstBad as HTMLElement).focus(); return; }
    const data: Record<string, unknown> = {};
    new FormData(form).forEach((v, k) => { data[k] = typeof v === 'string' ? v.trim() : ''; });
    data.t = started;
    data.lang = lang;
    data.page = location.pathname;
    data.attribution = attrib;
    btn.disabled = true;
    btn.textContent = form.dataset.sending || '…';
    status.className = 'form-status';
    try {
      const res = await fetch(`/api/forms/${form.dataset.form}`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(data),
      });
      const body = await res.json().catch(() => ({}));
      if (res.ok && body.ok) {
        status.className = 'form-status ok';
        status.innerHTML = '';
        const s = document.createElement('strong');
        s.textContent = `${form.dataset.ok} ${body.ref}`;
        status.append(s, document.createElement('br'), document.createTextNode(form.dataset.okmail || ''));
        form.querySelectorAll<HTMLElement>('.field, .check, .row2').forEach((el) => (el.hidden = true));
        btn.hidden = true;
        window.gtag('event', 'generate_lead', { form_type: form.dataset.form });
        if (cfg.ads && cfg.leadLabel) window.gtag('event', 'conversion', { send_to: `${cfg.ads}/${cfg.leadLabel}` });
      } else {
        status.className = 'form-status bad';
        status.textContent = res.status === 429 ? form.dataset.rate || '' : form.dataset.err || '';
        if (body.field) {
          const el = form.querySelector<HTMLElement>(`[name="${body.field}"]`);
          el?.setAttribute('aria-invalid', 'true');
          el?.focus();
        }
      }
    } catch {
      status.className = 'form-status bad';
      status.textContent = form.dataset.err || '';
    } finally {
      btn.disabled = false;
      btn.textContent = label;
    }
  });
});

export {};
