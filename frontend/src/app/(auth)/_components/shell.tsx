import Link from 'next/link';
import { Check, Sparkles } from 'lucide-react';
import { MessoraLogo } from '@/components/messora-logo';
import { localeNames, type Locale } from '@/i18n/config';
import { getLocale, getT } from '@/i18n/server';
import { LEGAL_LINKS } from '@/lib/auth/contracts';
import { cn } from '@/lib/utils';
import { setLocaleAction } from '../actions';

/** Turkish first: most workspaces are in Türkiye. */
const LOCALE_ORDER: readonly Locale[] = ['tr', 'en'];

/**
 * The frame every sign-in screen shares. Wide screens get a Night brand
 * panel on the left and the form on Fog on the right; phones get a compact
 * header with the logo and the form only.
 */
export async function AuthShell({ children }: { children: React.ReactNode }) {
    return (
        <div className="min-h-svh font-ui lg:grid lg:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
            <BrandPanel />
            <div className="flex min-h-svh flex-col bg-fog dark:bg-background">
                <header className="flex items-center px-4 pt-5 sm:px-8 lg:hidden">
                    <Link
                        href="/login"
                        className="rounded-lg focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
                    >
                        <MessoraLogo size="sm" />
                    </Link>
                </header>
                <main
                    id="main"
                    className="flex flex-1 items-start justify-center px-4 py-8 sm:items-center sm:px-8 sm:py-12"
                >
                    <div className="w-full max-w-[440px]">{children}</div>
                </main>
                <AuthFooter />
            </div>
        </div>
    );
}

async function BrandPanel() {
    const t = await getT();
    const values = [
        t('auth.shell.value1'),
        t('auth.shell.value2'),
        t('auth.shell.value3'),
    ];
    return (
        <aside className="relative hidden overflow-hidden bg-night text-fog lg:sticky lg:top-0 lg:flex lg:h-svh lg:flex-col lg:justify-between lg:gap-10 lg:p-12 xl:p-16">
            {/* Soft brand light behind the content; decorative only. */}
            <div
                aria-hidden
                className="pointer-events-none absolute inset-0 bg-[radial-gradient(60%_50%_at_85%_10%,rgb(59_76_202/0.35),transparent_70%),radial-gradient(40%_40%_at_10%_95%,rgb(45_212_167/0.14),transparent_70%)]"
            />
            <div className="relative">
                <MessoraLogo tone="dark" size="lg" />
            </div>
            <div className="relative flex max-w-xl flex-col gap-8">
                <div className="flex flex-col gap-4">
                    <p className="font-display text-[40px] leading-[1.05] font-extrabold tracking-[-0.03em] text-balance xl:text-[52px]">
                        {t('auth.shell.tagline')}
                    </p>
                    <p className="max-w-lg text-[17px] leading-relaxed text-night-muted">
                        {t('auth.shell.intro')}
                    </p>
                </div>
                <Conversation
                    label={t('auth.shell.conversation')}
                    customer={t('auth.shell.chatCustomer')}
                    reply={t('auth.shell.chatReply')}
                    chip={t('auth.shell.aiChip')}
                />
                <ul className="flex flex-col gap-3 [@media(max-height:860px)]:hidden">
                    {values.map((v) => (
                        <li
                            key={v}
                            className="flex items-start gap-3 text-[15px] text-night-muted"
                        >
                            <span className="mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full bg-mint text-night">
                                <Check
                                    className="size-3.5"
                                    strokeWidth={3}
                                    aria-hidden
                                />
                            </span>
                            {v}
                        </li>
                    ))}
                </ul>
            </div>
            <p className="relative text-sm text-night-subtle">
                © {new Date().getFullYear()} Messora
            </p>
        </aside>
    );
}

/**
 * A short WhatsApp-style exchange that plays once on load: the customer's
 * question, the AI typing, its reply and the "AI replied" chip. With
 * reduced motion everything is simply there.
 */
function Conversation({
    label,
    customer,
    reply,
    chip,
}: {
    label: string;
    customer: string;
    reply: string;
    chip: string;
}) {
    return (
        <figure
            aria-label={label}
            className="flex max-w-md flex-col gap-3 rounded-[20px] border border-night-line bg-night-raised/90 p-5 shadow-2xl shadow-black/30 [@media(max-height:680px)]:hidden"
        >
            <p className="max-w-[85%] self-start rounded-2xl rounded-bl-[4px] bg-night-line px-3.5 py-2.5 text-[15px] leading-snug motion-safe:animate-bubble-in motion-safe:[animation-delay:300ms]">
                {customer}
            </p>
            <div className="grid justify-items-end">
                <span
                    aria-hidden
                    className="col-start-1 row-start-1 flex h-10 items-center gap-1 self-end rounded-2xl rounded-br-[4px] bg-indigo-brand/60 px-3.5 opacity-0 motion-safe:animate-typing-window motion-safe:[animation-delay:900ms] motion-reduce:hidden"
                >
                    {[0, 150, 300].map((d) => (
                        <span
                            key={d}
                            className="size-1.5 rounded-full bg-fog motion-safe:animate-typing-dot"
                            style={{ animationDelay: `${d}ms` }}
                        />
                    ))}
                </span>
                <p className="col-start-1 row-start-1 max-w-[85%] rounded-2xl rounded-br-[4px] bg-indigo-brand px-3.5 py-2.5 text-[15px] leading-snug text-white motion-safe:animate-bubble-in motion-safe:[animation-delay:2000ms]">
                    {reply}
                </p>
            </div>
            <span className="inline-flex items-center gap-1.5 self-end rounded-full bg-mint px-2.5 py-1 text-xs font-semibold text-night motion-safe:animate-bubble-in motion-safe:[animation-delay:2400ms]">
                <Sparkles className="size-3.5" aria-hidden />
                {chip}
            </span>
        </figure>
    );
}

async function AuthFooter() {
    const [t, locale] = await Promise.all([getT(), getLocale()]);
    const link =
        'inline-flex min-h-11 items-center rounded-md px-2 underline-offset-4 hover:text-foreground hover:underline focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none';
    return (
        <footer className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 pb-4 text-sm text-muted-foreground sm:px-8 sm:pb-6">
            <form action={setLocaleAction}>
                <div
                    role="group"
                    aria-label={t('auth.shell.language')}
                    className="inline-flex rounded-full bg-foreground/5 p-1 dark:bg-white/5"
                >
                    {LOCALE_ORDER.map((l) => (
                        <button
                            key={l}
                            type="submit"
                            name="locale"
                            value={l}
                            lang={l}
                            aria-pressed={l === locale}
                            className={cn(
                                'min-h-9 min-w-11 rounded-full px-3 text-[13px] font-semibold transition-colors focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none',
                                l === locale
                                    ? 'bg-card text-foreground shadow-sm'
                                    : 'text-muted-foreground hover:text-foreground',
                            )}
                        >
                            <span aria-hidden>{l.toUpperCase()}</span>
                            <span className="sr-only">{localeNames[l]}</span>
                        </button>
                    ))}
                </div>
            </form>
            <nav
                aria-label={t('auth.shell.legal')}
                className="-mx-2 flex items-center"
            >
                <a href={LEGAL_LINKS.privacy} className={link}>
                    {t('auth.shell.privacy')}
                </a>
                <span aria-hidden>·</span>
                <a href={LEGAL_LINKS.terms} className={link}>
                    {t('auth.shell.terms')}
                </a>
            </nav>
        </footer>
    );
}

type Tone = 'default' | 'success' | 'warning' | 'danger' | 'info';

const toneClass: Record<Tone, string> = {
    default: 'bg-primary/10 text-primary',
    info: 'bg-primary/10 text-primary',
    success: 'bg-mint/20 text-emerald-700 dark:text-mint',
    warning: 'bg-coral/15 text-[#B4441F] dark:text-coral',
    danger: 'bg-destructive/10 text-destructive',
};

/** The card each screen puts its form or message in. */
export function AuthCard({
    title,
    description,
    icon: Icon,
    tone = 'default',
    logoUrl,
    children,
    footer,
}: {
    title: string;
    description?: React.ReactNode;
    icon?: React.ComponentType<{ className?: string }>;
    tone?: Tone;
    /** A workspace logo shown above the title. */
    logoUrl?: string | null;
    children?: React.ReactNode;
    /** Shown under the card (e.g. "New to Messora? Create an account"). */
    footer?: React.ReactNode;
}) {
    return (
        <div className="flex flex-col gap-5">
            <section
                aria-labelledby="auth-title"
                className="rounded-2xl border border-black/5 bg-card p-6 shadow-[0_1px_2px_rgb(11_18_32/0.04),0_12px_32px_-12px_rgb(11_18_32/0.12)] sm:p-8 dark:border-white/8 dark:shadow-none"
            >
                {logoUrl && (
                    // eslint-disable-next-line @next/next/no-img-element -- same-origin logo of any size
                    <img
                        src={logoUrl}
                        alt=""
                        className="mb-5 h-10 w-auto max-w-[160px] object-contain"
                    />
                )}
                {Icon && !logoUrl && (
                    <span
                        className={cn(
                            'mb-5 flex size-12 items-center justify-center rounded-2xl',
                            toneClass[tone],
                        )}
                    >
                        <Icon className="size-6" />
                    </span>
                )}
                <h1
                    id="auth-title"
                    className="font-display text-[26px] leading-tight font-extrabold tracking-[-0.02em] text-balance"
                >
                    {title}
                </h1>
                {description && (
                    <div className="mt-2 text-[15px] leading-relaxed text-muted-foreground">
                        {description}
                    </div>
                )}
                {children && <div className="mt-6">{children}</div>}
            </section>
            {footer && (
                <div className="text-center text-sm text-muted-foreground">
                    {footer}
                </div>
            )}
        </div>
    );
}

/** A text link styled for the sign-in screens, with a 44 px target. */
export function AuthLink({
    href,
    children,
    className,
    prefetch,
}: {
    href: string;
    children: React.ReactNode;
    className?: string;
    /** false for routes with side effects (sign out). */
    prefetch?: boolean;
}) {
    return (
        <Link
            href={href}
            prefetch={prefetch}
            className={cn(
                'inline-flex min-h-11 items-center rounded-md font-medium text-primary underline-offset-4 hover:underline focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none',
                className,
            )}
        >
            {children}
        </Link>
    );
}
