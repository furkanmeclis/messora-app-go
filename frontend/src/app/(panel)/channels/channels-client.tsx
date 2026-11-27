'use client';

import { useState, useTransition } from 'react';
import Link from 'next/link';
import {
    AlertCircle,
    AlertTriangle,
    Copy,
    KeyRound,
    Phone,
    Plus,
    RefreshCw,
    Star,
} from 'lucide-react';
import { toast } from 'sonner';
import { ClientDataTable } from '@/components/client-data-table';
import { TextField } from '@/components/forms';
import { newIdempotencyKey, notify } from '@/components/notify';
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
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from '@/components/ui/dialog';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { FieldGroup } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Spinner } from '@/components/ui/spinner';
import { useFormatDateTime } from '@/components/display-preferences';
import { useT } from '@/i18n/client';
import type { ActionResult, PhoneNumber, WhatsAppAccount } from '@/lib/types';
import {
    connectAccountAction,
    disconnectAccountAction,
    setDefaultNumberAction,
    syncAccountAction,
    updateAccountAction,
} from './actions';
import { AccountProblem, SystemUserGuide, TokenHealth } from './token-health';

type Failure = Extract<ActionResult<unknown>, { ok: false }>;

/** Errors for fields the form does not show, plus the overall message. */
function ErrorBanner({
    error,
    shown,
}: {
    error: Failure | null;
    shown: string[];
}) {
    if (!error) return null;
    const other = Object.entries(error.fieldErrors ?? {}).filter(
        ([k]) => !shown.includes(k),
    );
    const onlyShown =
        error.code === 'VALIDATION_ERROR' &&
        other.length === 0 &&
        Object.keys(error.fieldErrors ?? {}).length > 0;
    if (onlyShown) return null;
    return (
        <Alert variant="destructive">
            <AlertCircle />
            <AlertDescription>
                <p>{error.message}</p>
                {other.length > 0 && (
                    <ul className="mt-1 list-disc pl-4">
                        {other.map(([k, v]) => (
                            <li key={k}>
                                {k ? <code>{k}</code> : null} {v}
                            </li>
                        ))}
                    </ul>
                )}
            </AlertDescription>
        </Alert>
    );
}

const STATUS_BADGE: Record<
    WhatsAppAccount['status'],
    'secondary' | 'destructive' | 'outline'
> = {
    active: 'secondary',
    error: 'destructive',
    disabled: 'outline',
};

// ── Connect ───────────────────────────────────────────────────────────────

const CONNECT_FIELDS = [
    'name',
    'waba_id',
    'access_token',
    'app_id',
    'app_secret',
];

export function ConnectAccountDialog({
    trigger,
}: {
    trigger?: React.ReactNode;
}) {
    const t = useT();
    const { guard } = useReauth();
    const [open, setOpen] = useState(false);
    const [idem, setIdem] = useState(newIdempotencyKey);
    const [pending, startTransition] = useTransition();
    const [error, setError] = useState<Failure | null>(null);
    const fe = (name: string) => error?.fieldErrors?.[name];

    const submit = (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        const form = new FormData(e.currentTarget);
        form.set('idempotency_key', idem);
        startTransition(async () => {
            const r = await guard(() => connectAccountAction(form));
            if (r.ok) {
                notify(r);
                setError(null);
                setOpen(false);
                setIdem(newIdempotencyKey());
            } else {
                setError(r);
                // A rejected token may be fixed and sent again: new intent.
                setIdem(newIdempotencyKey());
            }
        });
    };

    return (
        <Dialog
            open={open}
            onOpenChange={(o) => {
                setOpen(o);
                if (!o) setError(null);
            }}
        >
            <DialogTrigger asChild>
                {trigger ?? (
                    <Button shortcut="n">
                        <Plus /> {t('channels.connect.open')}
                    </Button>
                )}
            </DialogTrigger>
            <DialogContent className="max-h-[90svh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>{t('channels.connect.title')}</DialogTitle>
                    <DialogDescription>
                        {t('channels.connect.description')}
                    </DialogDescription>
                </DialogHeader>
                <form
                    onSubmit={submit}
                    className="space-y-4"
                    noValidate={false}
                >
                    <ErrorBanner error={error} shown={CONNECT_FIELDS} />
                    <SystemUserGuide />
                    <FieldGroup>
                        <TextField
                            label={t('channels.connect.name')}
                            name="name"
                            required
                            maxLength={120}
                            placeholder={t('channels.connect.namePlaceholder')}
                            error={fe('name')}
                        />
                        <TextField
                            label={t('channels.connect.wabaId')}
                            name="waba_id"
                            required
                            inputMode="numeric"
                            pattern="[0-9]{1,32}"
                            title={t('channels.connect.digitsOnly')}
                            autoComplete="off"
                            error={fe('waba_id')}
                            hint={t('channels.connect.wabaIdHint')}
                        />
                        <TextField
                            label={t('channels.connect.accessToken')}
                            name="access_token"
                            type="password"
                            required
                            minLength={20}
                            maxLength={1024}
                            autoComplete="off"
                            error={fe('access_token')}
                            hint={t('channels.connect.accessTokenHint')}
                        />
                        <TextField
                            label={t('channels.connect.appId')}
                            name="app_id"
                            required
                            inputMode="numeric"
                            pattern="[0-9]{1,32}"
                            title={t('channels.connect.digitsOnly')}
                            autoComplete="off"
                            error={fe('app_id')}
                            hint={t('channels.connect.appIdHint')}
                        />
                        <TextField
                            label={t('channels.connect.appSecret')}
                            name="app_secret"
                            type="password"
                            required
                            minLength={8}
                            maxLength={256}
                            autoComplete="off"
                            error={fe('app_secret')}
                            hint={t('channels.connect.appSecretHint')}
                        />
                    </FieldGroup>
                    <Button type="submit" className="w-full" disabled={pending}>
                        {pending && <Spinner />} {t('channels.connect.submit')}
                    </Button>
                </form>
            </DialogContent>
        </Dialog>
    );
}

// ── Accounts ──────────────────────────────────────────────────────────────

export function AccountList({
    accounts,
    manage,
}: {
    accounts: WhatsAppAccount[];
    manage: boolean;
}) {
    const t = useT();
    if (accounts.length === 0) {
        return (
            <Empty className="py-10">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <Phone />
                    </EmptyMedia>
                    <EmptyTitle>{t('channels.accounts.empty')}</EmptyTitle>
                    <EmptyDescription>
                        {manage
                            ? t('channels.accounts.emptyManage')
                            : t('channels.accounts.emptyAsk')}
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }
    return (
        <ul className="divide-y" aria-label={t('channels.accounts.label')}>
            {accounts.map((a) => (
                <AccountRow key={a.id} account={a} manage={manage} />
            ))}
        </ul>
    );
}

function AccountRow({
    account: a,
    manage,
}: {
    account: WhatsAppAccount;
    manage: boolean;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const { guard } = useReauth();
    const [pending, startTransition] = useTransition();
    const [syncResult, setSyncResult] = useState<string | null>(null);

    const sync = () =>
        startTransition(async () => {
            const r = await guard(() => syncAccountAction(a.id));
            if (r.ok) {
                const text = t('channels.accounts.synced', {
                    numbers: r.data.phone_numbers,
                    templates: r.data.templates,
                });
                setSyncResult(text);
                toast.success(text);
            } else {
                setSyncResult(null);
                toast.error(r.message);
            }
        });

    const toggle = () =>
        startTransition(async () => {
            notify(
                await guard(() =>
                    updateAccountAction(a.id, {
                        status: a.status === 'disabled' ? 'active' : 'disabled',
                    }),
                ),
            );
        });

    return (
        <li
            className="space-y-3 py-4 first:pt-0 last:pb-0"
            data-testid="account"
        >
            <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium">{a.name}</span>
                        <Badge variant={STATUS_BADGE[a.status]}>
                            {t(`channels.accounts.statuses.${a.status}`)}
                        </Badge>
                        {!a.app_secret_configured && (
                            <Badge variant="outline">
                                {t('channels.accounts.noAppSecret')}
                            </Badge>
                        )}
                    </div>
                    <dl className="grid grid-cols-[auto_1fr] gap-x-3 text-sm text-muted-foreground">
                        <dt>{t('channels.accounts.wabaId')}</dt>
                        <dd className="font-mono">{a.waba_id}</dd>
                        {a.app_id && (
                            <>
                                <dt>{t('channels.accounts.appId')}</dt>
                                <dd className="font-mono">{a.app_id}</dd>
                            </>
                        )}
                        <dt>{t('channels.accounts.lastSynced')}</dt>
                        <dd>{formatDateTime(a.last_synced_at)}</dd>
                        <dt>{t('channels.accounts.connected')}</dt>
                        <dd>{formatDateTime(a.created_at)}</dd>
                    </dl>
                </div>
                {manage && (
                    <div className="flex flex-wrap gap-2">
                        <Button
                            size="sm"
                            variant="outline"
                            disabled={pending || a.status === 'disabled'}
                            onClick={sync}
                        >
                            {pending ? <Spinner /> : <RefreshCw />}{' '}
                            {t('channels.accounts.sync')}
                        </Button>
                        <CredentialsDialog account={a} />
                        <Button
                            size="sm"
                            variant="outline"
                            disabled={pending}
                            onClick={toggle}
                        >
                            {a.status === 'disabled'
                                ? t('channels.accounts.enable')
                                : t('channels.accounts.disable')}
                        </Button>
                        <DisconnectButton account={a} />
                    </div>
                )}
            </div>
            <AccountProblem
                account={a}
                action={
                    manage ? (
                        <CredentialsDialog
                            account={a}
                            trigger={
                                <Button size="sm" variant="destructive">
                                    <KeyRound /> {t('channels.token.replace')}
                                </Button>
                            }
                        />
                    ) : undefined
                }
            />
            {a.last_error && a.status !== 'error' && (
                <Alert variant="destructive">
                    <AlertTriangle />
                    <AlertTitle>{t('channels.accounts.lastError')}</AlertTitle>
                    <AlertDescription>
                        <p>{a.last_error}</p>
                        <p>{t('channels.accounts.lastErrorHint')}</p>
                    </AlertDescription>
                </Alert>
            )}
            {a.status !== 'disabled' && (
                <TokenHealth account={a} manage={manage} />
            )}
            {syncResult && (
                <p className="text-sm text-muted-foreground" role="status">
                    {syncResult}{' '}
                    <Link className="underline" href="/templates">
                        {t('channels.accounts.viewTemplates')}
                    </Link>
                </p>
            )}
        </li>
    );
}

function CredentialsDialog({
    account,
    trigger,
}: {
    account: WhatsAppAccount;
    trigger?: React.ReactNode;
}) {
    const t = useT();
    const { guard } = useReauth();
    const [open, setOpen] = useState(false);
    const [pending, startTransition] = useTransition();
    const [error, setError] = useState<Failure | null>(null);
    const fe = (name: string) => error?.fieldErrors?.[name];

    const submit = (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        const form = new FormData(e.currentTarget);
        const name = String(form.get('name') ?? '').trim();
        const token = String(form.get('access_token') ?? '').trim();
        const secret = String(form.get('app_secret') ?? '').trim();
        const appId = String(form.get('app_id') ?? '').trim();
        if (
            name === account.name &&
            !token &&
            !secret &&
            (!appId || appId === account.app_id)
        ) {
            setOpen(false);
            return;
        }
        startTransition(async () => {
            const r = await guard(() =>
                updateAccountAction(account.id, {
                    name: name !== account.name ? name : undefined,
                    access_token: token || undefined,
                    app_secret: secret || undefined,
                    app_id:
                        appId && appId !== account.app_id ? appId : undefined,
                }),
            );
            if (r.ok) {
                notify(r);
                setError(null);
                setOpen(false);
            } else {
                setError(r);
            }
        });
    };

    return (
        <Dialog
            open={open}
            onOpenChange={(o) => {
                setOpen(o);
                if (!o) setError(null);
            }}
        >
            <DialogTrigger asChild>
                {trigger ?? (
                    <Button size="sm" variant="outline">
                        <KeyRound /> {t('channels.edit.open')}
                    </Button>
                )}
            </DialogTrigger>
            <DialogContent className="max-h-[90svh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>
                        {t('channels.edit.title', { name: account.name })}
                    </DialogTitle>
                    <DialogDescription>
                        {t('channels.edit.description')}
                    </DialogDescription>
                </DialogHeader>
                <form onSubmit={submit} className="space-y-4">
                    <ErrorBanner
                        error={error}
                        shown={['name', 'access_token', 'app_secret', 'app_id']}
                    />
                    <SystemUserGuide defaultOpen={account.status === 'error'} />
                    <FieldGroup>
                        <TextField
                            label={t('channels.edit.name')}
                            name="name"
                            required
                            maxLength={120}
                            defaultValue={account.name}
                            id={`acc-name-${account.id}`}
                            error={fe('name')}
                        />
                        <TextField
                            label={t('channels.edit.accessToken')}
                            name="access_token"
                            type="password"
                            minLength={20}
                            maxLength={1024}
                            autoComplete="off"
                            id={`acc-token-${account.id}`}
                            error={fe('access_token')}
                            hint={t('channels.edit.accessTokenHint')}
                        />
                        <TextField
                            label={t('channels.connect.appId')}
                            name="app_id"
                            inputMode="numeric"
                            pattern="[0-9]{1,32}"
                            autoComplete="off"
                            defaultValue={account.app_id ?? ''}
                            id={`acc-app-${account.id}`}
                            error={fe('app_id')}
                            hint={t('channels.edit.appIdHint')}
                        />
                        <TextField
                            label={t('channels.edit.appSecret')}
                            name="app_secret"
                            type="password"
                            minLength={8}
                            maxLength={256}
                            autoComplete="off"
                            id={`acc-secret-${account.id}`}
                            error={fe('app_secret')}
                            hint={
                                account.app_secret_configured
                                    ? t('channels.edit.secretConfigured')
                                    : t('channels.edit.noSecret')
                            }
                        />
                    </FieldGroup>
                    <Button type="submit" className="w-full" disabled={pending}>
                        {pending && <Spinner />} {t('common.save')}
                    </Button>
                </form>
            </DialogContent>
        </Dialog>
    );
}

function DisconnectButton({ account }: { account: WhatsAppAccount }) {
    const t = useT();
    const { guard } = useReauth();
    const [open, setOpen] = useState(false);
    const [pending, startTransition] = useTransition();
    const [blocked, setBlocked] = useState<string | null>(null);

    const disconnect = () =>
        startTransition(async () => {
            const r = await guard(() => disconnectAccountAction(account.id));
            if (r.ok) {
                notify(r);
                setOpen(false);
            } else if (r.code === 'ACCOUNT_HAS_MESSAGES') {
                setBlocked(r.message);
            } else {
                toast.error(r.message);
            }
        });

    const disable = () =>
        startTransition(async () => {
            if (
                notify(
                    await guard(() =>
                        updateAccountAction(account.id, { status: 'disabled' }),
                    ),
                )
            ) {
                setOpen(false);
            }
        });

    return (
        <AlertDialog
            open={open}
            onOpenChange={(o) => {
                setOpen(o);
                if (!o) setBlocked(null);
            }}
        >
            <Button
                size="sm"
                variant="outline"
                className="text-destructive"
                onClick={() => setOpen(true)}
            >
                {t('channels.disconnect.open')}
            </Button>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle>
                        {blocked
                            ? t('channels.disconnect.blockedTitle')
                            : t('channels.disconnect.title', {
                                  name: account.name,
                              })}
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                        {blocked
                            ? t('channels.disconnect.blockedDescription', {
                                  reason: blocked,
                              })
                            : t('channels.disconnect.description')}
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <AlertDialogCancel disabled={pending}>
                        {t('common.cancel')}
                    </AlertDialogCancel>
                    {blocked ? (
                        account.status !== 'disabled' && (
                            <Button onClick={disable} disabled={pending}>
                                {pending && <Spinner />}{' '}
                                {t('channels.disconnect.disableInstead')}
                            </Button>
                        )
                    ) : (
                        <AlertDialogAction
                            onClick={(e) => {
                                e.preventDefault();
                                disconnect();
                            }}
                            disabled={pending}
                            variant="destructive"
                        >
                            {pending && <Spinner />}{' '}
                            {t('channels.disconnect.confirm')}
                        </AlertDialogAction>
                    )}
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    );
}

// ── Phone numbers ─────────────────────────────────────────────────────────

const QUALITY_RANK: Record<string, number> = {
    GREEN: 0,
    HIGH: 0,
    YELLOW: 1,
    MEDIUM: 1,
    RED: 2,
    LOW: 2,
};

export function PhoneNumberTable({
    numbers,
    manage,
}: {
    numbers: PhoneNumber[];
    manage: boolean;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const { guard } = useReauth();
    const [pending, startTransition] = useTransition();
    const setDefault = (n: PhoneNumber) =>
        startTransition(async () => {
            notify(await guard(() => setDefaultNumberAction(n.id)));
        });
    return (
        <ClientDataTable
            rows={numbers}
            rowId={(n) => n.id}
            storageKey="phone-numbers"
            initialSort="number"
            searchText={(n) =>
                `${n.display_phone_number} ${n.verified_name} ${n.account_name}`
            }
            searchPlaceholder={t('table.search')}
            empty={
                <p className="p-6 text-sm text-muted-foreground">
                    {t('channels.numbers.empty')}
                </p>
            }
            noMatch={
                <p className="p-8 text-center text-sm text-muted-foreground">
                    {t('table.noMatch')}
                </p>
            }
            columns={[
                {
                    id: 'number',
                    header: t('channels.numbers.number'),
                    hideable: false,
                    sortValue: (n) => n.display_phone_number,
                    cell: (n) => (
                        <span className="font-mono whitespace-nowrap">
                            {n.display_phone_number}
                        </span>
                    ),
                },
                {
                    id: 'verifiedName',
                    header: t('channels.numbers.verifiedName'),
                    sortValue: (n) => n.verified_name || undefined,
                    cell: (n) => n.verified_name || '—',
                },
                {
                    id: 'account',
                    header: t('channels.numbers.account'),
                    sortValue: (n) => n.account_name,
                    cell: (n) => (
                        <span className="text-muted-foreground">
                            {n.account_name}
                        </span>
                    ),
                },
                {
                    id: 'quality',
                    header: t('channels.numbers.quality'),
                    sortValue: (n) =>
                        QUALITY_RANK[n.quality_rating.toUpperCase()],
                    cell: (n) => <QualityBadge rating={n.quality_rating} />,
                },
                {
                    id: 'messagingLimit',
                    header: t('channels.numbers.messagingLimit'),
                    sortValue: (n) => n.messaging_limit || undefined,
                    cell: (n) => (
                        <div className="text-sm">
                            {n.messaging_limit || '—'}
                            {n.throughput_level && (
                                <div className="text-xs text-muted-foreground">
                                    {t('channels.numbers.throughput', {
                                        level: n.throughput_level,
                                    })}
                                </div>
                            )}
                        </div>
                    ),
                },
                {
                    id: 'throughputMps',
                    header: t('channels.numbers.throughputMps'),
                    sortValue: (n) => n.throughput_mps ?? undefined,
                    cell: (n) => (
                        <div
                            className="text-sm whitespace-nowrap"
                            title={
                                n.signals_checked_at
                                    ? t('channels.numbers.signalsChecked', {
                                          when: formatDateTime(
                                              n.signals_checked_at,
                                          ),
                                      })
                                    : undefined
                            }
                        >
                            {n.throughput_mps
                                ? t('channels.numbers.perSecond', {
                                      mps: n.throughput_mps,
                                  })
                                : '—'}
                        </div>
                    ),
                },
            ]}
            rowActions={(n) =>
                n.is_default ? (
                    <Badge>
                        <Star /> {t('channels.numbers.default')}
                    </Badge>
                ) : (
                    manage && (
                        <Button
                            size="sm"
                            variant="outline"
                            disabled={pending}
                            onClick={() => setDefault(n)}
                        >
                            {t('channels.numbers.setDefault')}
                        </Button>
                    )
                )
            }
            rowMenu={(n) => [
                [
                    {
                        id: 'copy',
                        label: t('contextMenu.copyPhone'),
                        icon: Phone,
                        run: () =>
                            void navigator.clipboard
                                .writeText(n.display_phone_number)
                                .then(
                                    () =>
                                        toast.success(t('contextMenu.copied')),
                                    () => undefined,
                                ),
                    },
                    ...(manage && !n.is_default
                        ? [
                              {
                                  id: 'default',
                                  label: t('channels.numbers.setDefault'),
                                  icon: Star,
                                  disabled: pending,
                                  run: () => setDefault(n),
                              },
                          ]
                        : []),
                ],
            ]}
        />
    );
}

function QualityBadge({ rating }: { rating: string }) {
    const t = useT();
    const r = rating.toUpperCase();
    if (!r || r === 'UNKNOWN')
        return (
            <Badge variant="outline">
                {t('channels.numbers.qualities.unknown')}
            </Badge>
        );
    const variant =
        r === 'RED' || r === 'LOW'
            ? 'destructive'
            : r === 'YELLOW' || r === 'MEDIUM'
              ? 'outline'
              : 'secondary';
    const key = ({ GREEN: 'high', YELLOW: 'medium', RED: 'low' } as const)[
        r as 'GREEN' | 'YELLOW' | 'RED'
    ];
    const label = key ? t(`channels.numbers.qualities.${key}`) : rating;
    return <Badge variant={variant}>{label}</Badge>;
}

// ── Webhook ───────────────────────────────────────────────────────────────

export function CopyField({
    id,
    label,
    copyLabel,
    value,
}: {
    id: string;
    label: string;
    copyLabel: string;
    value: string;
}) {
    const t = useT();
    return (
        <div className="space-y-1.5">
            <Label htmlFor={id}>{label}</Label>
            <div className="flex gap-2">
                <Input
                    id={id}
                    readOnly
                    value={value}
                    className="font-mono text-xs"
                />
                <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    aria-label={copyLabel}
                    onClick={() => {
                        navigator.clipboard.writeText(value).then(
                            () =>
                                toast.success(
                                    t('channels.webhook.copied', { label }),
                                ),
                            () => toast.error(t('channels.webhook.copyFailed')),
                        );
                    }}
                >
                    <Copy />
                </Button>
            </div>
        </div>
    );
}
