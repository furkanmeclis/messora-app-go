'use client';

import { useState, useTransition } from 'react';
import Link from 'next/link';
import {
    Activity,
    AlertTriangle,
    CheckCircle2,
    CircleAlert,
    ExternalLink,
    KeyRound,
    Plus,
    RefreshCw,
    Sparkles,
    Stethoscope,
    XCircle,
} from 'lucide-react';
import { toast } from 'sonner';
import { useFormatDateTime } from '@/components/display-preferences';
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
import { Label } from '@/components/ui/label';
import { RadioCard, RadioCards } from '@/components/ui/radio-cards';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { useT } from '@/i18n/client';
import type { MessageKey } from '@/i18n/translate';
import type {
    MetaConnection,
    MetaConnectionCheck,
    MetaConnectionTest,
    TechProvider,
    WhatsAppAccount,
} from '@/lib/types';
import { cn } from '@/lib/utils';
import {
    ConnectAccountDialog,
    CopyField,
} from '../../channels/channels-client';
import { TokenHealth } from '../../channels/token-health';
import {
    rotateVerifyTokenAction,
    rotateWebhookKeyAction,
    testConnectionAction,
} from './actions';
import { TechProviderCard } from './tech-provider';

/** Settings → WhatsApp / Meta: connect the company's own Meta app. */
export function MetaConnectionView({
    connection: c,
    accounts,
    techProvider,
}: {
    connection: MetaConnection;
    accounts: WhatsAppAccount[];
    /** Set only while the platform offers "Connect with Meta". */
    techProvider: TechProvider | null;
}) {
    const t = useT();
    const offered = c.tech_provider_enabled && techProvider != null;
    const [method, setMethod] = useState<'own_app' | 'tech_provider'>(
        offered && c.method === 'tech_provider' ? 'tech_provider' : 'own_app',
    );
    const ownAccounts = accounts.filter(
        (a) => a.connection_method !== 'tech_provider',
    );
    return (
        <div className="space-y-6">
            <div className="space-y-1">
                <h2 className="text-lg font-semibold">
                    {t('channels.meta.title')}
                </h2>
                <p className="text-sm text-muted-foreground">
                    {t('channels.meta.description')}
                </p>
            </div>
            <MethodPicker
                techProvider={offered}
                value={method}
                onChange={setMethod}
            />
            {method === 'tech_provider' && techProvider && (
                <TechProviderCard config={techProvider} accounts={accounts} />
            )}
            <Card hidden={method !== 'own_app'}>
                <CardHeader>
                    <CardTitle>{t('channels.meta.steps.title')}</CardTitle>
                    <CardDescription>
                        {t('channels.meta.steps.description')}
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <ol className="space-y-6" data-testid="meta-steps">
                        <Step n={1} title={t('channels.meta.steps.app.title')}>
                            <p>{t('channels.meta.steps.app.body')}</p>
                            <ExternalAnchor href="https://developers.facebook.com/apps">
                                {t('channels.meta.steps.app.link')}
                            </ExternalAnchor>
                        </Step>
                        <Step
                            n={2}
                            title={t('channels.meta.steps.token.title')}
                        >
                            <p>{t('channels.meta.steps.token.body')}</p>
                            <ExternalAnchor href="https://business.facebook.com/settings/system-users">
                                {t('channels.guide.open')}
                            </ExternalAnchor>
                        </Step>
                        <Step
                            n={3}
                            title={t('channels.meta.steps.account.title')}
                            done={ownAccounts.length > 0}
                        >
                            <p>{t('channels.meta.steps.account.body')}</p>
                            <div className="flex flex-wrap items-center gap-3">
                                <ConnectAccountDialog
                                    trigger={
                                        <Button
                                            variant={
                                                accounts.length > 0
                                                    ? 'outline'
                                                    : 'default'
                                            }
                                        >
                                            <Plus />{' '}
                                            {accounts.length > 0
                                                ? t(
                                                      'channels.meta.steps.account.another',
                                                  )
                                                : t(
                                                      'channels.meta.steps.account.connect',
                                                  )}
                                        </Button>
                                    }
                                />
                                {accounts.length > 0 && (
                                    <Link
                                        href="/channels"
                                        className="text-xs text-muted-foreground underline-offset-4 hover:underline"
                                    >
                                        {t(
                                            'channels.meta.steps.account.manage',
                                        )}
                                    </Link>
                                )}
                            </div>
                        </Step>
                        <Step
                            n={4}
                            title={t('channels.meta.steps.webhook.title')}
                            done={c.health.last_webhook_at != null}
                        >
                            <WebhookValues connection={c} />
                        </Step>
                        <Step n={5} title={t('channels.meta.steps.test.title')}>
                            <p>{t('channels.meta.steps.test.body')}</p>
                            <ConnectionTests accounts={ownAccounts} />
                        </Step>
                    </ol>
                </CardContent>
            </Card>
            <HealthCard connection={c} accounts={accounts} />
        </div>
    );
}

function MethodPicker({
    techProvider,
    value,
    onChange,
}: {
    techProvider: boolean;
    value: 'own_app' | 'tech_provider';
    onChange: (v: 'own_app' | 'tech_provider') => void;
}) {
    const t = useT();
    return (
        <section className="space-y-2" aria-labelledby="meta-method">
            <h3 id="meta-method" className="text-sm font-medium">
                {t('channels.meta.method.title')}
            </h3>
            <RadioCards
                value={value}
                onValueChange={(v) =>
                    onChange(v as 'own_app' | 'tech_provider')
                }
                className="sm:grid-cols-2"
                aria-labelledby="meta-method"
            >
                <RadioCard value="own_app" data-testid="method-own-app">
                    <span className="flex items-center gap-2 font-medium">
                        <KeyRound className="size-4" />
                        {t('channels.meta.method.ownApp')}
                    </span>
                    <span className="mt-1 block text-muted-foreground">
                        {t('channels.meta.method.ownAppHint')}
                    </span>
                </RadioCard>
                {/* On only with the platform switch (platform_settings.tech_provider). */}
                <RadioCard
                    value="tech_provider"
                    disabled={!techProvider}
                    data-testid="method-tech-provider"
                >
                    <span className="flex flex-wrap items-center gap-2 font-medium">
                        <Sparkles className="size-4" />
                        {t('channels.meta.method.techProvider')}
                        {!techProvider && (
                            <Badge variant="secondary">
                                {t('channels.meta.method.soon')}
                            </Badge>
                        )}
                    </span>
                    <span className="mt-1 block text-muted-foreground">
                        {techProvider
                            ? t('channels.meta.method.techProviderHintOn')
                            : t('channels.meta.method.techProviderHint')}
                    </span>
                </RadioCard>
            </RadioCards>
        </section>
    );
}

function Step({
    n,
    title,
    done,
    children,
}: {
    n: number;
    title: string;
    done?: boolean;
    children: React.ReactNode;
}) {
    return (
        <li className="flex gap-3">
            <span
                className={cn(
                    'flex size-7 shrink-0 items-center justify-center rounded-full text-sm font-semibold',
                    done
                        ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400'
                        : 'bg-primary/10 text-primary',
                )}
                aria-hidden
            >
                {done ? <CheckCircle2 className="size-4" /> : n}
            </span>
            <div className="min-w-0 flex-1 space-y-3 text-sm">
                <h4 className="pt-1 font-medium">{title}</h4>
                <div className="space-y-3 text-muted-foreground">
                    {children}
                </div>
            </div>
        </li>
    );
}

function ExternalAnchor({
    href,
    children,
}: {
    href: string;
    children: React.ReactNode;
}) {
    return (
        <a
            className="inline-flex items-center gap-1 text-xs text-primary underline-offset-4 hover:underline"
            href={href}
            target="_blank"
            rel="noreferrer"
        >
            {children} <ExternalLink className="size-3" />
        </a>
    );
}

function WebhookValues({ connection: c }: { connection: MetaConnection }) {
    const t = useT();
    const dateTime = useFormatDateTime();
    return (
        <div className="space-y-4">
            <p>{t('channels.meta.steps.webhook.body')}</p>
            <div className="flex flex-wrap gap-1">
                {c.fields.map((f) => (
                    <Badge key={f} variant="outline" className="font-mono">
                        {f}
                    </Badge>
                ))}
            </div>
            <div className="space-y-3 text-foreground">
                <CopyField
                    id="meta-callback"
                    label={t('channels.webhook.callbackUrl')}
                    copyLabel={t('channels.webhook.copyCallbackUrl')}
                    value={c.callback_url}
                />
                <CopyField
                    id="meta-verify-token"
                    label={t('channels.webhook.verifyToken')}
                    copyLabel={t('channels.webhook.copyVerifyToken')}
                    value={c.verify_token}
                />
            </div>
            {c.previous_key_expires_at && (
                <p className="text-xs" role="status">
                    {t('channels.meta.rotate.previousUntil', {
                        date: dateTime(c.previous_key_expires_at),
                    })}
                </p>
            )}
            <div className="flex flex-wrap gap-2">
                <RotateKeyButton />
                <RotateVerifyTokenButton />
            </div>
            <details className="rounded-lg border px-3 py-2">
                <summary className="cursor-pointer text-xs font-medium text-foreground">
                    {t('channels.meta.hostUrl')}
                </summary>
                <div className="mt-2 space-y-2">
                    <p className="text-xs">{t('channels.meta.hostUrlHint')}</p>
                    <code
                        className="block rounded bg-muted px-2 py-1 font-mono text-xs break-all text-foreground"
                        data-testid="host-callback-url"
                    >
                        {c.host_callback_url}
                    </code>
                </div>
            </details>
        </div>
    );
}

const GRACE_HOURS = [0, 24, 72, 168] as const;

function RotateKeyButton() {
    const t = useT();
    const { guard } = useReauth();
    const [grace, setGrace] = useState<string>('24');
    const [pending, start] = useTransition();
    const labels: Record<(typeof GRACE_HOURS)[number], string> = {
        0: t('channels.meta.rotate.graceNone'),
        24: t('channels.meta.rotate.graceDay'),
        72: t('channels.meta.rotate.grace3'),
        168: t('channels.meta.rotate.graceWeek'),
    };
    const rotate = () =>
        start(async () => {
            const r = await guard(() => rotateWebhookKeyAction(Number(grace)));
            if (r.ok) toast.success(r.message);
            else toast.error(r.message);
        });
    return (
        <AlertDialog>
            <AlertDialogTrigger asChild>
                <Button size="sm" variant="outline" disabled={pending}>
                    {pending ? <Spinner /> : <RefreshCw />}{' '}
                    {t('channels.meta.rotate.open')}
                </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle>
                        {t('channels.meta.rotate.title')}
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                        {t('channels.meta.rotate.description')}
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <div className="space-y-1.5">
                    <Label htmlFor="meta-grace">
                        {t('channels.meta.rotate.grace')}
                    </Label>
                    <Select value={grace} onValueChange={setGrace}>
                        <SelectTrigger id="meta-grace" className="w-full">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {GRACE_HOURS.map((h) => (
                                <SelectItem key={h} value={String(h)}>
                                    {labels[h]}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>
                <AlertDialogFooter>
                    <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                    <AlertDialogAction onClick={rotate}>
                        {t('channels.meta.rotate.confirm')}
                    </AlertDialogAction>
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    );
}

function RotateVerifyTokenButton() {
    const t = useT();
    const { guard } = useReauth();
    const [pending, start] = useTransition();
    const rotate = () =>
        start(async () => {
            const r = await guard(() => rotateVerifyTokenAction());
            if (r.ok) toast.success(r.message);
            else toast.error(r.message);
        });
    return (
        <AlertDialog>
            <AlertDialogTrigger asChild>
                <Button size="sm" variant="outline" disabled={pending}>
                    {pending ? <Spinner /> : <KeyRound />}{' '}
                    {t('channels.meta.verify.open')}
                </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle>
                        {t('channels.meta.verify.title')}
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                        {t('channels.meta.verify.description')}
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                    <AlertDialogAction onClick={rotate}>
                        {t('channels.meta.verify.confirm')}
                    </AlertDialogAction>
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    );
}

function ConnectionTests({ accounts }: { accounts: WhatsAppAccount[] }) {
    const t = useT();
    if (accounts.length === 0) {
        return <p className="text-xs">{t('channels.meta.test.none')}</p>;
    }
    return (
        <ul className="space-y-3 text-foreground">
            {accounts.map((a) => (
                <AccountTest key={a.id} account={a} />
            ))}
        </ul>
    );
}

const CHECK_ICON = {
    ok: CheckCircle2,
    warning: CircleAlert,
    error: XCircle,
} as const;

const CHECK_TONE = {
    ok: 'text-emerald-600 dark:text-emerald-400',
    warning: 'text-amber-600 dark:text-amber-400',
    error: 'text-destructive',
} as const;

function AccountTest({ account: a }: { account: WhatsAppAccount }) {
    const t = useT();
    const dateTime = useFormatDateTime();
    const { guard } = useReauth();
    const [pending, start] = useTransition();
    const [result, setResult] = useState<MetaConnectionTest | null>(null);

    const run = () =>
        start(async () => {
            const r = await guard(() => testConnectionAction(a.id));
            if (!r.ok) {
                toast.error(r.message);
                return;
            }
            setResult(r.data);
            if (r.data.ok) toast.success(t('channels.meta.test.ok'));
            else toast.error(t('channels.meta.test.failed'));
        });

    /** The translated outcome; the server's English text when unknown. */
    const text = (c: MetaConnectionCheck) => {
        const vars: Record<string, string> = { ...c.params };
        for (const k of ['at', 'expires_at'] as const) {
            if (vars[k]) vars[k] = dateTime(vars[k]);
        }
        const key = `channels.meta.test.codes.${c.code}` as MessageKey;
        const out = t(key, vars);
        return out === key ? c.message : out;
    };

    return (
        <li
            className="space-y-3 rounded-lg border p-3"
            data-testid="account-test"
        >
            <div className="flex flex-wrap items-center gap-2">
                <span className="font-medium">{a.name}</span>
                <span className="font-mono text-xs text-muted-foreground">
                    {a.waba_id}
                </span>
                <Button
                    size="sm"
                    className="ml-auto"
                    disabled={pending || a.status === 'disabled'}
                    onClick={run}
                >
                    {pending ? <Spinner /> : <Stethoscope />}{' '}
                    {t('channels.meta.test.button')}
                </Button>
            </div>
            {result && (
                <ul className="space-y-2 text-sm" role="status">
                    {result.checks.map((c) => {
                        const Icon = CHECK_ICON[c.status];
                        return (
                            <li key={c.key} className="flex gap-2">
                                <Icon
                                    className={cn(
                                        'mt-0.5 size-4 shrink-0',
                                        CHECK_TONE[c.status],
                                    )}
                                    aria-label={c.status}
                                />
                                <span>
                                    <span className="font-medium">
                                        {t(`channels.meta.test.keys.${c.key}`)}
                                    </span>
                                    <span className="text-muted-foreground">
                                        {' — '}
                                        {text(c)}
                                    </span>
                                </span>
                            </li>
                        );
                    })}
                </ul>
            )}
        </li>
    );
}

function HealthCard({
    connection: c,
    accounts,
}: {
    connection: MetaConnection;
    accounts: WhatsAppAccount[];
}) {
    const t = useT();
    const dateTime = useFormatDateTime();
    const h = c.health;
    const signatureRecent =
        h.last_signature_failure_at != null &&
        (h.last_webhook_at == null ||
            Date.parse(h.last_signature_failure_at) >
                Date.parse(h.last_webhook_at));
    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    <Activity className="size-4" />
                    {t('channels.meta.health.title')}
                </CardTitle>
                <CardDescription>
                    {t('channels.meta.health.description')}
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
                {!c.signature_verification && (
                    <Alert
                        variant="destructive"
                        data-testid="signature-warning"
                    >
                        <AlertTriangle />
                        <AlertTitle>
                            {t('channels.webhook.signatureOff')}
                        </AlertTitle>
                        <AlertDescription>
                            {t('channels.meta.health.noSecret')}
                        </AlertDescription>
                    </Alert>
                )}
                <dl className="grid gap-3 sm:grid-cols-3">
                    <Stat label={t('channels.meta.health.lastWebhook')}>
                        {h.last_webhook_at
                            ? dateTime(h.last_webhook_at)
                            : t('channels.meta.health.never')}
                    </Stat>
                    <Stat
                        label={t('channels.meta.health.signatureFailures')}
                        hint={
                            h.last_signature_failure_at
                                ? t('channels.meta.health.lastAt', {
                                      date: dateTime(
                                          h.last_signature_failure_at,
                                      ),
                                  })
                                : undefined
                        }
                    >
                        {h.signature_failures}
                    </Stat>
                    <Stat
                        label={t('channels.meta.health.mismatches')}
                        hint={
                            h.last_waba_mismatch_at
                                ? t('channels.meta.health.lastAt', {
                                      date: dateTime(h.last_waba_mismatch_at),
                                  })
                                : undefined
                        }
                    >
                        {h.waba_mismatches}
                    </Stat>
                </dl>
                {signatureRecent && (
                    <Alert>
                        <CircleAlert />
                        <AlertDescription>
                            {t('channels.meta.health.signatureHint')}
                        </AlertDescription>
                    </Alert>
                )}
                {h.last_mismatch_waba_id && (
                    <Alert>
                        <CircleAlert />
                        <AlertDescription>
                            {t('channels.meta.health.mismatchHint', {
                                waba: h.last_mismatch_waba_id,
                            })}
                        </AlertDescription>
                    </Alert>
                )}
                {accounts.some((a) => a.status !== 'disabled') && (
                    <div className="space-y-2">
                        <h4 className="text-sm font-medium">
                            {t('channels.meta.health.tokens')}
                        </h4>
                        {accounts
                            .filter((a) => a.status !== 'disabled')
                            .map((a) => (
                                <div key={a.id} className="space-y-1">
                                    <p className="text-xs text-muted-foreground">
                                        {a.name}
                                    </p>
                                    <TokenHealth account={a} manage />
                                </div>
                            ))}
                    </div>
                )}
            </CardContent>
        </Card>
    );
}

function Stat({
    label,
    hint,
    children,
}: {
    label: string;
    hint?: string;
    children: React.ReactNode;
}) {
    return (
        <div className="rounded-lg border bg-muted/20 p-3">
            <dt className="text-xs text-muted-foreground">{label}</dt>
            <dd className="mt-1 text-sm font-medium tabular-nums">
                {children}
            </dd>
            {hint && <dd className="text-xs text-muted-foreground">{hint}</dd>}
        </div>
    );
}
