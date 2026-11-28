// Interface behaviour: header (scroll states, mega-menu disclosures, mobile
// sheet), scroll reveals, cursor spotlight, count-ups, FAQ height animation,
// sticky feature story, hero product animation, reading progress and TOC.
// Everything degrades to a static, fully readable page without JS or with
// prefers-reduced-motion.

const root = document.documentElement;
const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
const finePointer = matchMedia('(hover: hover) and (pointer: fine)').matches;
const lang = root.lang === 'en' ? 'en' : 'tr';

/* ── Scroll scheduler: one rAF per frame for all scroll work ─ */
const onScroll: Array<() => void> = [];
let ticking = false;
addEventListener('scroll', () => {
  if (ticking) return;
  ticking = true;
  requestAnimationFrame(() => { ticking = false; onScroll.forEach((f) => f()); });
}, { passive: true });

/* ── Header ──────────────────────────────────────────────── */
const hdr = document.querySelector<HTMLElement>('[data-hdr]');
const megas = [...document.querySelectorAll<HTMLButtonElement>('[data-mega]')];
const burger = document.querySelector<HTMLButtonElement>('[data-burger]');
const sheet = document.querySelector<HTMLElement>('[data-msheet]');
let sheetOpen = false;
let anchorY = scrollY;

function syncOpen() {
  hdr?.classList.toggle('is-open', sheetOpen || megas.some((b) => b.getAttribute('aria-expanded') === 'true'));
}
function hdrUpdate() {
  if (!hdr) return;
  const y = scrollY;
  hdr.classList.toggle('is-scrolled', y > 8);
  const busy = hdr.classList.contains('is-open') || !!hdr.querySelector(':focus-visible');
  if (y < 200) { hdr.classList.remove('is-hidden'); anchorY = y; }
  else if (y - anchorY > 10) { if (!busy) hdr.classList.add('is-hidden'); anchorY = y; }
  else if (anchorY - y > 10) { hdr.classList.remove('is-hidden'); anchorY = y; }
  root.style.setProperty('--hdr-vis', hdr.classList.contains('is-hidden') ? '0px' : `${hdr.offsetHeight}px`);
}
onScroll.push(hdrUpdate);
hdrUpdate();

const panelOf = (b: HTMLButtonElement) => document.getElementById(b.getAttribute('aria-controls')!)!;
function setMega(b: HTMLButtonElement, open: boolean) {
  b.setAttribute('aria-expanded', String(open));
  panelOf(b).classList.toggle('open', open);
}
function closeMegas(except?: HTMLButtonElement) {
  megas.forEach((b) => { if (b !== except) setMega(b, false); });
  syncOpen();
}
let hoverAt = 0;
let closeTimer = 0;
megas.forEach((b) => {
  const li = b.parentElement!;
  const panel = panelOf(b);
  b.addEventListener('click', (e) => {
    // A mouse click right after a hover-open keeps the menu open.
    if (e.detail > 0 && performance.now() - hoverAt < 500) return;
    const open = b.getAttribute('aria-expanded') !== 'true';
    closeMegas(b);
    setMega(b, open);
    syncOpen();
  });
  li.addEventListener('pointerenter', (e) => {
    if (e.pointerType !== 'mouse') return;
    clearTimeout(closeTimer);
    if (b.getAttribute('aria-expanded') !== 'true') hoverAt = performance.now();
    closeMegas(b);
    setMega(b, true);
    syncOpen();
  });
  li.addEventListener('pointerleave', (e) => {
    if (e.pointerType !== 'mouse') return;
    closeTimer = window.setTimeout(() => { setMega(b, false); syncOpen(); }, 160);
  });
  b.addEventListener('keydown', (e) => {
    if (e.key !== 'ArrowDown') return;
    e.preventDefault();
    closeMegas(b);
    setMega(b, true);
    syncOpen();
    panel.querySelector<HTMLElement>('a')?.focus();
  });
  panel.addEventListener('keydown', (e) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
    const links = [...panel.querySelectorAll<HTMLElement>('a')];
    const i = links.indexOf(document.activeElement as HTMLElement);
    e.preventDefault();
    links[(i + (e.key === 'ArrowDown' ? 1 : -1) + links.length) % links.length]?.focus();
  });
  li.addEventListener('focusout', (e) => {
    if (!li.contains(e.relatedTarget as Node)) { setMega(b, false); syncOpen(); }
  });
});
document.addEventListener('click', (e) => {
  if (!(e.target as Element).closest('.nav-list > li')) closeMegas();
});

// Sliding hover pill behind the top-level links.
const navList = document.querySelector<HTMLElement>('[data-nav]');
const pill = document.querySelector<HTMLElement>('[data-pill]');
if (navList && pill && finePointer) {
  navList.querySelectorAll<HTMLElement>('.nav-a').forEach((a) => a.addEventListener('pointerenter', () => {
    const jump = pill.style.opacity !== '1';
    if (jump) pill.style.transition = 'none';
    pill.style.width = `${a.offsetWidth}px`;
    pill.style.transform = `translateX(${a.offsetLeft}px)`;
    if (jump) { void pill.offsetWidth; pill.style.transition = ''; }
    pill.style.opacity = '1';
  }));
  navList.addEventListener('pointerleave', () => { pill.style.opacity = '0'; });
}

// Mobile sheet: full-screen, staggered items, scroll lock, focus kept inside.
const mainEl = document.getElementById('main');
const footEl = document.querySelector<HTMLElement>('.ftr');
function setSheet(open: boolean, restoreFocus = false) {
  if (!burger || !sheet) return;
  sheetOpen = open;
  burger.setAttribute('aria-expanded', String(open));
  burger.setAttribute('aria-label', open ? (lang === 'tr' ? 'Menüyü kapat' : 'Close menu') : (lang === 'tr' ? 'Menü' : 'Menu'));
  sheet.classList.toggle('open', open);
  root.classList.toggle('lock', open);
  if (mainEl) mainEl.inert = open;
  if (footEl) footEl.inert = open;
  hdr?.classList.remove('is-hidden');
  syncOpen();
  if (restoreFocus) burger.focus();
}
burger?.addEventListener('click', () => setSheet(!sheetOpen));
sheet?.querySelectorAll<HTMLButtonElement>('[data-acc]').forEach((b) => b.addEventListener('click', () => {
  const open = b.getAttribute('aria-expanded') !== 'true';
  b.setAttribute('aria-expanded', String(open));
  document.getElementById(b.getAttribute('aria-controls')!)?.classList.toggle('open', open);
}));
sheet?.addEventListener('click', (e) => { if ((e.target as Element).closest('a')) setSheet(false); });
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    if (sheetOpen) { setSheet(false, true); return; }
    const open = megas.find((b) => b.getAttribute('aria-expanded') === 'true');
    closeMegas();
    open?.focus();
  }
  if (e.key === 'Tab' && sheetOpen && sheet && burger) {
    const f = [burger, ...sheet.querySelectorAll<HTMLElement>('a, button')].filter((el) => el.offsetParent !== null);
    const i = f.indexOf(document.activeElement as HTMLElement);
    if (e.shiftKey && i <= 0) { e.preventDefault(); f[f.length - 1].focus(); }
    else if (!e.shiftKey && i === f.length - 1) { e.preventDefault(); f[0].focus(); }
  }
});
matchMedia('(min-width: 1041px)').addEventListener('change', (m) => { if (m.matches && sheetOpen) setSheet(false); });

/* ── Scroll reveal (staggered per batch) + count-ups ──────── */
const fmt = new Intl.NumberFormat(lang === 'tr' ? 'tr-TR' : 'en-GB');
function countUp(el: HTMLElement) {
  const to = Number(el.dataset.count);
  if (!Number.isFinite(to) || reduce) return;
  el.style.display = 'inline-block';
  el.style.minWidth = `${el.getBoundingClientRect().width}px`;
  const t0 = performance.now();
  const dur = 1300;
  const step = (t: number) => {
    const k = Math.min(1, (t - t0) / dur);
    const e = k === 1 ? 1 : 1 - Math.pow(2, -10 * k);
    el.textContent = fmt.format(Math.round(to * e));
    if (k < 1) requestAnimationFrame(step);
  };
  requestAnimationFrame(step);
}
const revealEls = document.querySelectorAll<HTMLElement>('.rv, .rv-h');
const counters = document.querySelectorAll<HTMLElement>('[data-count]');
if (!reduce && 'IntersectionObserver' in window) {
  const io = new IntersectionObserver((es) => {
    let n = 0;
    es.forEach((en) => {
      if (!en.isIntersecting) return;
      const el = en.target as HTMLElement;
      io.unobserve(el);
      if (el.dataset.count !== undefined && !el.classList.contains('rv')) { countUp(el); return; }
      el.style.setProperty('--rd', `${Math.min(n++, 6) * 80}ms`);
      el.classList.add('in');
      el.querySelectorAll<HTMLElement>('[data-count]').forEach(countUp);
    });
  }, { rootMargin: '0px 0px -8% 0px', threshold: 0.01 });
  revealEls.forEach((el) => io.observe(el));
  counters.forEach((el) => { if (!el.closest('.rv')) io.observe(el); });
} else revealEls.forEach((el) => el.classList.add('in'));

/* ── Cursor spotlight on cards ───────────────────────────── */
if (finePointer && !reduce) {
  document.addEventListener('pointermove', (e) => {
    const c = (e.target as Element).closest?.<HTMLElement>('.card, .spot');
    if (!c) return;
    const r = c.getBoundingClientRect();
    c.style.setProperty('--mx', `${e.clientX - r.left}px`);
    c.style.setProperty('--my', `${e.clientY - r.top}px`);
  }, { passive: true });
}

/* ── FAQ: animated open/close ────────────────────────────── */
if (!reduce) {
  document.querySelectorAll<HTMLDetailsElement>('.faq details').forEach((d) => {
    const s = d.querySelector('summary');
    const a = d.querySelector<HTMLElement>('.a');
    if (!s || !a) return;
    let anim: Animation | null = null;
    s.addEventListener('click', (e) => {
      e.preventDefault();
      anim?.cancel();
      if (d.open && !d.classList.contains('shut')) {
        d.classList.add('shut');
        anim = a.animate([{ height: `${a.offsetHeight}px`, opacity: 1 }, { height: '0px', paddingBottom: '0px', opacity: 0 }], { duration: 300, easing: 'cubic-bezier(.65,0,.35,1)' });
        anim.onfinish = () => { d.open = false; d.classList.remove('shut'); anim = null; };
      } else {
        d.classList.remove('shut');
        d.open = true;
        anim = a.animate([{ height: '0px', paddingBottom: '0px', opacity: 0 }, { height: `${a.offsetHeight}px`, opacity: 1 }], { duration: 440, easing: 'cubic-bezier(.16,1,.3,1)' });
        anim.onfinish = () => { anim = null; };
      }
    });
  });
}

/* ── Sticky feature story ────────────────────────────────── */
document.querySelectorAll<HTMLElement>('[data-story]').forEach((story) => {
  const steps = [...story.querySelectorAll<HTMLElement>('[data-step]')];
  const vis = [...story.querySelectorAll<HTMLElement>('[data-vis]')];
  const dots = [...story.querySelectorAll<HTMLElement>('[data-dot]')];
  const set = (i: number) => {
    steps.forEach((s, j) => s.classList.toggle('on', j === i));
    vis.forEach((v, j) => v.classList.toggle('on', j === i));
    dots.forEach((v, j) => v.classList.toggle('on', j === i));
    story.style.setProperty('--prog', String(steps.length > 1 ? i / (steps.length - 1) : 1));
  };
  set(0);
  const sio = new IntersectionObserver((es) => es.forEach((en) => {
    if (en.isIntersecting) set(steps.indexOf(en.target as HTMLElement));
  }), { rootMargin: '-45% 0px -45% 0px' });
  steps.forEach((s) => sio.observe(s));
});

/* ── Hero product animation: sequence, parallax, tilt ────── */
const hv = document.querySelector<HTMLElement>('[data-hv]');
if (hv && !reduce) {
  hv.dataset.play = '';
  const items = [...hv.querySelectorAll<HTMLElement>('[data-at]')];
  const last = Math.max(...items.map((i) => Number(i.dataset.at)));
  let step = -1;
  let timer = 0;
  const tick = () => {
    step = step >= last + 3 ? -1 : step + 1;
    items.forEach((el) => {
      const at = Number(el.dataset.at);
      const until = el.dataset.until ? Number(el.dataset.until) : Infinity;
      el.classList.toggle('on', step >= at && step < until);
    });
    timer = window.setTimeout(tick, step === -1 ? 900 : step >= last ? 2200 : 1300);
  };
  new IntersectionObserver(([en]) => {
    if (en.isIntersecting) { if (!timer) tick(); }
    else { clearTimeout(timer); timer = 0; }
  }).observe(hv);
  const layers = [...hv.querySelectorAll<HTMLElement>('[data-depth]')];
  const para = () => {
    const r = hv.getBoundingClientRect();
    if (r.bottom < 0 || r.top > innerHeight) return;
    const p = (innerHeight - r.top) / (innerHeight + r.height) - 0.5;
    layers.forEach((l) => l.style.setProperty("--py", `${(p * Number(l.dataset.depth) * (innerWidth < 820 ? -12 : -40)).toFixed(1)}px`));
  };
  onScroll.push(para);
  para();
  if (finePointer) {
    const stage = hv.querySelector<HTMLElement>('[data-tilt]');
    hv.addEventListener('pointermove', (e) => {
      if (!stage) return;
      const r = hv.getBoundingClientRect();
      const x = (e.clientX - r.left) / r.width - 0.5;
      const y = (e.clientY - r.top) / r.height - 0.5;
      stage.style.setProperty('--rx', `${(-y * 3).toFixed(2)}deg`);
      stage.style.setProperty('--ry', `${(x * 4).toFixed(2)}deg`);
    });
    hv.addEventListener('pointerleave', () => { stage?.style.setProperty('--rx', '0deg'); stage?.style.setProperty('--ry', '0deg'); });
  }
}

/* ── Long-form: reading progress + active TOC entry ──────── */
const prose = document.querySelector<HTMLElement>('[data-progress]');
if (prose) {
  const bar = document.createElement('div');
  bar.className = 'progress';
  bar.setAttribute('aria-hidden', 'true');
  document.body.appendChild(bar);
  const prog = () => {
    const r = prose.getBoundingClientRect();
    const p = Math.min(1, Math.max(0, (innerHeight * 0.35 - r.top) / Math.max(1, r.height - innerHeight * 0.5)));
    bar.style.setProperty('--p', p.toFixed(4));
  };
  onScroll.push(prog);
  prog();
  const links = new Map<string, HTMLElement>();
  document.querySelectorAll<HTMLAnchorElement>('.toc a[href^="#"]').forEach((a) => links.set(decodeURIComponent(a.hash.slice(1)), a));
  if (links.size) {
    const heads = [...prose.querySelectorAll<HTMLElement>('h2[id]')].filter((h) => links.has(h.id));
    const tio = new IntersectionObserver((es) => es.forEach((en) => {
      if (!en.isIntersecting) return;
      links.forEach((a) => a.classList.remove('on'));
      links.get((en.target as HTMLElement).id)?.classList.add('on');
    }), { rootMargin: '0px 0px -70% 0px' });
    heads.forEach((h) => tio.observe(h));
  }
  // Tables that scroll sideways on small screens must be reachable by keyboard.
  prose.querySelectorAll<HTMLElement>('table').forEach((t) => {
    if (t.scrollWidth > t.clientWidth + 1) t.tabIndex = 0;
  });
}

export {};
