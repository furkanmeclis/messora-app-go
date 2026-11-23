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
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
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
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { useFormatDateTime } from '@/components/display-preferences';
import { useT } from '@/i18n/client';
import type {
    ActionResult,
    PhoneNumber,
    WebhookSetup,
    WhatsAppAccount,
} from '@/lib/types';
import {
    connectAccountAction,
    disconnectAccountAction,
    setDefaultNumberAction,
    syncAccountAction,
    updateAccountAction,
} from './actions';

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

export function ConnectAccountDialog() {
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
                <Button shortcut="n">
                    <Plus /> {t('channels.connect.open')}
                </Button>
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
                            inputMode="numeric"
                            pattern="[0-9]{1,32}"
                            title={t('channels.connect.digitsOnly')}
                            autoComplete="off"
                            error={fe('app_id')}
                        />
                        <TextField
                            label={t('channels.connect.appSecret')}
                            name="app_secret"
                            type="password"
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
            {a.last_error && (
                <Alert variant="destructive">
                    <AlertTriangle />
                    <AlertTitle>{t('channels.accounts.lastError')}</AlertTitle>
                    <AlertDescription>
                        <p>{a.last_error}</p>
                        <p>{t('channels.accounts.lastErrorHint')}</p>
                    </AlertDescription>
                </Alert>
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

function CredentialsDialog({ account }: { account: WhatsAppAccount }) {
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
        if (name === account.name && !token && !secret) {
            setOpen(false);
            return;
        }
        startTransition(async () => {
            const r = await guard(() =>
                updateAccountAction(account.id, {
                    name: name !== account.name ? name : undefined,
                    access_token: token || undefined,
                    app_secret: secret || undefined,
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
                <Button size="sm" variant="outline">
                    <KeyRound /> {t('channels.edit.open')}
                </Button>
            </DialogTrigger>
            <DialogContent>
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
                        shown={['name', 'access_token', 'app_secret']}
                    />
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

export function PhoneNumberTable({
    numbers,
    manage,
}: {
    numbers: PhoneNumber[];
    manage: boolean;
}) {
    const t = useT();
    const { guard } = useReauth();
    const [pending, startTransition] = useTransition();
    if (numbers.length === 0) {
        return (
            <p className="p-6 text-sm text-muted-foreground">
                {t('channels.numbers.empty')}
            </p>
        );
    }
    return (
        <Table>
            <TableHeader>
                <TableRow>
                    <TableHead>{t('channels.numbers.number')}</TableHead>
                    <TableHead>{t('channels.numbers.verifiedName')}</TableHead>
                    <TableHead>{t('channels.numbers.account')}</TableHead>
                    <TableHead>{t('channels.numbers.quality')}</TableHead>
                    <TableHead>
                        {t('channels.numbers.messagingLimit')}
                    </TableHead>
                    <TableHead>
                        <span className="sr-only">
                            {t('channels.numbers.default')}
                        </span>
                    </TableHead>
                </TableRow>
            </TableHeader>
            <TableBody>
                {numbers.map((n) => (
                    <TableRow key={n.id}>
                        <TableCell className="font-mono">
                            {n.display_phone_number}
                        </TableCell>
                        <TableCell>{n.verified_name || '—'}</TableCell>
                        <TableCell className="text-muted-foreground">
                            {n.account_name}
                        </TableCell>
                        <TableCell>
                            <QualityBadge rating={n.quality_rating} />
                        </TableCell>
                        <TableCell className="text-sm">
                            {n.messaging_limit || '—'}
                            {n.throughput_level && (
                                <div className="text-xs text-muted-foreground">
                                    {t('channels.numbers.throughput', {
                                        level: n.throughput_level,
                                    })}
                                </div>
                            )}
                        </TableCell>
                        <TableCell className="text-right">
                            {n.is_default ? (
                                <Badge>
                                    <Star /> {t('channels.numbers.default')}
                                </Badge>
                            ) : (
                                manage && (
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        disabled={pending}
                                        onClick={() =>
                                            startTransition(async () => {
                                                notify(
                                                    await guard(() =>
                                                        setDefaultNumberAction(
                                                            n.id,
                                                        ),
                                                    ),
                                                );
                                            })
                                        }
                                    >
                                        {t('channels.numbers.setDefault')}
                                    </Button>
                                )
                            )}
                        </TableCell>
                    </TableRow>
                ))}
            </TableBody>
        </Table>
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

function CopyField({
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

export function WebhookCard({ setup }: { setup: WebhookSetup }) {
    const t = useT();
    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('channels.webhook.title')}</CardTitle>
                <CardDescription>
                    {t('channels.webhook.description')}
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
                {!setup.signature_verification && (
                    <Alert
                        variant="destructive"
                        data-testid="signature-warning"
                    >
                        <AlertTriangle />
                        <AlertTitle>
                            {t('channels.webhook.signatureOff')}
                        </AlertTitle>
                        <AlertDescription>
                            {t('channels.webhook.signatureOffHint')}
                        </AlertDescription>
                    </Alert>
                )}
                <CopyField
                    id="webhook-callback"
                    label={t('channels.webhook.callbackUrl')}
                    copyLabel={t('channels.webhook.copyCallbackUrl')}
                    value={setup.callback_url}
                />
                <CopyField
                    id="webhook-verify-token"
                    label={t('channels.webhook.verifyToken')}
                    copyLabel={t('channels.webhook.copyVerifyToken')}
                    value={setup.verify_token}
                />
                {setup.fields && setup.fields.length > 0 && (
                    <div className="space-y-1.5">
                        <p className="text-sm font-medium">
                            {t('channels.webhook.fields')}
                        </p>
                        <div className="flex flex-wrap gap-1">
                            {setup.fields.map((f) => (
                                <Badge
                                    key={f}
                                    variant="outline"
                                    className="font-mono"
                                >
                                    {f}
                                </Badge>
                            ))}
                        </div>
                    </div>
                )}
            </CardContent>
        </Card>
    );
}
