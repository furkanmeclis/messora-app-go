'use client';

import { useCallback, useEffect, useRef, useState, useTransition } from 'react';
import {
    CheckCircle2,
    CircleAlert,
    Copy,
    Link2Off,
    Loader2,
    RotateCcw,
    ShieldCheck,
    Sparkles,
    XCircle,
} from 'lucide-react';
import { toast } from 'sonner';
import { useReauth } from '@/components/reauth';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
    AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { RadioCard, RadioCards } from '@/components/ui/radio-cards';
import { Spinner } from '@/components/ui/spinner';
import { useT } from '@/i18n/client';
import type { MessageKey } from '@/i18n/translate';
import {
    isMetaOrigin,
    isSignupPage,
    loginOptions,
    parseSignupMessage,
    SDK_URL,
    type SignupEvent,
} from '@/lib/embedded-signup';
import type {
    TechProvider,
    TechProviderOnboarding,
    WhatsAppAccount,
} from '@/lib/types';
import { cn } from '@/lib/utils';
import {
    cancelOnboardingAction,
    disconnectTechProviderAction,
    retryOnboardingAction,
    startOnboardingAction,
} from './actions';

// ── Meta's JavaScript SDK ──────────────────────────────────────────────────

interface FBLoginResponse {
    authResponse?: { code?: string } | null;
    status?: string;
}

interface FBStatic {
    init(o: Record<string, unknown>): void;
    login(
        cb: (r: FBLoginResponse) => void,
        o: ReturnType<typeof loginOptions>,
    ): void;
}

declare global {
    interface Window {
        FB?: FBStatic;
        fbAsyncInit?: () => void;
    }
}

let sdk: Promise<FBStatic> | null = null;

/** Loads Meta's SDK once and initialises it with Messora's app. */
function loadSDK(appId: string, version: string): Promise<FBStatic> {
    if (sdk) return sdk;
    sdk = new Promise<FBStatic>((resolve, reject) => {
        const ready = () => {
            const fb = window.FB;
            if (!fb) return reject(new Error('sdk'));
            fb.init({
                appId,
                autoLogAppEvents: true,
                xfbml: false,
                version,
            });
            resolve(fb);
        };
        if (window.FB) return ready();
        window.fbAsyncInit = ready;
        const s = document.createElement('script');
        s.src = SDK_URL;
        s.async = true;
        s.defer = true;
        s.crossOrigin = 'anonymous';
        s.onerror = () => reject(new Error('sdk'));
        document.body.appendChild(s);
        setTimeout(() => reject(new Error('sdk timeout')), 20_000);
    }).catch((e) => {
        sdk = null;
        throw e;
    });
    return sdk;
}

/**
 * The relaxed CSP of this page belongs to its own document: after a soft
 * navigation from another page the browser still enforces the strict one,
 * so the page loads itself once more.
 */
function useOwnDocument(active: boolean) {
    useEffect(() => {
        if (!active) return;
        const nav = performance.getEntriesByType('navigation')[0] as
            PerformanceNavigationTiming | undefined;
        const loaded = nav ? new URL(nav.name).pathname : '';
        if (nav && !isSignupPage(loaded)) window.location.reload();
    }, [active]);
}

// ── The card ───────────────────────────────────────────────────────────────

type Phase =
    | { kind: 'idle' }
    | { kind: 'loading' }
    | { kind: 'popup' }
    | { kind: 'running' }
    | { kind: 'done'; onboarding: TechProviderOnboarding }
    | { kind: 'failed'; onboarding: TechProviderOnboarding }
    | { kind: 'message'; tone: 'info' | 'error'; text: string };

const STEPS = ['exchange', 'claim', 'subscribe', 'register', 'finish'] as const;

const SCOPES = [
    'whatsapp_business_management',
    'whatsapp_business_messaging',
    'business_management',
] as const;

/** Settings → WhatsApp / Meta → "Connect with Meta" (Embedded Signup). */
export function TechProviderCard({
    config,
    accounts,
}: {
    config: TechProvider;
    accounts: WhatsAppAccount[];
}) {
    const t = useT();
    const { guard } = useReauth();
    const [phase, setPhase] = useState<Phase>({ kind: 'idle' });
    const [pinMode, setPinMode] = useState<'auto' | 'own'>('auto');
    const [pin, setPin] = useState('');
    const [, start] = useTransition();
    // What the popup said and what FB.login returned, joined when both are in.
    const session = useRef<SignupEvent | null>(null);
    const code = useRef<string | null>(null);
    const retryOf = useRef<string | null>(null);
    useOwnDocument(true);

    const pinValid = pinMode === 'auto' || /^[0-9]{6}$/.test(pin);

    const finished = useCallback(
        (o: TechProviderOnboarding) => {
            if (o.status === 'completed') {
                setPhase({ kind: 'done', onboarding: o });
                toast.success(t('channels.meta.tp.success', { name: o.name }));
            } else {
                setPhase({ kind: 'failed', onboarding: o });
            }
        },
        [t],
    );

    const submit = useCallback(() => {
        const s = session.current;
        const c = code.current;
        if (!s || !c || s.kind !== 'finish') return;
        session.current = null;
        code.current = null;
        setPhase({ kind: 'running' });
        start(async () => {
            const again = retryOf.current;
            retryOf.current = null;
            const r = await guard(() =>
                again
                    ? retryOnboardingAction(again, { code: c })
                    : startOnboardingAction({
                          code: c,
                          waba_id: s.wabaId,
                          phone_number_id: s.phoneNumberId,
                          business_id: s.businessId,
                          pin: pinMode === 'own' ? pin : undefined,
                      }),
            );
            if (!r.ok) {
                setPhase({ kind: 'message', tone: 'error', text: r.message });
                return;
            }
            finished(r.data);
        });
    }, [finished, guard, pin, pinMode]);

    // Meta's popup reports the outcome with a window message.
    useEffect(() => {
        const onMessage = (e: MessageEvent) => {
            if (!isMetaOrigin(e.origin)) return;
            const ev = parseSignupMessage(e.data);
            if (!ev) return;
            switch (ev.kind) {
                case 'finish':
                    session.current = ev;
                    submit();
                    break;
                case 'finish_app':
                    setPhase({
                        kind: 'message',
                        tone: 'error',
                        text: t('channels.meta.tp.coexistence'),
                    });
                    break;
                case 'cancel':
                    setPhase({
                        kind: 'message',
                        tone: 'info',
                        text: t('channels.meta.tp.cancelledByUser', {
                            step: ev.step || '—',
                        }),
                    });
                    break;
                case 'error':
                    setPhase({
                        kind: 'message',
                        tone: 'error',
                        text: t('channels.meta.tp.metaError', {
                            message: ev.message || ev.code || '—',
                        }),
                    });
                    break;
            }
        };
        window.addEventListener('message', onMessage);
        return () => window.removeEventListener('message', onMessage);
    }, [submit, t]);

    /** Opens Meta's popup; retrying onboarding id when given. */
    const connect = (retry?: string) => {
        retryOf.current = retry ?? null;
        session.current = null;
        code.current = null;
        setPhase({ kind: 'loading' });
        loadSDK(config.app_id, config.graph_api_version)
            .then((fb) => {
                setPhase({ kind: 'popup' });
                fb.login((res) => {
                    const c = res.authResponse?.code;
                    if (!c) {
                        // Closed without finishing; the message (if any)
                        // already said why.
                        setPhase((p) =>
                            p.kind === 'popup' ? { kind: 'idle' } : p,
                        );
                        return;
                    }
                    code.current = c;
                    submit();
                }, loginOptions(config.configuration_id));
            })
            .catch(() =>
                setPhase({
                    kind: 'message',
                    tone: 'error',
                    text: t('channels.meta.tp.sdkFailed'),
                }),
            );
    };

    const busy =
        phase.kind === 'loading' ||
        phase.kind === 'popup' ||
        phase.kind === 'running';
    const tpAccounts = accounts.filter(
        (a) => a.connection_method === 'tech_provider',
    );
    const unfinished = config.onboardings.filter(
        (o) => o.status === 'failed' || o.status === 'running',
    );

    return (
        <Card data-testid="tech-provider-card">
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    <Sparkles className="size-4" />
                    {t('channels.meta.tp.title')}
                </CardTitle>
                <CardDescription>
                    {t('channels.meta.tp.description')}
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6 text-sm">
                <section className="space-y-2">
                    <h4 className="flex items-center gap-2 font-medium">
                        <ShieldCheck className="size-4" />
                        {t('channels.meta.tp.permissionsTitle')}
                    </h4>
                    <ul className="space-y-1.5 text-muted-foreground">
                        {SCOPES.map((s) => (
                            <li key={s} className="flex gap-2">
                                <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
                                <span>
                                    <code className="font-mono text-xs text-foreground">
                                        {s}
                                    </code>
                                    {' — '}
                                    {t(`channels.meta.tp.scopes.${s}`)}
                                </span>
                            </li>
                        ))}
                    </ul>
                    <p className="text-xs text-muted-foreground">
                        {t('channels.meta.tp.revoke')}
                    </p>
                </section>

                <section className="space-y-2">
                    <h4 className="font-medium" id="tp-pin">
                        {t('channels.meta.tp.pinTitle')}
                    </h4>
                    <p className="text-muted-foreground">
                        {t('channels.meta.tp.pinHint')}
                    </p>
                    <RadioCards
                        value={pinMode}
                        onValueChange={(v) => setPinMode(v as 'auto' | 'own')}
                        className="sm:grid-cols-2"
                        aria-labelledby="tp-pin"
                    >
                        <RadioCard value="auto" data-testid="tp-pin-auto">
                            {t('channels.meta.tp.pinAuto')}
                        </RadioCard>
                        <RadioCard value="own" data-testid="tp-pin-own">
                            {t('channels.meta.tp.pinOwn')}
                        </RadioCard>
                    </RadioCards>
                    {pinMode === 'own' && (
                        <div className="max-w-48 space-y-1.5">
                            <Label htmlFor="tp-pin-input">
                                {t('channels.meta.tp.pinLabel')}
                            </Label>
                            <Input
                                id="tp-pin-input"
                                inputMode="numeric"
                                autoComplete="off"
                                maxLength={6}
                                value={pin}
                                onChange={(e) =>
                                    setPin(e.target.value.replace(/\D/g, ''))
                                }
                                aria-invalid={pin !== '' && !pinValid}
                            />
                            {pin !== '' && !pinValid && (
                                <p className="text-xs text-destructive">
                                    {t('channels.meta.tp.pinInvalid')}
                                </p>
                            )}
                        </div>
                    )}
                </section>

                <div className="flex flex-wrap items-center gap-3">
                    <Button
                        onClick={() => connect()}
                        disabled={busy || !pinValid}
                        data-testid="tp-connect"
                        className="bg-[#1877F2] text-white hover:bg-[#1877F2]/90"
                    >
                        {busy ? <Spinner /> : <MetaMark />}{' '}
                        {t('channels.meta.tp.connect')}
                    </Button>
                    {phase.kind === 'loading' && (
                        <span className="text-muted-foreground">
                            {t('channels.meta.tp.sdkLoading')}
                        </span>
                    )}
                    {phase.kind === 'popup' && (
                        <span className="text-muted-foreground">
                            {t('channels.meta.tp.popupOpen')}
                        </span>
                    )}
                </div>

                {phase.kind === 'running' && <Progress />}
                {phase.kind === 'message' && (
                    <Alert
                        variant={
                            phase.tone === 'error' ? 'destructive' : 'default'
                        }
                        role="status"
                    >
                        <CircleAlert />
                        <AlertDescription>{phase.text}</AlertDescription>
                    </Alert>
                )}
                {phase.kind === 'done' && (
                    <Success onboarding={phase.onboarding} />
                )}
                {phase.kind === 'failed' && (
                    <Failure
                        onboarding={phase.onboarding}
                        onConnect={connect}
                        onResult={finished}
                    />
                )}

                {unfinished.length > 0 && phase.kind !== 'failed' && (
                    <section className="space-y-2">
                        <h4 className="font-medium">
                            {t('channels.meta.tp.unfinished')}
                        </h4>
                        {unfinished.map((o) => (
                            <Failure
                                key={o.id}
                                onboarding={o}
                                onConnect={connect}
                                onResult={finished}
                            />
                        ))}
                    </section>
                )}

                {tpAccounts.length > 0 && (
                    <section className="space-y-2">
                        <h4 className="font-medium">
                            {t('channels.meta.tp.accountsTitle')}
                        </h4>
                        <ul className="space-y-2" data-testid="tp-accounts">
                            {tpAccounts.map((a) => (
                                <TPAccount key={a.id} account={a} />
                            ))}
                        </ul>
                    </section>
                )}
            </CardContent>
        </Card>
    );
}

function MetaMark() {
    return (
        <svg viewBox="0 0 24 24" aria-hidden className="size-4 fill-current">
            <path d="M24 12.07C24 5.41 18.63 0 12 0S0 5.4 0 12.07C0 18.1 4.39 23.1 10.13 24v-8.44H7.08v-3.49h3.04V9.41c0-3.02 1.8-4.7 4.54-4.7 1.31 0 2.68.24 2.68.24v2.97h-1.5c-1.5 0-1.96.93-1.96 1.89v2.26h3.32l-.53 3.5h-2.8V24C19.62 23.1 24 18.1 24 12.07" />
        </svg>
    );
}

/** The steps while the server connects (it answers at the end). */
function Progress() {
    const t = useT();
    return (
        <ol className="space-y-1.5" role="status" data-testid="tp-progress">
            <li className="flex items-center gap-2 font-medium">
                <Loader2 className="size-4 animate-spin" />
                {t('channels.meta.tp.running')}
            </li>
            {STEPS.map((s) => (
                <li
                    key={s}
                    className="flex items-center gap-2 pl-6 text-muted-foreground"
                >
                    {t(`channels.meta.tp.steps.${s}`)}
                </li>
            ))}
        </ol>
    );
}

function StepList({ onboarding: o }: { onboarding: TechProviderOnboarding }) {
    const t = useT();
    const at = STEPS.indexOf(o.step as (typeof STEPS)[number]);
    return (
        <ol className="space-y-1">
            {STEPS.map((s, i) => {
                const done = o.status === 'completed' || (at >= 0 && i < at);
                const failed = o.status === 'failed' && i === at;
                const Icon = done ? CheckCircle2 : failed ? XCircle : null;
                return (
                    <li
                        key={s}
                        className={cn(
                            'flex items-center gap-2',
                            done && 'text-foreground',
                            failed && 'text-destructive',
                            !done && !failed && 'text-muted-foreground',
                        )}
                        data-state={
                            done ? 'done' : failed ? 'failed' : 'pending'
                        }
                    >
                        {Icon ? (
                            <Icon className="size-4" />
                        ) : (
                            <span className="size-4" />
                        )}
                        {t(`channels.meta.tp.steps.${s}`)}
                    </li>
                );
            })}
        </ol>
    );
}

function Success({ onboarding: o }: { onboarding: TechProviderOnboarding }) {
    const t = useT();
    return (
        <Alert data-testid="tp-success" role="status">
            <CheckCircle2 className="text-emerald-600 dark:text-emerald-400" />
            <AlertTitle>
                {t('channels.meta.tp.success', { name: o.name })}
            </AlertTitle>
            <AlertDescription className="space-y-2">
                <StepList onboarding={o} />
                {o.pin && (
                    <p className="flex flex-wrap items-center gap-2">
                        {t('channels.meta.tp.pinShown', { pin: o.pin })}
                        <Button
                            size="sm"
                            variant="outline"
                            onClick={() =>
                                navigator.clipboard
                                    .writeText(o.pin ?? '')
                                    .then(() =>
                                        toast.success(
                                            t('channels.meta.tp.pinCopied'),
                                        ),
                                    )
                                    .catch(() => undefined)
                            }
                        >
                            <Copy /> {t('channels.meta.tp.pinCopy')}
                        </Button>
                    </p>
                )}
            </AlertDescription>
        </Alert>
    );
}

/** A failed onboarding: where it stopped, why, and how to go on. */
function Failure({
    onboarding: o,
    onConnect,
    onResult,
}: {
    onboarding: TechProviderOnboarding;
    onConnect: (retry?: string) => void;
    onResult: (o: TechProviderOnboarding) => void;
}) {
    const t = useT();
    const { guard } = useReauth();
    const [pending, start] = useTransition();
    const [newPin, setNewPin] = useState('');
    const needsCode =
        o.error_code === 'code_required' || o.error_code === 'code_invalid';
    const needsPin = o.step === 'register';
    const key =
        `channels.meta.tp.errors.${o.error_code ?? 'internal'}` as MessageKey;
    const explained = t(key);
    const reason = explained === key ? (o.error_message ?? '') : explained;

    const retry = () =>
        start(async () => {
            const r = await guard(() =>
                retryOnboardingAction(o.id, {
                    pin: /^[0-9]{6}$/.test(newPin) ? newPin : undefined,
                }),
            );
            if (!r.ok) toast.error(r.message);
            else onResult(r.data);
        });
    const cancel = () =>
        start(async () => {
            const r = await guard(() => cancelOnboardingAction(o.id));
            if (r.ok) toast.success(r.message);
            else toast.error(r.message);
        });

    return (
        <Alert
            variant="destructive"
            data-testid="tp-failure"
            data-step={o.step}
            data-code={o.error_code ?? ''}
        >
            <XCircle />
            <AlertTitle>
                {t('channels.meta.tp.failed', {
                    step: t(
                        `channels.meta.tp.steps.${o.step === 'done' ? 'finish' : o.step}` as MessageKey,
                    ),
                })}{' '}
                <span className="font-mono text-xs">{o.waba_id}</span>
            </AlertTitle>
            <AlertDescription className="space-y-3">
                <p>{reason}</p>
                {o.error_message && reason !== o.error_message && (
                    <p className="text-xs opacity-80">{o.error_message}</p>
                )}
                <StepList onboarding={o} />
                {needsPin && (
                    <div className="max-w-48 space-y-1.5">
                        <Label htmlFor={`tp-new-pin-${o.id}`}>
                            {t('channels.meta.tp.pinLabel')}
                        </Label>
                        <Input
                            id={`tp-new-pin-${o.id}`}
                            inputMode="numeric"
                            autoComplete="off"
                            maxLength={6}
                            value={newPin}
                            onChange={(e) =>
                                setNewPin(e.target.value.replace(/\D/g, ''))
                            }
                        />
                    </div>
                )}
                <div className="flex flex-wrap gap-2">
                    {needsCode ? (
                        <Button
                            size="sm"
                            onClick={() => onConnect(o.id)}
                            disabled={pending}
                        >
                            <MetaMark /> {t('channels.meta.tp.retryWithMeta')}
                        </Button>
                    ) : (
                        <Button
                            size="sm"
                            onClick={retry}
                            disabled={
                                pending ||
                                (newPin !== '' && !/^[0-9]{6}$/.test(newPin))
                            }
                            data-testid="tp-retry"
                        >
                            {pending ? <Spinner /> : <RotateCcw />}{' '}
                            {t('channels.meta.tp.retry')}
                        </Button>
                    )}
                    <Button
                        size="sm"
                        variant="outline"
                        onClick={cancel}
                        disabled={pending}
                    >
                        {t('channels.meta.tp.cancel')}
                    </Button>
                </div>
            </AlertDescription>
        </Alert>
    );
}

function TPAccount({ account: a }: { account: WhatsAppAccount }) {
    const t = useT();
    const { guard } = useReauth();
    const [pending, start] = useTransition();
    const disconnect = () =>
        start(async () => {
            const r = await guard(() => disconnectTechProviderAction(a.id));
            if (!r.ok) {
                toast.error(r.message);
                return;
            }
            toast.success(r.message);
            if (r.data.warning) toast.warning(r.data.warning);
        });
    return (
        <li className="flex flex-wrap items-center gap-2 rounded-lg border p-3">
            <span className="font-medium">{a.name}</span>
            <span className="font-mono text-xs text-muted-foreground">
                {a.waba_id}
            </span>
            <Badge variant={a.status === 'active' ? 'secondary' : 'outline'}>
                {t(`channels.accounts.statuses.${a.status}`)}
            </Badge>
            {a.status !== 'disabled' && (
                <AlertDialog>
                    <AlertDialogTrigger asChild>
                        <Button
                            size="sm"
                            variant="outline"
                            className="ml-auto"
                            disabled={pending}
                        >
                            {pending ? <Spinner /> : <Link2Off />}{' '}
                            {t('channels.meta.tp.disconnect')}
                        </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                        <AlertDialogHeader>
                            <AlertDialogTitle>
                                {t('channels.meta.tp.disconnectTitle', {
                                    name: a.name,
                                })}
                            </AlertDialogTitle>
                            <AlertDialogDescription>
                                {t('channels.meta.tp.disconnectBody')}
                            </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                            <AlertDialogCancel>
                                {t('common.cancel')}
                            </AlertDialogCancel>
                            <AlertDialogAction onClick={disconnect}>
                                {t('channels.meta.tp.disconnect')}
                            </AlertDialogAction>
                        </AlertDialogFooter>
                    </AlertDialogContent>
                </AlertDialog>
            )}
        </li>
    );
}
