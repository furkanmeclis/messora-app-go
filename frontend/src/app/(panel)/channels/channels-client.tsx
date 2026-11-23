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
    { label: string; variant: 'secondary' | 'destructive' | 'outline' }
> = {
    active: { label: 'Active', variant: 'secondary' },
    error: { label: 'Error', variant: 'destructive' },
    disabled: { label: 'Disabled', variant: 'outline' },
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
                <Button>
                    <Plus /> Connect account
                </Button>
            </DialogTrigger>
            <DialogContent className="max-h-[90svh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>
                        Connect a WhatsApp Business account
                    </DialogTitle>
                    <DialogDescription>
                        Use a permanent system-user token with the
                        whatsapp_business_messaging and
                        whatsapp_business_management permissions. Messora checks
                        it with Meta before saving anything.
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
                            label="Name"
                            name="name"
                            required
                            maxLength={120}
                            placeholder="Main brand"
                            error={fe('name')}
                        />
                        <TextField
                            label="WhatsApp Business Account ID"
                            name="waba_id"
                            required
                            inputMode="numeric"
                            pattern="[0-9]{1,32}"
                            title="Digits only"
                            autoComplete="off"
                            error={fe('waba_id')}
                            hint="Digits only, from Meta Business Suite → WhatsApp accounts."
                        />
                        <TextField
                            label="Access token"
                            name="access_token"
                            type="password"
                            required
                            minLength={20}
                            maxLength={1024}
                            autoComplete="off"
                            error={fe('access_token')}
                            hint="Stored encrypted; it is never displayed again."
                        />
                        <TextField
                            label="App ID (optional)"
                            name="app_id"
                            inputMode="numeric"
                            pattern="[0-9]{1,32}"
                            title="Digits only"
                            autoComplete="off"
                            error={fe('app_id')}
                        />
                        <TextField
                            label="App secret (recommended)"
                            name="app_secret"
                            type="password"
                            minLength={8}
                            maxLength={256}
                            autoComplete="off"
                            error={fe('app_secret')}
                            hint="Needed to verify webhook signatures. Without it every webhook from Meta is rejected."
                        />
                    </FieldGroup>
                    <Button type="submit" className="w-full" disabled={pending}>
                        {pending && <Spinner />} Connect
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
    if (accounts.length === 0) {
        return (
            <Empty className="py-10">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <Phone />
                    </EmptyMedia>
                    <EmptyTitle>No WhatsApp account connected</EmptyTitle>
                    <EmptyDescription>
                        {manage
                            ? 'Connect a WhatsApp Business account to send messages, templates and campaigns.'
                            : 'Ask an administrator to connect a WhatsApp Business account.'}
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }
    return (
        <ul className="divide-y" aria-label="WhatsApp accounts">
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
    const formatDateTime = useFormatDateTime();
    const { guard } = useReauth();
    const [pending, startTransition] = useTransition();
    const [syncResult, setSyncResult] = useState<string | null>(null);
    const badge = STATUS_BADGE[a.status];

    const sync = () =>
        startTransition(async () => {
            const r = await guard(() => syncAccountAction(a.id));
            if (r.ok) {
                const text = `Synced ${r.data.phone_numbers} phone number(s) and ${r.data.templates} template(s).`;
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
                        <Badge variant={badge.variant}>{badge.label}</Badge>
                        {!a.app_secret_configured && (
                            <Badge variant="outline">No app secret</Badge>
                        )}
                    </div>
                    <dl className="grid grid-cols-[auto_1fr] gap-x-3 text-sm text-muted-foreground">
                        <dt>WABA ID</dt>
                        <dd className="font-mono">{a.waba_id}</dd>
                        {a.app_id && (
                            <>
                                <dt>App ID</dt>
                                <dd className="font-mono">{a.app_id}</dd>
                            </>
                        )}
                        <dt>Last synced</dt>
                        <dd>{formatDateTime(a.last_synced_at)}</dd>
                        <dt>Connected</dt>
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
                            {pending ? <Spinner /> : <RefreshCw />} Sync
                        </Button>
                        <CredentialsDialog account={a} />
                        <Button
                            size="sm"
                            variant="outline"
                            disabled={pending}
                            onClick={toggle}
                        >
                            {a.status === 'disabled' ? 'Enable' : 'Disable'}
                        </Button>
                        <DisconnectButton account={a} />
                    </div>
                )}
            </div>
            {a.last_error && (
                <Alert variant="destructive">
                    <AlertTriangle />
                    <AlertTitle>Last error</AlertTitle>
                    <AlertDescription>
                        <p>{a.last_error}</p>
                        <p>
                            Sending fails fast until a successful sync or new
                            credentials.
                        </p>
                    </AlertDescription>
                </Alert>
            )}
            {syncResult && (
                <p className="text-sm text-muted-foreground" role="status">
                    {syncResult}{' '}
                    <Link className="underline" href="/templates">
                        View templates
                    </Link>
                </p>
            )}
        </li>
    );
}

function CredentialsDialog({ account }: { account: WhatsAppAccount }) {
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
                    <KeyRound /> Edit / rotate
                </Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Edit {account.name}</DialogTitle>
                    <DialogDescription>
                        Leave a secret empty to keep the current one. A new
                        token is checked with Meta first.
                    </DialogDescription>
                </DialogHeader>
                <form onSubmit={submit} className="space-y-4">
                    <ErrorBanner
                        error={error}
                        shown={['name', 'access_token', 'app_secret']}
                    />
                    <FieldGroup>
                        <TextField
                            label="Name"
                            name="name"
                            required
                            maxLength={120}
                            defaultValue={account.name}
                            id={`acc-name-${account.id}`}
                            error={fe('name')}
                        />
                        <TextField
                            label="New access token"
                            name="access_token"
                            type="password"
                            minLength={20}
                            maxLength={1024}
                            autoComplete="off"
                            id={`acc-token-${account.id}`}
                            error={fe('access_token')}
                        />
                        <TextField
                            label="New app secret"
                            name="app_secret"
                            type="password"
                            minLength={8}
                            maxLength={256}
                            autoComplete="off"
                            id={`acc-secret-${account.id}`}
                            error={fe('app_secret')}
                            hint={
                                account.app_secret_configured
                                    ? 'An app secret is configured.'
                                    : 'No app secret yet: webhooks are rejected until you add one.'
                            }
                        />
                    </FieldGroup>
                    <Button type="submit" className="w-full" disabled={pending}>
                        {pending && <Spinner />} Save
                    </Button>
                </form>
            </DialogContent>
        </Dialog>
    );
}

function DisconnectButton({ account }: { account: WhatsAppAccount }) {
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
                Disconnect
            </Button>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle>
                        {blocked
                            ? 'This account cannot be removed'
                            : `Disconnect ${account.name}?`}
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                        {blocked
                            ? `${blocked} Conversations and delivery history refer to its numbers, so it stays; a disabled account sends nothing and can be enabled again later.`
                            : 'Its phone numbers and templates are removed from Messora. Accounts with message history cannot be removed; disable them instead.'}
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <AlertDialogCancel disabled={pending}>
                        Cancel
                    </AlertDialogCancel>
                    {blocked ? (
                        account.status !== 'disabled' && (
                            <Button onClick={disable} disabled={pending}>
                                {pending && <Spinner />} Disable instead
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
                            {pending && <Spinner />} Disconnect
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
    const { guard } = useReauth();
    const [pending, startTransition] = useTransition();
    if (numbers.length === 0) {
        return (
            <p className="p-6 text-sm text-muted-foreground">
                No phone numbers yet. They are imported when an account is
                connected or synced.
            </p>
        );
    }
    return (
        <Table>
            <TableHeader>
                <TableRow>
                    <TableHead>Number</TableHead>
                    <TableHead>Verified name</TableHead>
                    <TableHead>Account</TableHead>
                    <TableHead>Quality</TableHead>
                    <TableHead>Messaging limit</TableHead>
                    <TableHead>
                        <span className="sr-only">Default</span>
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
                                    Throughput: {n.throughput_level}
                                </div>
                            )}
                        </TableCell>
                        <TableCell className="text-right">
                            {n.is_default ? (
                                <Badge>
                                    <Star /> Default
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
                                        Set default
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
    const r = rating.toUpperCase();
    if (!r || r === 'UNKNOWN') return <Badge variant="outline">Unknown</Badge>;
    const variant =
        r === 'RED' || r === 'LOW'
            ? 'destructive'
            : r === 'YELLOW' || r === 'MEDIUM'
              ? 'outline'
              : 'secondary';
    const label = { GREEN: 'High', YELLOW: 'Medium', RED: 'Low' }[r] ?? rating;
    return <Badge variant={variant}>{label}</Badge>;
}

// ── Webhook ───────────────────────────────────────────────────────────────

function CopyField({
    id,
    label,
    value,
}: {
    id: string;
    label: string;
    value: string;
}) {
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
                    aria-label={`Copy ${label.toLowerCase()}`}
                    onClick={() => {
                        navigator.clipboard.writeText(value).then(
                            () => toast.success(`${label} copied`),
                            () =>
                                toast.error(
                                    'Copying is not allowed here; select the text instead.',
                                ),
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
    return (
        <Card>
            <CardHeader>
                <CardTitle>Webhook setup</CardTitle>
                <CardDescription>
                    Enter these values in the Meta App dashboard under WhatsApp
                    → Configuration. The callback URL must be reachable over
                    public HTTPS.
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
                {!setup.signature_verification && (
                    <Alert
                        variant="destructive"
                        data-testid="signature-warning"
                    >
                        <AlertTriangle />
                        <AlertTitle>Signature verification is off</AlertTitle>
                        <AlertDescription>
                            No app secret is configured, so every webhook from
                            Meta is rejected: no incoming messages and no
                            delivery updates. Add the app secret to an account
                            (Edit / rotate) or set META_APP_SECRET.
                        </AlertDescription>
                    </Alert>
                )}
                <CopyField
                    id="webhook-callback"
                    label="Callback URL"
                    value={setup.callback_url}
                />
                <CopyField
                    id="webhook-verify-token"
                    label="Verify token"
                    value={setup.verify_token}
                />
                {setup.fields && setup.fields.length > 0 && (
                    <div className="space-y-1.5">
                        <p className="text-sm font-medium">
                            Subscribe to these fields
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
