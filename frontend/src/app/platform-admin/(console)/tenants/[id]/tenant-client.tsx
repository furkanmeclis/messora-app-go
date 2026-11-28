'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import {
    CheckCircle2,
    Clock,
    ExternalLink,
    Globe,
    LifeBuoy,
    PauseCircle,
    PlayCircle,
    Plus,
    Star,
    Trash2,
    UserPlus,
} from 'lucide-react';
import { toast } from 'sonner';
import { useConfirm } from '@/components/confirm';
import { notify } from '@/components/notify';
import { useFormatDateTime } from '@/components/display-preferences';
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
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { DateTimePicker } from '@/components/ui/date-picker';
import { Label } from '@/components/ui/label';
import { NumberField } from '@/components/ui/number-field';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';

import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import {
    addDomainAction,
    domainAction,
    inviteOwnerAction,
    startSupportAction,
    suspendTenantAction,
    unsuspendTenantAction,
    updateTenantAction,
} from '../../../actions';
import { SubscriptionBadge, TenantStatusBadge } from '../../badges';
import { CopyField } from '../../copy-field';

type Tenant = Schemas['PlatformTenantDetail'];

/** datetime-local value (local time) of an ISO instant. */
function localInput(iso: string | null | undefined): string {
    if (!iso) return '';
    const d = new Date(iso);
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function TenantDetail({ tenant }: { tenant: Tenant }) {
    return (
        <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]">
            <div className="grid gap-6">
                <Details tenant={tenant} />
                <Domains tenant={tenant} />
            </div>
            <div className="grid gap-6">
                <StatusCard tenant={tenant} />
                <Owners tenant={tenant} />
                <Support tenant={tenant} />
            </div>
        </div>
    );
}

function Details({ tenant }: { tenant: Tenant }) {
    const t = useT();
    const router = useRouter();
    const fmt = useFormatDateTime();
    const [name, setName] = useState(tenant.name);
    const [trial, setTrial] = useState(localInput(tenant.trial_ends_at));
    const [sub, setSub] = useState(tenant.subscription_status);
    const [pending, start] = useTransition();
    const save = (body: Schemas['UpdatePlatformTenantRequest']) =>
        start(async () => {
            if (notify(await updateTenantAction(tenant.id, body)))
                router.refresh();
        });
    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('platform.tenant.details')}</CardTitle>
                <CardDescription className="flex flex-wrap items-center gap-2">
                    <TenantStatusBadge status={tenant.status} />
                    <SubscriptionBadge status={tenant.subscription_status} />
                    <span>
                        {t('platform.tenant.members', {
                            count: tenant.member_count,
                        })}
                    </span>
                    <span>· {fmt(tenant.created_at)}</span>
                </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-5">
                <form
                    className="grid gap-2"
                    onSubmit={(e) => {
                        e.preventDefault();
                        save({ name: name.trim() });
                    }}
                >
                    <Label htmlFor="tenant-name">
                        {t('platform.tenants.name')}
                    </Label>
                    <div className="flex gap-2">
                        <Input
                            id="tenant-name"
                            value={name}
                            maxLength={120}
                            onChange={(e) => setName(e.target.value)}
                        />
                        <Button
                            type="submit"
                            variant="outline"
                            disabled={
                                pending ||
                                !name.trim() ||
                                name.trim() === tenant.name
                            }
                        >
                            {t('platform.tenant.save')}
                        </Button>
                    </div>
                </form>
                <div className="grid gap-2">
                    <Label htmlFor="tenant-trial">
                        {t('platform.tenant.trialEnd')}
                    </Label>
                    <div className="flex flex-wrap gap-2">
                        <DateTimePicker
                            id="tenant-trial"
                            value={trial}
                            onValueChange={setTrial}
                        />
                        <Button
                            variant="outline"
                            disabled={pending || !trial}
                            onClick={() =>
                                save({
                                    trial_ends_at: new Date(
                                        trial,
                                    ).toISOString(),
                                })
                            }
                        >
                            {t('platform.tenant.save')}
                        </Button>
                        <Button
                            variant="ghost"
                            disabled={pending || !tenant.trial_ends_at}
                            onClick={() => {
                                setTrial('');
                                save({ trial_ends_at: null });
                            }}
                        >
                            {t('platform.tenant.clearTrial')}
                        </Button>
                    </div>
                    {!tenant.trial_ends_at && (
                        <p className="text-xs text-muted-foreground">
                            {t('platform.tenant.noTrial')}
                        </p>
                    )}
                </div>
                <div className="grid gap-2">
                    <Label htmlFor="tenant-sub">
                        {t('platform.tenant.subscription')}
                    </Label>
                    <div className="flex gap-2">
                        <Select
                            value={sub}
                            onValueChange={(v) =>
                                setSub(v as Schemas['SubscriptionStatus'])
                            }
                        >
                            <SelectTrigger id="tenant-sub" className="w-44">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {(
                                    [
                                        'trial',
                                        'active',
                                        'past_due',
                                        'read_only',
                                        'canceled',
                                    ] as const
                                ).map((s) => (
                                    <SelectItem key={s} value={s}>
                                        {t(
                                            `platform.tenants.subscription.${s}`,
                                        )}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                        <Button
                            variant="outline"
                            disabled={
                                pending || sub === tenant.subscription_status
                            }
                            onClick={() => save({ subscription_status: sub })}
                        >
                            {t('platform.tenant.save')}
                        </Button>
                    </div>
                </div>
            </CardContent>
        </Card>
    );
}

function StatusCard({ tenant }: { tenant: Tenant }) {
    const t = useT();
    const router = useRouter();
    const [open, setOpen] = useState(false);
    const [reason, setReason] = useState('');
    const [pending, start] = useTransition();
    const suspended = tenant.status === 'suspended';
    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    <TenantStatusBadge status={tenant.status} />
                </CardTitle>
                {suspended && tenant.suspended_reason && (
                    <CardDescription data-testid="suspended-reason">
                        {t('platform.tenant.suspended', {
                            reason: tenant.suspended_reason,
                        })}
                    </CardDescription>
                )}
            </CardHeader>
            <CardContent className="flex flex-wrap gap-2">
                {tenant.url && !suspended && (
                    <Button asChild variant="outline">
                        <a href={tenant.url} target="_blank" rel="noreferrer">
                            <ExternalLink /> {t('platform.tenant.open')}
                        </a>
                    </Button>
                )}
                {suspended ? (
                    <Button
                        variant="outline"
                        disabled={pending}
                        onClick={() =>
                            start(async () => {
                                if (
                                    notify(
                                        await unsuspendTenantAction(tenant.id),
                                    )
                                )
                                    router.refresh();
                            })
                        }
                    >
                        <PlayCircle /> {t('platform.tenant.unsuspend')}
                    </Button>
                ) : (
                    tenant.status === 'active' && (
                        <Dialog open={open} onOpenChange={setOpen}>
                            <DialogTrigger asChild>
                                <Button variant="destructive">
                                    <PauseCircle />{' '}
                                    {t('platform.tenant.suspend')}
                                </Button>
                            </DialogTrigger>
                            <DialogContent>
                                <DialogHeader>
                                    <DialogTitle>
                                        {t('platform.tenant.suspendTitle', {
                                            name: tenant.name,
                                        })}
                                    </DialogTitle>
                                    <DialogDescription>
                                        {t('platform.tenant.suspendBody')}
                                    </DialogDescription>
                                </DialogHeader>
                                <div className="grid gap-2">
                                    <Label htmlFor="suspend-reason">
                                        {t('platform.tenant.reason')}
                                    </Label>
                                    <Textarea
                                        id="suspend-reason"
                                        value={reason}
                                        maxLength={500}
                                        onChange={(e) =>
                                            setReason(e.target.value)
                                        }
                                    />
                                </div>
                                <DialogFooter>
                                    <Button
                                        variant="destructive"
                                        disabled={
                                            pending || reason.trim().length < 3
                                        }
                                        onClick={() =>
                                            start(async () => {
                                                if (
                                                    notify(
                                                        await suspendTenantAction(
                                                            tenant.id,
                                                            reason.trim(),
                                                        ),
                                                    )
                                                ) {
                                                    setOpen(false);
                                                    setReason('');
                                                    router.refresh();
                                                }
                                            })
                                        }
                                    >
                                        {t('platform.tenant.suspend')}
                                    </Button>
                                </DialogFooter>
                            </DialogContent>
                        </Dialog>
                    )
                )}
            </CardContent>
        </Card>
    );
}

function Owners({ tenant }: { tenant: Tenant }) {
    const t = useT();
    const router = useRouter();
    const [open, setOpen] = useState(false);
    const [email, setEmail] = useState('');
    const [link, setLink] = useState<string | null>(null);
    const [pending, start] = useTransition();
    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('platform.tenant.owners')}</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-3">
                {tenant.owners.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        {t('platform.tenant.noOwners')}
                    </p>
                ) : (
                    <ul className="grid gap-1.5 text-sm">
                        {tenant.owners.map((o) => (
                            <li key={o.id}>
                                <span className="font-medium">{o.name}</span>{' '}
                                <span className="text-muted-foreground">
                                    {o.email}
                                </span>
                            </li>
                        ))}
                    </ul>
                )}
                <Dialog
                    open={open}
                    onOpenChange={(o) => {
                        setOpen(o);
                        if (!o) {
                            setEmail('');
                            setLink(null);
                        }
                    }}
                >
                    <DialogTrigger asChild>
                        <Button variant="outline" className="w-fit">
                            <UserPlus /> {t('platform.tenant.inviteOwner')}
                        </Button>
                    </DialogTrigger>
                    <DialogContent>
                        <DialogHeader>
                            <DialogTitle>
                                {t('platform.tenant.inviteTitle')}
                            </DialogTitle>
                        </DialogHeader>
                        {link ? (
                            <div className="grid gap-2">
                                <Label>
                                    {t('platform.tenants.acceptLink')}
                                </Label>
                                <CopyField
                                    value={link}
                                    testId="owner-accept-url"
                                />
                                <p className="text-xs text-muted-foreground">
                                    {t('platform.tenants.acceptLinkHint')}
                                </p>
                            </div>
                        ) : (
                            <form
                                id="invite-owner"
                                className="grid gap-2"
                                onSubmit={(e) => {
                                    e.preventDefault();
                                    start(async () => {
                                        const r = await inviteOwnerAction(
                                            tenant.id,
                                            email.trim(),
                                        );
                                        if (notify(r)) {
                                            setLink(r.data.accept_url);
                                            router.refresh();
                                        }
                                    });
                                }}
                            >
                                <Label htmlFor="owner-email">
                                    {t('auth.email')}
                                </Label>
                                <Input
                                    id="owner-email"
                                    type="email"
                                    required
                                    value={email}
                                    onChange={(e) => setEmail(e.target.value)}
                                />
                            </form>
                        )}
                        <DialogFooter>
                            {link ? (
                                <Button onClick={() => setOpen(false)}>
                                    {t('common.close')}
                                </Button>
                            ) : (
                                <Button
                                    type="submit"
                                    form="invite-owner"
                                    disabled={pending || !email}
                                >
                                    {t('platform.tenant.inviteOwner')}
                                </Button>
                            )}
                        </DialogFooter>
                    </DialogContent>
                </Dialog>
            </CardContent>
        </Card>
    );
}

function Domains({ tenant }: { tenant: Tenant }) {
    const t = useT();
    const router = useRouter();
    const ask = useConfirm();
    const fmt = useFormatDateTime();
    const [host, setHost] = useState('');
    const [pending, start] = useTransition();
    const run = (id: string, op: 'verify' | 'primary' | 'delete') =>
        start(async () => {
            if (notify(await domainAction(id, op))) router.refresh();
        });
    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    <Globe className="size-4 text-primary" />
                    {t('platform.tenant.domains')}
                </CardTitle>
            </CardHeader>
            <CardContent className="grid gap-4">
                <ul className="grid gap-3">
                    {tenant.domains.map((d) => (
                        <li
                            key={d.id}
                            className="rounded-lg border p-3"
                            data-testid="domain-row"
                        >
                            <div className="flex flex-wrap items-center gap-2">
                                <span className="font-mono text-sm">
                                    {d.hostname}
                                </span>
                                <Badge variant="outline">
                                    {d.kind === 'subdomain'
                                        ? t('platform.tenant.subdomain')
                                        : t('platform.tenant.custom')}
                                </Badge>
                                {d.is_primary && (
                                    <Badge>
                                        {t('platform.tenant.primary')}
                                    </Badge>
                                )}
                                {d.verified_at ? (
                                    <Badge
                                        variant="outline"
                                        className="border-success/40 bg-success/10 text-success"
                                    >
                                        <CheckCircle2 />{' '}
                                        {t('platform.tenant.verified')}
                                    </Badge>
                                ) : (
                                    <Badge
                                        variant="outline"
                                        className="border-warning/40 bg-warning/10 text-warning"
                                    >
                                        <Clock /> {t('platform.tenant.pending')}
                                    </Badge>
                                )}
                                <div className="ml-auto flex flex-wrap gap-1.5">
                                    {!d.verified_at && (
                                        <Button
                                            size="sm"
                                            variant="outline"
                                            disabled={pending}
                                            onClick={() => run(d.id, 'verify')}
                                        >
                                            {t('platform.tenant.verify')}
                                        </Button>
                                    )}
                                    {d.verified_at && !d.is_primary && (
                                        <Button
                                            size="sm"
                                            variant="outline"
                                            disabled={pending}
                                            onClick={() => run(d.id, 'primary')}
                                        >
                                            <Star />{' '}
                                            {t('platform.tenant.makePrimary')}
                                        </Button>
                                    )}
                                    {d.kind === 'custom' && !d.is_primary && (
                                        <Button
                                            size="sm"
                                            variant="ghost"
                                            disabled={pending}
                                            aria-label={t(
                                                'platform.tenant.remove',
                                            )}
                                            onClick={async () => {
                                                const ok = await ask({
                                                    title: t(
                                                        'platform.tenant.removeTitle',
                                                        { domain: d.hostname },
                                                    ),
                                                    description: t(
                                                        'platform.tenant.removeBody',
                                                    ),
                                                    confirmLabel: t(
                                                        'platform.tenant.remove',
                                                    ),
                                                    destructive: true,
                                                });
                                                if (ok) run(d.id, 'delete');
                                            }}
                                        >
                                            <Trash2 />
                                        </Button>
                                    )}
                                </div>
                            </div>
                            {d.verification && !d.verified_at && (
                                <div className="mt-3 grid gap-2 text-sm">
                                    <p className="text-muted-foreground">
                                        {t('platform.tenant.txtHint')}
                                    </p>
                                    <div className="grid gap-2 sm:grid-cols-[auto_1fr] sm:items-center">
                                        <span className="font-mono text-xs">
                                            TXT
                                        </span>
                                        <CopyField
                                            value={d.verification.name}
                                        />
                                        <span />
                                        <CopyField
                                            value={d.verification.value}
                                            testId="txt-value"
                                        />
                                    </div>
                                    {d.last_check_error && (
                                        <p className="text-xs text-destructive">
                                            {d.last_check_error}
                                            {d.last_checked_at &&
                                                ` · ${fmt(d.last_checked_at)}`}
                                        </p>
                                    )}
                                </div>
                            )}
                        </li>
                    ))}
                </ul>
                <form
                    className="flex flex-wrap gap-2"
                    onSubmit={(e) => {
                        e.preventDefault();
                        start(async () => {
                            const r = await addDomainAction(
                                tenant.id,
                                host.trim(),
                            );
                            if (notify(r)) {
                                setHost('');
                                router.refresh();
                            }
                        });
                    }}
                >
                    <Input
                        value={host}
                        onChange={(e) => setHost(e.target.value)}
                        placeholder={t('platform.tenant.domainHint')}
                        aria-label={t('platform.tenant.domainName')}
                        className="min-w-56 flex-1 font-mono"
                    />
                    <Button
                        type="submit"
                        variant="outline"
                        disabled={pending || !host.trim()}
                    >
                        <Plus /> {t('platform.tenant.addDomain')}
                    </Button>
                </form>
            </CardContent>
        </Card>
    );
}

function Support({ tenant }: { tenant: Tenant }) {
    const t = useT();
    const router = useRouter();
    const [reason, setReason] = useState('');
    const [minutes, setMinutes] = useState(30);
    const [write, setWrite] = useState(false);
    const [pending, start] = useTransition();
    if (tenant.status !== 'active') return null;
    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    <LifeBuoy className="size-4 text-primary" />
                    {t('platform.tenant.supportTitle')}
                </CardTitle>
                <CardDescription>
                    {t('platform.tenant.supportBody')}
                </CardDescription>
            </CardHeader>
            <CardContent>
                <form
                    className="grid gap-4"
                    onSubmit={(e) => {
                        e.preventDefault();
                        // Opened before the await so the browser allows it.
                        const tab = window.open('about:blank', '_blank');
                        start(async () => {
                            const r = await startSupportAction(tenant.id, {
                                reason: reason.trim(),
                                minutes,
                                write,
                            });
                            if (!r.ok) {
                                tab?.close();
                                notify(r);
                                return;
                            }
                            if (tab) tab.location.href = r.data.redirect_url;
                            else window.location.href = r.data.redirect_url;
                            toast.success(t('platform.tenant.supportStarted'));
                            setReason('');
                            router.refresh();
                        });
                    }}
                >
                    <div className="grid gap-2">
                        <Label htmlFor="support-reason">
                            {t('platform.tenant.supportReason')}
                        </Label>
                        <Textarea
                            id="support-reason"
                            value={reason}
                            maxLength={500}
                            onChange={(e) => setReason(e.target.value)}
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="support-minutes">
                            {t('platform.tenant.supportMinutes')}
                        </Label>
                        <NumberField
                            id="support-minutes"
                            min={5}
                            max={240}
                            value={minutes}
                            onValueChange={(v) => setMinutes(v ?? 30)}
                            className="w-28"
                        />
                    </div>
                    <Label className="flex items-center gap-2 font-normal">
                        <Switch checked={write} onCheckedChange={setWrite} />
                        {t('platform.tenant.supportWrite')}
                    </Label>
                    <Button
                        type="submit"
                        disabled={pending || reason.trim().length < 3}
                        className="w-fit"
                    >
                        <ExternalLink /> {t('platform.tenant.supportStart')}
                    </Button>
                </form>
            </CardContent>
        </Card>
    );
}
