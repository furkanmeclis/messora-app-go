'use client';

import { useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
    ArrowLeft,
    ArrowRight,
    Building2,
    CalendarClock,
    Check,
    Copy,
    Languages,
    Mail,
    MessageCircle,
    Plus,
    ShieldCheck,
    Sparkles,
    Trash2,
    Users,
} from 'lucide-react';
import { toast } from 'sonner';
import { BrandMark } from '@/components/brand';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { notify } from '@/components/notify';
import { useT } from '@/i18n/client';
import type { Locale } from '@/i18n/config';
import type {
    RoleKey,
    SetupStep,
    Settings,
    SmtpSettings,
    WhatsAppAccount,
} from '@/lib/types';
import { cn } from '@/lib/utils';
import {
    completeSetupStepAction,
    saveSettingsAction,
} from '../../(panel)/settings/actions';
import { LogoUploader } from '../../(panel)/settings/logo-uploader';
import { SmtpForm } from '../../(panel)/settings/email/smtp-form';
import {
    DateFormatField,
    LocaleField,
    MfaRolesField,
    TimeFormatField,
    TimezoneField,
} from '../../(panel)/settings/fields';
import { LockedBadge } from '../../(panel)/settings/locked';
import { Row } from '../../(panel)/settings/section';
import {
    connectAccountAction,
    inviteTeamAction,
    setOwnLocaleAction,
    type SentInvitation,
} from './actions';
import { SystemUserGuide } from '../../(panel)/channels/token-health';

/** The wizard's backdrop, drawn with the panel's colours (light and dark). */
const BACKDROP = [
    'radial-gradient(60% 50% at 15% 20%, color-mix(in oklch, var(--primary) 55%, transparent), transparent 70%)',
    'radial-gradient(50% 45% at 85% 25%, color-mix(in oklch, var(--chart-2) 45%, transparent), transparent 70%)',
    'radial-gradient(70% 60% at 60% 100%, color-mix(in oklch, var(--chart-4) 35%, transparent), transparent 70%)',
    'linear-gradient(160deg, color-mix(in oklch, var(--background) 85%, var(--primary)), var(--background))',
].join(',');

const STEPS: { key: SetupStep; icon: typeof Building2 }[] = [
    { key: 'organization', icon: Building2 },
    { key: 'language', icon: Languages },
    { key: 'region', icon: CalendarClock },
    { key: 'email', icon: Mail },
    { key: 'whatsapp', icon: MessageCircle },
    { key: 'security', icon: ShieldCheck },
    { key: 'team', icon: Users },
];

export function Wizard({
    settings: initial,
    smtp,
    accounts: initialAccounts,
    ownerEmail,
    customSmtp = true,
}: {
    settings: Settings;
    smtp: SmtpSettings;
    accounts: WhatsAppAccount[];
    ownerEmail: string;
    /** The plan includes the company's own SMTP server (Enterprise). */
    customSmtp?: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const [settings, setSettings] = useState(initial);
    const [done, setDone] = useState<SetupStep[]>(initial.setup.steps);
    // -1 = welcome screen, STEPS.length = finished.
    const [index, setIndex] = useState(() => {
        if (initial.setup.steps.length === 0) return -1;
        const next = STEPS.findIndex(
            (s) => !initial.setup.steps.includes(s.key),
        );
        return next === -1 ? STEPS.length : next;
    });
    const [busy, start] = useTransition();
    const [direction, setDirection] = useState<1 | -1>(1);
    // The two-factor policy is saved when the wizard finishes: requiring it
    // for the owner earlier would lock the rest of the wizard until the
    // owner enrols.
    const [mfaRoles, setMfaRoles] = useState<RoleKey[] | null>(null);

    const go = (i: number) => {
        setDirection(i > index ? 1 : -1);
        setIndex(i);
    };

    /** Runs save (if any), records the step and moves on. */
    const advance = (save?: () => Promise<boolean>) =>
        start(async () => {
            if (save && !(await save())) return;
            const step = STEPS[index]!.key;
            const r = await completeSetupStepAction(step, done);
            if (r.ok) {
                setDone(r.data.setup.steps);
                setSettings(r.data);
            }
            go(index + 1);
        });

    const finish = () =>
        start(async () => {
            const r = await completeSetupStepAction(
                STEPS[STEPS.length - 1]!.key,
                done,
                true,
                mfaRoles && !settings.locked.includes('mfa_required_roles')
                    ? { mfa_required_roles: mfaRoles }
                    : {},
            );
            if (notify(r)) {
                // A full load: when the policy
                // now requires two-factor the server redirects straight to
                // enrolment (a client-side chain of redirects stalls).
                // eslint-disable-next-line @next/next/no-location-assign-relative-destination -- deliberate full load (see above)
                window.location.assign('/');
            }
        });

    const patch = async (p: Parameters<typeof saveSettingsAction>[0]) => {
        const r = await saveSettingsAction(p, 'common.saved');
        if (!r.ok) {
            toast.error(r.message);
            return false;
        }
        setSettings(r.data);
        return true;
    };

    const step = STEPS[index];
    const locked = (f: string) => settings.locked.includes(f);

    return (
        <div
            className="relative flex min-h-svh flex-col"
            style={{ background: BACKDROP }}
        >
            <header className="flex items-center justify-between px-6 py-4">
                <div className="flex items-center gap-2.5">
                    <BrandMark className="size-8" />
                    <span className="font-semibold tracking-tight">
                        {settings.organization_name}
                    </span>
                </div>
                {index < STEPS.length && (
                    <Button asChild variant="ghost" size="sm">
                        <Link href="/">{t('wizard.exit')}</Link>
                    </Button>
                )}
            </header>

            <main className="flex flex-1 items-start justify-center px-4 pt-4 pb-16 sm:pt-10">
                <div className="grid w-full max-w-5xl gap-6 lg:grid-cols-[15rem_minmax(0,1fr)]">
                    {index >= 0 && index < STEPS.length && (
                        <nav
                            aria-label={t('wizard.stepOf', {
                                current: index + 1,
                                total: STEPS.length,
                            })}
                            className="hidden lg:block"
                        >
                            <ol className="space-y-1 rounded-2xl border bg-card/70 p-2 shadow-sm backdrop-blur-xl">
                                {STEPS.map((s, i) => {
                                    const isDone = done.includes(s.key);
                                    const current = i === index;
                                    return (
                                        <li key={s.key}>
                                            <button
                                                type="button"
                                                onClick={() => go(i)}
                                                aria-current={
                                                    current ? 'step' : undefined
                                                }
                                                className={cn(
                                                    'flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm transition-colors hover:bg-accent/60',
                                                    current &&
                                                        'bg-accent font-medium text-accent-foreground',
                                                )}
                                            >
                                                <span
                                                    className={cn(
                                                        'flex size-7 shrink-0 items-center justify-center rounded-lg border text-muted-foreground transition-colors',
                                                        isDone &&
                                                            'border-transparent bg-success/15 text-success',
                                                        current &&
                                                            'border-transparent bg-primary text-primary-foreground',
                                                    )}
                                                >
                                                    {isDone && !current ? (
                                                        <Check className="size-3.5" />
                                                    ) : (
                                                        <s.icon className="size-3.5" />
                                                    )}
                                                </span>
                                                {t(`wizard.steps.${s.key}`)}
                                            </button>
                                        </li>
                                    );
                                })}
                            </ol>
                        </nav>
                    )}

                    <section
                        key={index}
                        className={cn(
                            'animate-in rounded-3xl border bg-card/90 shadow-xl shadow-black/5 backdrop-blur-xl duration-300 fade-in-0',
                            direction === 1
                                ? 'slide-in-from-right-4'
                                : 'slide-in-from-left-4',
                            (index < 0 || index >= STEPS.length) &&
                                'mx-auto w-full max-w-2xl lg:col-span-2',
                        )}
                    >
                        {index < 0 && (
                            <div className="space-y-6 p-8 text-center sm:p-12">
                                <span className="mx-auto flex size-14 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-lg shadow-primary/30">
                                    <Sparkles className="size-7" />
                                </span>
                                <div className="space-y-2">
                                    <h1 className="text-3xl font-semibold tracking-tight">
                                        {t('wizard.welcome')}
                                    </h1>
                                    <p className="mx-auto max-w-md text-muted-foreground">
                                        {t('wizard.welcomeBody')}
                                    </p>
                                </div>
                                <div className="flex flex-wrap justify-center gap-2 text-xs text-muted-foreground">
                                    {STEPS.map((s) => (
                                        <span
                                            key={s.key}
                                            className="flex items-center gap-1 rounded-full border bg-background/60 px-2.5 py-1"
                                        >
                                            <s.icon className="size-3" />
                                            {t(`wizard.steps.${s.key}`)}
                                        </span>
                                    ))}
                                </div>
                                <Button size="lg" onClick={() => go(0)}>
                                    {t('wizard.start')} <ArrowRight />
                                </Button>
                            </div>
                        )}

                        {step && (
                            <StepFrame
                                title={t(`wizard.${step.key}.title`)}
                                body={t(`wizard.${step.key}.body`)}
                                counter={t('wizard.stepOf', {
                                    current: index + 1,
                                    total: STEPS.length,
                                })}
                            >
                                {step.key === 'organization' && (
                                    <OrganizationStep
                                        settings={settings}
                                        locked={locked('organization_name')}
                                        busy={busy}
                                        onBack={() => go(index - 1)}
                                        onNext={(name) =>
                                            advance(() =>
                                                name ===
                                                    settings.organization_name ||
                                                locked('organization_name')
                                                    ? Promise.resolve(true)
                                                    : patch({
                                                          organization_name:
                                                              name,
                                                      }),
                                            )
                                        }
                                    />
                                )}
                                {step.key === 'language' && (
                                    <LanguageStep
                                        value={
                                            settings.default_locale as Locale
                                        }
                                        locked={locked('default_locale')}
                                        busy={busy}
                                        onBack={() => go(index - 1)}
                                        onNext={(l) =>
                                            advance(async () => {
                                                if (
                                                    !locked('default_locale') &&
                                                    l !==
                                                        settings.default_locale
                                                ) {
                                                    if (
                                                        !(await patch({
                                                            default_locale: l,
                                                        }))
                                                    )
                                                        return false;
                                                }
                                                await setOwnLocaleAction(l);
                                                router.refresh();
                                                return true;
                                            })
                                        }
                                    />
                                )}
                                {step.key === 'region' && (
                                    <RegionStep
                                        settings={settings}
                                        busy={busy}
                                        onBack={() => go(index - 1)}
                                        onNext={(v) =>
                                            advance(() => {
                                                const p = Object.fromEntries(
                                                    Object.entries(v).filter(
                                                        ([k, val]) =>
                                                            !locked(k) &&
                                                            val !==
                                                                settings[
                                                                    k as keyof typeof v
                                                                ],
                                                    ),
                                                );
                                                return Object.keys(p).length
                                                    ? patch(p)
                                                    : Promise.resolve(true);
                                            })
                                        }
                                    />
                                )}
                                {step.key === 'email' && (
                                    <>
                                        {customSmtp ? (
                                            <SmtpForm
                                                smtp={smtp}
                                                readOnly={false}
                                                defaultTestTo={ownerEmail}
                                                bare
                                            />
                                        ) : (
                                            <p className="rounded-xl border bg-muted/30 p-4 text-sm text-muted-foreground">
                                                {t('plans.email.senderHint')}
                                            </p>
                                        )}
                                        <StepNav
                                            busy={busy}
                                            onBack={() => go(index - 1)}
                                            onNext={() => advance()}
                                            nextLabel={t('common.continue')}
                                        />
                                    </>
                                )}
                                {step.key === 'whatsapp' && (
                                    <WhatsAppStep
                                        accounts={initialAccounts}
                                        busy={busy}
                                        onBack={() => go(index - 1)}
                                        onNext={() => advance()}
                                    />
                                )}
                                {step.key === 'security' && (
                                    <SecurityStep
                                        value={
                                            mfaRoles ??
                                            settings.mfa_required_roles
                                        }
                                        locked={locked('mfa_required_roles')}
                                        busy={busy}
                                        onBack={() => go(index - 1)}
                                        onNext={(roles) => {
                                            setMfaRoles(roles);
                                            advance();
                                        }}
                                    />
                                )}
                                {step.key === 'team' && (
                                    <TeamStep
                                        busy={busy}
                                        onBack={() => go(index - 1)}
                                        onNext={() => advance()}
                                    />
                                )}
                            </StepFrame>
                        )}

                        {index >= STEPS.length && (
                            <div className="space-y-6 p-8 text-center sm:p-12">
                                <span className="mx-auto flex size-14 items-center justify-center rounded-2xl bg-success/15 text-success">
                                    <Check className="size-7" />
                                </span>
                                <div className="space-y-2">
                                    <h1 className="text-3xl font-semibold tracking-tight">
                                        {t('wizard.done.title')}
                                    </h1>
                                    <p className="text-muted-foreground">
                                        {t('wizard.done.body')}
                                    </p>
                                </div>
                                <ul className="mx-auto grid max-w-md gap-2 text-left text-sm">
                                    {(
                                        [
                                            ['/contacts/import', 'import'],
                                            ['/templates', 'template'],
                                            ['/automations/new', 'automation'],
                                        ] as const
                                    ).map(([href, k]) => (
                                        <li key={k}>
                                            <Link
                                                href={href}
                                                className="flex items-center justify-between rounded-xl border bg-background/60 px-4 py-3 transition-colors hover:bg-accent"
                                            >
                                                {t(`wizard.done.next.${k}`)}
                                                <ArrowRight className="size-4 text-muted-foreground" />
                                            </Link>
                                        </li>
                                    ))}
                                </ul>
                                <div className="flex justify-center gap-2">
                                    <Button
                                        variant="ghost"
                                        onClick={() => go(STEPS.length - 1)}
                                    >
                                        <ArrowLeft /> {t('common.back')}
                                    </Button>
                                    <Button
                                        size="lg"
                                        onClick={finish}
                                        disabled={busy}
                                    >
                                        {t('wizard.done.open')} <ArrowRight />
                                    </Button>
                                </div>
                            </div>
                        )}
                    </section>
                </div>
            </main>
        </div>
    );
}

function StepFrame({
    title,
    body,
    counter,
    children,
}: {
    title: string;
    body: string;
    counter: string;
    children: React.ReactNode;
}) {
    return (
        <div className="space-y-6 p-6 sm:p-8">
            <div className="space-y-1.5">
                <p className="text-xs font-medium tracking-wide text-primary uppercase">
                    {counter}
                </p>
                <h1 className="text-2xl font-semibold tracking-tight">
                    {title}
                </h1>
                <p className="text-sm text-muted-foreground">{body}</p>
            </div>
            <div className="space-y-6">{children}</div>
        </div>
    );
}

function StepNav({
    busy,
    onBack,
    onNext,
    nextLabel,
    canSkip = true,
    onSkip,
}: {
    busy: boolean;
    onBack: () => void;
    onNext: () => void;
    nextLabel?: string;
    canSkip?: boolean;
    onSkip?: () => void;
}) {
    const t = useT();
    return (
        <div className="flex items-center gap-2 border-t pt-5">
            <Button
                type="button"
                variant="ghost"
                onClick={onBack}
                disabled={busy}
            >
                <ArrowLeft /> {t('common.back')}
            </Button>
            <div className="ml-auto flex gap-2">
                {canSkip && onSkip && (
                    <Button
                        type="button"
                        variant="ghost"
                        onClick={onSkip}
                        disabled={busy}
                    >
                        {t('common.skip')}
                    </Button>
                )}
                <Button type="button" onClick={onNext} disabled={busy}>
                    {nextLabel ?? t('wizard.saveAndContinue')} <ArrowRight />
                </Button>
            </div>
        </div>
    );
}

type StepProps = { busy: boolean; onBack: () => void };

function OrganizationStep({
    settings,
    locked,
    busy,
    onBack,
    onNext,
}: StepProps & {
    settings: Settings;
    locked: boolean;
    onNext: (name: string) => void;
}) {
    const t = useT();
    const [name, setName] = useState(settings.organization_name);
    return (
        <>
            <Row
                label={t('settings.general.organizationName')}
                htmlFor="org-name"
                badge={
                    locked ? <LockedBadge field="organization_name" /> : null
                }
            >
                <Input
                    id="org-name"
                    value={name}
                    maxLength={120}
                    disabled={locked}
                    autoFocus
                    onChange={(e) => setName(e.target.value)}
                />
            </Row>
            <Row
                label={t('settings.logo.title')}
                hint={t('settings.logo.hint')}
            >
                <LogoUploader logo={settings.logo} readOnly={false} />
            </Row>
            <StepNav
                busy={busy}
                onBack={onBack}
                onNext={() => onNext(name.trim() || settings.organization_name)}
            />
        </>
    );
}

function LanguageStep({
    value,
    locked,
    busy,
    onBack,
    onNext,
}: StepProps & {
    value: Locale;
    locked: boolean;
    onNext: (l: Locale) => void;
}) {
    const [locale, setLocale] = useState<Locale>(value);
    return (
        <>
            {locked && <LockedBadge field="default_locale" />}
            <LocaleField value={locale} onChange={setLocale} />
            <StepNav
                busy={busy}
                onBack={onBack}
                onNext={() => onNext(locale)}
            />
        </>
    );
}

function RegionStep({
    settings,
    busy,
    onBack,
    onNext,
}: StepProps & {
    settings: Settings;
    onNext: (
        v: Pick<Settings, 'default_timezone' | 'date_format' | 'time_format'>,
    ) => void;
}) {
    const t = useT();
    const browserZone =
        typeof Intl !== 'undefined'
            ? Intl.DateTimeFormat().resolvedOptions().timeZone
            : 'UTC';
    const [v, setV] = useState({
        default_timezone:
            settings.default_timezone === 'UTC' &&
            !settings.locked.includes('default_timezone')
                ? browserZone
                : settings.default_timezone,
        date_format: settings.date_format,
        time_format: settings.time_format,
    });
    const l = settings.default_locale as Locale;
    const lockedBadge = (f: string) =>
        settings.locked.includes(f) ? <LockedBadge field={f} /> : null;
    return (
        <>
            <Row
                label={t('settings.general.defaultTimezone')}
                badge={lockedBadge('default_timezone')}
            >
                <TimezoneField
                    value={v.default_timezone}
                    onChange={(z) => setV({ ...v, default_timezone: z })}
                    disabled={settings.locked.includes('default_timezone')}
                />
            </Row>
            <Row
                label={t('settings.general.dateFormat')}
                badge={lockedBadge('date_format')}
            >
                <DateFormatField
                    value={v.date_format}
                    onChange={(f) => setV({ ...v, date_format: f })}
                    locale={l}
                    disabled={settings.locked.includes('date_format')}
                />
            </Row>
            <Row
                label={t('settings.general.timeFormat')}
                badge={lockedBadge('time_format')}
            >
                <TimeFormatField
                    value={v.time_format}
                    onChange={(f) => setV({ ...v, time_format: f })}
                    locale={l}
                    disabled={settings.locked.includes('time_format')}
                />
            </Row>
            <StepNav busy={busy} onBack={onBack} onNext={() => onNext(v)} />
        </>
    );
}

function WhatsAppStep({
    accounts: initial,
    busy,
    onBack,
    onNext,
}: StepProps & { accounts: WhatsAppAccount[]; onNext: () => void }) {
    const t = useT();
    const [accounts, setAccounts] = useState(initial);
    const [connecting, start] = useTransition();
    const [v, setV] = useState({
        name: '',
        waba_id: '',
        app_id: '',
        access_token: '',
        app_secret: '',
    });
    const connect = () =>
        start(async () => {
            const r = await connectAccountAction({
                name: v.name.trim(),
                waba_id: v.waba_id.trim(),
                access_token: v.access_token.trim(),
                app_id: v.app_id.trim(),
                app_secret: v.app_secret.trim(),
            });
            if (notify(r)) {
                setAccounts((a) => [...a, r.data]);
                setV({
                    name: '',
                    waba_id: '',
                    app_id: '',
                    access_token: '',
                    app_secret: '',
                });
            }
        });
    const field = (
        k: keyof typeof v,
        label: string,
        props: React.ComponentProps<typeof Input> = {},
    ) => (
        <Row label={label} htmlFor={`wa-${k}`}>
            <Input
                id={`wa-${k}`}
                value={v[k]}
                autoComplete="off"
                onChange={(e) => setV({ ...v, [k]: e.target.value })}
                {...props}
            />
        </Row>
    );
    return (
        <>
            {accounts.length > 0 && (
                <ul className="space-y-2">
                    {accounts.map((a) => (
                        <li
                            key={a.id}
                            className="flex items-center gap-3 rounded-xl border border-success/30 bg-success/10 px-4 py-3 text-sm"
                        >
                            <Check className="size-4 text-success" />
                            {t('wizard.whatsapp.connected', { name: a.name })}
                            <span className="ml-auto font-mono text-xs text-muted-foreground">
                                {a.waba_id}
                            </span>
                        </li>
                    ))}
                </ul>
            )}
            <div className="space-y-4 rounded-2xl border bg-muted/20 p-4">
                <SystemUserGuide />
                <div className="grid gap-4 sm:grid-cols-2">
                    {field('name', t('wizard.whatsapp.name'), {
                        placeholder: t('wizard.whatsapp.namePlaceholder'),
                    })}
                    {field('waba_id', t('wizard.whatsapp.wabaId'), {
                        inputMode: 'numeric',
                    })}
                    {field('app_id', t('wizard.whatsapp.appId'), {
                        inputMode: 'numeric',
                    })}
                    {field('app_secret', t('wizard.whatsapp.appSecret'), {
                        type: 'password',
                    })}
                </div>
                {field('access_token', t('wizard.whatsapp.accessToken'), {
                    type: 'password',
                })}
                <div className="flex items-center justify-between gap-2">
                    <p className="text-xs text-muted-foreground">
                        {t('wizard.whatsapp.later')}
                    </p>
                    <Button
                        type="button"
                        variant="secondary"
                        onClick={connect}
                        disabled={
                            connecting ||
                            !v.name ||
                            !v.waba_id ||
                            !v.app_id ||
                            v.app_secret.length < 8 ||
                            v.access_token.length < 20
                        }
                    >
                        {connecting
                            ? t('wizard.whatsapp.connecting')
                            : t('wizard.whatsapp.connect')}
                    </Button>
                </div>
            </div>
            <StepNav
                busy={busy}
                onBack={onBack}
                onNext={onNext}
                nextLabel={t('common.continue')}
            />
        </>
    );
}

function SecurityStep({
    value,
    locked,
    busy,
    onBack,
    onNext,
}: StepProps & {
    value: RoleKey[];
    locked: boolean;
    onNext: (r: RoleKey[]) => void;
}) {
    const t = useT();
    const [roles, setRoles] = useState(value);
    return (
        <>
            {locked && <LockedBadge field="mfa_required_roles" />}
            <MfaRolesField
                value={roles}
                onChange={setRoles}
                disabled={locked}
            />
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
                <ShieldCheck className="size-4 text-primary" />
                {t('wizard.security.recommendation')}
            </p>
            <StepNav busy={busy} onBack={onBack} onNext={() => onNext(roles)} />
        </>
    );
}

const INVITE_ROLES: RoleKey[] = ['admin', 'manager', 'agent'];

function TeamStep({
    busy,
    onBack,
    onNext,
}: StepProps & { onNext: () => void }) {
    const t = useT();
    const [rows, setRows] = useState<{ email: string; role: RoleKey }[]>([
        { email: '', role: 'agent' },
    ]);
    const [results, setResults] = useState<SentInvitation[]>([]);
    const [sending, start] = useTransition();
    const valid = rows.filter((r) =>
        /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(r.email.trim()),
    );
    const send = () =>
        start(async () => {
            const r = await inviteTeamAction(
                valid.map((x) => ({ email: x.email.trim(), role: x.role })),
            );
            if (!r.ok) return;
            setResults(r.data);
            const sent = r.data.filter((x) => x.ok).length;
            if (sent) toast.success(t('wizard.team.invited', { count: sent }));
            setRows(
                rows
                    .filter(
                        (row) =>
                            !r.data.some(
                                (x) => x.ok && x.email === row.email.trim(),
                            ),
                    )
                    .concat(
                        r.data.every((x) => x.ok)
                            ? [{ email: '', role: 'agent' }]
                            : [],
                    ),
            );
        });
    const links = results.filter((r) => r.ok && !r.emailSent && r.acceptUrl);
    return (
        <>
            <div className="space-y-2">
                {rows.map((row, i) => (
                    <div key={i} className="flex gap-2">
                        <Input
                            type="email"
                            aria-label={t('wizard.team.email')}
                            placeholder={t('wizard.team.email')}
                            value={row.email}
                            onChange={(e) =>
                                setRows(
                                    rows.map((r, j) =>
                                        j === i
                                            ? { ...r, email: e.target.value }
                                            : r,
                                    ),
                                )
                            }
                        />
                        <Select
                            value={row.role}
                            onValueChange={(role) =>
                                setRows(
                                    rows.map((r, j) =>
                                        j === i
                                            ? { ...r, role: role as RoleKey }
                                            : r,
                                    ),
                                )
                            }
                        >
                            <SelectTrigger
                                className="w-40"
                                aria-label={t('wizard.team.role')}
                            >
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {INVITE_ROLES.map((r) => (
                                    <SelectItem key={r} value={r}>
                                        {t(`settings.security.roles.${r}`)}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                        <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            aria-label={t('common.remove')}
                            disabled={rows.length === 1}
                            onClick={() =>
                                setRows(rows.filter((_, j) => j !== i))
                            }
                        >
                            <Trash2 />
                        </Button>
                    </div>
                ))}
                <div className="flex justify-between gap-2">
                    <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() =>
                            setRows([...rows, { email: '', role: 'agent' }])
                        }
                    >
                        <Plus /> {t('wizard.team.addAnother')}
                    </Button>
                    <Button
                        type="button"
                        variant="secondary"
                        onClick={send}
                        disabled={sending || valid.length === 0}
                    >
                        {t('wizard.team.invite')}
                    </Button>
                </div>
            </div>
            {results.length > 0 && (
                <ul className="space-y-2">
                    {links.length > 0 && (
                        <p className="text-sm text-muted-foreground">
                            {t('wizard.team.linkHint')}
                        </p>
                    )}
                    {results.map((r) => (
                        <li
                            key={r.email}
                            className={cn(
                                'flex items-center gap-3 rounded-xl border px-3 py-2 text-sm',
                                r.ok
                                    ? 'border-success/30 bg-success/5'
                                    : 'border-destructive/30 bg-destructive/5',
                            )}
                        >
                            <span className="min-w-0 flex-1 truncate font-medium">
                                {r.email}
                            </span>
                            {r.ok && r.emailSent && (
                                <span className="text-xs text-success">
                                    {t('wizard.team.sentByEmail')}
                                </span>
                            )}
                            {!r.ok && (
                                <span className="text-xs text-destructive">
                                    {t('wizard.team.failed')}: {r.message}
                                </span>
                            )}
                            {r.ok && !r.emailSent && r.acceptUrl && (
                                <Button
                                    type="button"
                                    size="sm"
                                    variant="outline"
                                    onClick={() =>
                                        navigator.clipboard
                                            .writeText(r.acceptUrl!)
                                            .then(() =>
                                                toast.success(
                                                    t('common.copied'),
                                                ),
                                            )
                                    }
                                >
                                    <Copy /> {t('wizard.team.copyLink')}
                                </Button>
                            )}
                        </li>
                    ))}
                </ul>
            )}
            <StepNav
                busy={busy}
                onBack={onBack}
                onNext={onNext}
                nextLabel={t('common.continue')}
            />
        </>
    );
}
