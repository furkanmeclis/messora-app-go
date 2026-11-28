'use client';

import { useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Plus, Search } from 'lucide-react';
import { notify } from '@/components/notify';
import { useFormatDateTime } from '@/components/display-preferences';
import { Button } from '@/components/ui/button';
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
import { Label } from '@/components/ui/label';
import { NumberField } from '@/components/ui/number-field';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';

import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { useT } from '@/i18n/client';
import { sanitizeSlugInput, slugify } from '@/lib/auth/slug';
import type { Schemas } from '@/lib/types';
import { createTenantAction, loadTenantsAction } from '../../actions';
import { SubscriptionBadge, TenantStatusBadge } from '../badges';
import { CopyField } from '../copy-field';

type Tenant = Schemas['PlatformTenant'];

export function TenantList({
    initial,
    initialCursor,
    query,
    status,
}: {
    initial: Tenant[];
    initialCursor?: string;
    query: string;
    status: string;
}) {
    const t = useT();
    const router = useRouter();
    const fmt = useFormatDateTime();
    const [rows, setRows] = useState(initial);
    const [cursor, setCursor] = useState(initialCursor);
    const [loading, start] = useTransition();
    const [q, setQ] = useState(query);
    const [st, setSt] = useState(status);

    const search = (nextQ: string, nextStatus: string) => {
        const p = new URLSearchParams();
        if (nextQ) p.set('q', nextQ);
        if (nextStatus) p.set('status', nextStatus);
        router.push(`/tenants${p.size ? '?' + p : ''}`);
    };
    const more = () =>
        start(async () => {
            if (!cursor) return;
            const p = new URLSearchParams({ limit: '50', cursor });
            if (query) p.set('q', query);
            if (status) p.set('status', status);
            const r = await loadTenantsAction(p.toString());
            if (!notify(r)) return;
            setRows((x) => [...x, ...r.data.rows]);
            setCursor(r.data.next);
        });

    return (
        <div className="space-y-4">
            <form
                className="flex flex-wrap gap-2"
                onSubmit={(e) => {
                    e.preventDefault();
                    search(q.trim(), st);
                }}
            >
                <div className="relative min-w-64 flex-1">
                    <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        name="q"
                        value={q}
                        onChange={(e) => setQ(e.target.value)}
                        placeholder={t('platform.tenants.search')}
                        className="pl-8"
                        aria-label={t('platform.tenants.search')}
                    />
                </div>
                <Select
                    value={st || 'all'}
                    onValueChange={(v) => {
                        const next = v === 'all' ? '' : v;
                        setSt(next);
                        search(q.trim(), next);
                    }}
                >
                    <SelectTrigger
                        className="w-44"
                        aria-label={t('platform.tenants.columns.status')}
                    >
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="all">
                            {t('platform.tenants.allStatuses')}
                        </SelectItem>
                        {(['active', 'suspended', 'deleted'] as const).map(
                            (s) => (
                                <SelectItem key={s} value={s}>
                                    {t(`platform.tenants.status.${s}`)}
                                </SelectItem>
                            ),
                        )}
                    </SelectContent>
                </Select>
                <Button type="submit" variant="outline">
                    {t('common.search')}
                </Button>
            </form>
            <div className="overflow-x-auto rounded-xl border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>
                                {t('platform.tenants.columns.name')}
                            </TableHead>
                            <TableHead>
                                {t('platform.tenants.columns.address')}
                            </TableHead>
                            <TableHead>
                                {t('platform.tenants.columns.status')}
                            </TableHead>
                            <TableHead>
                                {t('platform.tenants.columns.subscription')}
                            </TableHead>
                            <TableHead className="text-right">
                                {t('platform.tenants.columns.members')}
                            </TableHead>
                            <TableHead>
                                {t('platform.tenants.columns.created')}
                            </TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {rows.length === 0 && (
                            <TableRow>
                                <TableCell
                                    colSpan={6}
                                    className="py-10 text-center text-muted-foreground"
                                >
                                    {t('platform.tenants.empty')}
                                </TableCell>
                            </TableRow>
                        )}
                        {rows.map((r) => (
                            <TableRow key={r.id} data-testid="tenant-row">
                                <TableCell>
                                    <Link
                                        href={`/tenants/${r.id}`}
                                        className="font-medium hover:underline"
                                    >
                                        {r.name}
                                    </Link>
                                    <div className="text-xs text-muted-foreground">
                                        {r.slug}
                                    </div>
                                </TableCell>
                                <TableCell className="font-mono text-xs">
                                    {r.primary_host ?? '—'}
                                </TableCell>
                                <TableCell>
                                    <TenantStatusBadge status={r.status} />
                                </TableCell>
                                <TableCell>
                                    <SubscriptionBadge
                                        status={r.subscription_status}
                                    />
                                    {r.trial_ends_at && (
                                        <div className="mt-1 text-xs text-muted-foreground">
                                            {t('platform.tenants.trialEnds', {
                                                date: fmt(r.trial_ends_at),
                                            })}
                                        </div>
                                    )}
                                </TableCell>
                                <TableCell className="text-right tabular-nums">
                                    {r.member_count}
                                </TableCell>
                                <TableCell className="text-sm whitespace-nowrap text-muted-foreground">
                                    {fmt(r.created_at)}
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </div>
            {cursor && (
                <Button variant="outline" onClick={more} disabled={loading}>
                    {t('platform.tenants.more')}
                </Button>
            )}
        </div>
    );
}

export function CreateTenant({ baseDomain }: { baseDomain: string }) {
    const t = useT();
    const router = useRouter();
    const [open, setOpen] = useState(false);
    const [name, setName] = useState('');
    const [slug, setSlug] = useState('');
    const [slugEdited, setSlugEdited] = useState(false);
    const [owner, setOwner] = useState('');
    const [trial, setTrial] = useState(14);
    const [locale, setLocale] = useState<'tr' | 'en'>('tr');
    const [created, setCreated] = useState<
        Schemas['PlatformTenantCreated'] | null
    >(null);
    const [pending, start] = useTransition();
    const effectiveSlug = slugEdited ? slug : slugify(name);

    const reset = () => {
        setName('');
        setSlug('');
        setSlugEdited(false);
        setOwner('');
        setTrial(14);
        setCreated(null);
    };
    const submit = () =>
        start(async () => {
            const r = await createTenantAction({
                name: name.trim(),
                slug: effectiveSlug,
                owner_email: owner.trim() || undefined,
                trial_days: trial,
                locale,
            });
            if (!notify(r)) return;
            setCreated(r.data);
            router.refresh();
        });

    return (
        <Dialog
            open={open}
            onOpenChange={(o) => {
                setOpen(o);
                if (!o) reset();
            }}
        >
            <DialogTrigger asChild>
                <Button>
                    <Plus /> {t('platform.tenants.create')}
                </Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>
                        {t('platform.tenants.createTitle')}
                    </DialogTitle>
                    <DialogDescription>
                        {t('platform.tenants.createDescription')}
                    </DialogDescription>
                </DialogHeader>
                {created ? (
                    <div className="space-y-4">
                        <p className="text-sm">
                            <Link
                                href={`/tenants/${created.tenant.id}`}
                                className="font-medium text-primary hover:underline"
                            >
                                {created.tenant.name}
                            </Link>{' '}
                            <span className="font-mono text-xs text-muted-foreground">
                                {created.tenant.primary_host}
                            </span>
                        </p>
                        {created.owner_invitation && (
                            <div className="space-y-2">
                                <Label>
                                    {t('platform.tenants.acceptLink')}
                                </Label>
                                <CopyField
                                    value={created.owner_invitation.accept_url}
                                    testId="owner-accept-url"
                                />
                                <p className="text-xs text-muted-foreground">
                                    {t('platform.tenants.acceptLinkHint')}
                                </p>
                            </div>
                        )}
                    </div>
                ) : (
                    <form
                        id="create-tenant"
                        className="grid gap-4"
                        onSubmit={(e) => {
                            e.preventDefault();
                            submit();
                        }}
                    >
                        <div className="grid gap-2">
                            <Label htmlFor="tenant-name">
                                {t('platform.tenants.name')}
                            </Label>
                            <Input
                                id="tenant-name"
                                required
                                maxLength={120}
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                            />
                        </div>
                        <div className="grid gap-2">
                            <Label htmlFor="tenant-slug">
                                {t('platform.tenants.slug')}
                            </Label>
                            <div className="flex items-center gap-2">
                                <Input
                                    id="tenant-slug"
                                    required
                                    maxLength={30}
                                    value={effectiveSlug}
                                    onChange={(e) => {
                                        setSlugEdited(true);
                                        setSlug(
                                            sanitizeSlugInput(e.target.value),
                                        );
                                    }}
                                    className="font-mono"
                                />
                                <span className="text-sm whitespace-nowrap text-muted-foreground">
                                    .{baseDomain}
                                </span>
                            </div>
                        </div>
                        <div className="grid gap-2">
                            <Label htmlFor="tenant-owner">
                                {t('platform.tenants.ownerEmail')}
                            </Label>
                            <Input
                                id="tenant-owner"
                                type="email"
                                value={owner}
                                onChange={(e) => setOwner(e.target.value)}
                            />
                        </div>
                        <div className="grid gap-4 sm:grid-cols-2">
                            <div className="grid gap-2">
                                <Label htmlFor="tenant-trial">
                                    {t('platform.tenants.trialDays')}
                                </Label>
                                <NumberField
                                    id="tenant-trial"
                                    min={0}
                                    max={365}
                                    value={trial}
                                    onValueChange={(v) => setTrial(v ?? 0)}
                                />
                                <p className="text-xs text-muted-foreground">
                                    {t('platform.tenants.trialDaysHint')}
                                </p>
                            </div>
                            <div className="grid gap-2">
                                <Label htmlFor="tenant-locale">
                                    {t('platform.tenants.language')}
                                </Label>
                                <Select
                                    value={locale}
                                    onValueChange={(v) =>
                                        setLocale(v === 'en' ? 'en' : 'tr')
                                    }
                                >
                                    <SelectTrigger id="tenant-locale">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="tr">
                                            Türkçe
                                        </SelectItem>
                                        <SelectItem value="en">
                                            English
                                        </SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>
                        </div>
                    </form>
                )}
                <DialogFooter>
                    {created ? (
                        <Button onClick={() => setOpen(false)}>
                            {t('common.close')}
                        </Button>
                    ) : (
                        <Button
                            type="submit"
                            form="create-tenant"
                            disabled={pending || !name || !effectiveSlug}
                        >
                            {t('common.create')}
                        </Button>
                    )}
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
