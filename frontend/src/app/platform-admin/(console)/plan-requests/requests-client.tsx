'use client';

import { useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useFormatDateTime } from '@/components/display-preferences';
import { notify } from '@/components/notify';
import { usePlanName } from '@/components/plan-usage';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
    Dialog,
    DialogContent,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Segmented } from '@/components/ui/segmented';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { Textarea } from '@/components/ui/textarea';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';
import { resolvePlanRequestAction } from '../../plan-actions';

type Req = Schemas['PlatformPlanRequest'];
const FILTERS = ['open', 'done', 'declined', 'all'] as const;

export function RequestsClient({
    requests,
    plans,
    status,
}: {
    requests: Req[];
    plans: Schemas['PlatformPlan'][];
    status: string;
}) {
    const t = useT();
    const fmt = useFormatDateTime();
    const planName = usePlanName();
    const [resolving, setResolving] = useState<Req | null>(null);
    const nameOf = (key: string | null) => {
        if (!key) return '—';
        const p = plans.find((x) => x.key === key);
        return p ? planName(p) : key;
    };
    return (
        <div className="grid gap-4">
            <nav className="flex flex-wrap gap-2">
                {FILTERS.map((f) => (
                    <Link
                        key={f}
                        href={`/plan-requests?status=${f}`}
                        className={cn(
                            'rounded-full border px-3 py-1 text-sm',
                            status === f
                                ? 'border-primary bg-primary/10 text-primary'
                                : 'text-muted-foreground hover:text-foreground',
                        )}
                    >
                        {f === 'all'
                            ? t('plans.admin.requests.all')
                            : t(`plans.admin.requests.status.${f}`)}
                    </Link>
                ))}
            </nav>
            <div className="overflow-x-auto rounded-xl border bg-card">
                <Table data-testid="plan-requests">
                    <TableHeader>
                        <TableRow>
                            <TableHead>
                                {t('plans.admin.requests.company')}
                            </TableHead>
                            <TableHead>
                                {t('plans.admin.requests.from')}
                            </TableHead>
                            <TableHead>
                                {t('plans.admin.requests.to')}
                            </TableHead>
                            <TableHead>
                                {t('plans.admin.requests.cycle')}
                            </TableHead>
                            <TableHead>
                                {t('plans.admin.requests.message')}
                            </TableHead>
                            <TableHead>
                                {t('plans.admin.requests.requestedBy')}
                            </TableHead>
                            <TableHead>
                                {t('plans.admin.requests.created')}
                            </TableHead>
                            <TableHead />
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {requests.length === 0 && (
                            <TableRow>
                                <TableCell
                                    colSpan={8}
                                    className="py-10 text-center text-muted-foreground"
                                >
                                    {t('plans.admin.requests.empty')}
                                </TableCell>
                            </TableRow>
                        )}
                        {requests.map((r) => (
                            <TableRow key={r.id} data-testid="plan-request-row">
                                <TableCell>
                                    <Link
                                        href={`/tenants/${r.tenant.id}`}
                                        className="font-medium hover:underline"
                                    >
                                        {r.tenant.name}
                                    </Link>
                                    <div className="text-xs text-muted-foreground">
                                        {r.tenant.slug}
                                    </div>
                                </TableCell>
                                <TableCell>{nameOf(r.current_plan)}</TableCell>
                                <TableCell className="font-medium">
                                    {nameOf(r.requested_plan)}
                                </TableCell>
                                <TableCell>
                                    {r.billing_cycle === 'yearly'
                                        ? t('plans.yearly')
                                        : t('plans.monthly')}
                                </TableCell>
                                <TableCell className="max-w-64 text-sm">
                                    <p className="line-clamp-3">
                                        {r.message || '—'}
                                    </p>
                                    {r.resolution_note && (
                                        <p className="mt-1 text-xs text-muted-foreground">
                                            {r.resolution_note}
                                        </p>
                                    )}
                                </TableCell>
                                <TableCell className="text-sm">
                                    {r.requested_by ? (
                                        <>
                                            {r.requested_by.name}
                                            <div className="text-xs text-muted-foreground">
                                                {r.requested_by.email}
                                            </div>
                                        </>
                                    ) : (
                                        '—'
                                    )}
                                </TableCell>
                                <TableCell className="text-sm whitespace-nowrap text-muted-foreground">
                                    {fmt(r.created_at)}
                                </TableCell>
                                <TableCell className="text-right">
                                    {r.status === 'open' ? (
                                        <Button
                                            size="sm"
                                            variant="outline"
                                            onClick={() => setResolving(r)}
                                        >
                                            {t('plans.admin.requests.resolve')}
                                        </Button>
                                    ) : (
                                        <Badge variant="outline">
                                            {t(
                                                `plans.admin.requests.status.${r.status}`,
                                            )}
                                        </Badge>
                                    )}
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </div>
            {resolving && (
                <ResolveDialog
                    request={resolving}
                    onClose={() => setResolving(null)}
                />
            )}
        </div>
    );
}

function ResolveDialog({
    request,
    onClose,
}: {
    request: Req;
    onClose: () => void;
}) {
    const t = useT();
    const router = useRouter();
    const [status, setStatus] = useState<'done' | 'declined'>('done');
    const [note, setNote] = useState('');
    const [apply, setApply] = useState(true);
    const [pending, start] = useTransition();
    const submit = () =>
        start(async () => {
            const r = await resolvePlanRequestAction(request.id, {
                status,
                note: note.trim(),
                apply: status === 'done' && apply,
            });
            if (notify(r)) {
                onClose();
                router.refresh();
            }
        });
    return (
        <Dialog open onOpenChange={(o) => !o && onClose()}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>
                        {t('plans.admin.requests.resolveTitle')}
                    </DialogTitle>
                </DialogHeader>
                <div className="grid gap-4">
                    <div className="grid gap-2">
                        <Label>{t('plans.admin.requests.outcome')}</Label>
                        <Segmented
                            value={status}
                            onValueChange={setStatus}
                            options={(['done', 'declined'] as const).map(
                                (k) => ({
                                    value: k,
                                    label: t(
                                        `plans.admin.requests.status.${k}`,
                                    ),
                                }),
                            )}
                        />
                    </div>
                    {status === 'done' && (
                        <Label className="flex items-center gap-2 font-normal">
                            <Checkbox
                                checked={apply}
                                onCheckedChange={(v) => setApply(v === true)}
                            />
                            {t('plans.admin.requests.apply')}
                        </Label>
                    )}
                    <div className="grid gap-2">
                        <Label htmlFor="resolve-note">
                            {t('plans.admin.requests.note')}
                        </Label>
                        <Textarea
                            id="resolve-note"
                            value={note}
                            maxLength={1000}
                            onChange={(e) => setNote(e.target.value)}
                        />
                    </div>
                </div>
                <DialogFooter>
                    <Button variant="outline" onClick={onClose}>
                        {t('common.cancel')}
                    </Button>
                    <Button onClick={submit} disabled={pending}>
                        {t('plans.admin.requests.resolve')}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
