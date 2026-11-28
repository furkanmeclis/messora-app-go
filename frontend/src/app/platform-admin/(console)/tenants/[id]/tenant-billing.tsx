'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { BillingStatusBadge, useMoney } from '@/components/billing';
import { useFormatDate } from '@/components/display-preferences';
import { notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
import { Textarea } from '@/components/ui/textarea';
import { useT } from '@/i18n/client';
import { fromKurus, toKurus } from '@/lib/billing';
import type { Schemas } from '@/lib/types';
import { billingActionAction } from '../../../billing-actions';
import { ProformasTable } from '../../proformas/proformas-client';
import { InvoicesTable } from '../../invoices/invoices-client';

type Action = Schemas['PlatformBillingActionRequest']['action'];
const ACTIONS: Action[] = [
    'extend_days',
    'comp_months',
    'set_cycle',
    'set_status',
    'set_custom_price',
    'adjust_message_balance',
    'adjust_account_credit',
];

/** A company's billing in the platform panel, with the admin's actions. */
export function TenantBilling({
    tenantId,
    data,
}: {
    tenantId: string;
    data: Schemas['PlatformTenantBilling'];
}) {
    const t = useT();
    const money = useMoney();
    const fmtDate = useFormatDate();
    const s = data.subscription;
    const p = data.profile;
    return (
        <section
            className="mt-6 grid gap-4 rounded-2xl border bg-card p-5 shadow-xs"
            data-testid="tenant-billing"
        >
            <div className="flex flex-wrap items-center gap-2">
                <h2 className="mr-2 text-lg font-semibold">
                    {t('billing.admin.tenant.title')}
                </h2>
                <BillingStatusBadge status={s.status} />
                <Badge variant="outline">
                    {s.plan_key ?? '—'} ·{' '}
                    {t(`billing.cycle.${s.billing_cycle}`)}
                </Badge>
                <span className="font-mono text-xs text-muted-foreground">
                    {s.code}
                </span>
            </div>
            <dl className="grid gap-x-6 gap-y-1 text-sm sm:grid-cols-2 lg:grid-cols-4">
                <div>
                    <dt className="text-xs text-muted-foreground">
                        {t('billing.admin.subs.periodEnd')}
                    </dt>
                    <dd>
                        {s.period_start && s.period_end
                            ? `${fmtDate(s.period_start)} – ${fmtDate(s.period_end)}`
                            : t('billing.plan.noPeriod')}
                    </dd>
                </div>
                <div>
                    <dt className="text-xs text-muted-foreground">
                        {t('billing.plan.accountCredit')}
                    </dt>
                    <dd>{money(s.account_credit)}</dd>
                </div>
                <div>
                    <dt className="text-xs text-muted-foreground">
                        {t('billing.balances.message')}
                    </dt>
                    <dd>
                        {data.message_balance.used
                            ? money(data.message_balance.balance)
                            : '—'}
                    </dd>
                </div>
                <div>
                    <dt className="text-xs text-muted-foreground">
                        {t('billing.admin.tenant.profile')}
                    </dt>
                    <dd>
                        {p
                            ? `${p.title} · ${p.tax_office ? p.tax_office + ' · ' : ''}${p.tax_number} · ${p.email}`
                            : t('billing.admin.tenant.noProfile')}
                    </dd>
                </div>
            </dl>
            {(s.status_reason || s.grace_ends_at || s.pending_plan_key) && (
                <p className="text-sm text-muted-foreground">
                    {[
                        s.status_reason,
                        s.grace_ends_at &&
                            t('billing.plan.graceUntil', {
                                date: fmtDate(s.grace_ends_at),
                            }),
                        s.pending_plan_key && `→ ${s.pending_plan_key}`,
                    ]
                        .filter(Boolean)
                        .join(' · ')}
                </p>
            )}
            <ActionForm tenantId={tenantId} />
            <h3 className="mt-2 text-sm font-medium">
                {t('billing.docs.proformas')}
            </h3>
            <ProformasTable rows={data.proformas} showTenant={false} />
            <h3 className="mt-2 text-sm font-medium">
                {t('billing.docs.invoices')}
            </h3>
            <InvoicesTable rows={data.invoices} />
            {(data.payments.length > 0 || data.account_entries.length > 0) && (
                <div className="grid gap-4 lg:grid-cols-2">
                    <div>
                        <h3 className="mb-2 text-sm font-medium">
                            {t('billing.admin.tenant.payments')}
                        </h3>
                        <ul className="grid gap-1 text-sm">
                            {data.payments.map((x) => (
                                <li key={x.id}>
                                    {fmtDate(x.received_on)} · {money(x.amount)}{' '}
                                    ·{' '}
                                    {t(
                                        `billing.admin.tenant.source.${x.source}`,
                                    )}
                                </li>
                            ))}
                        </ul>
                    </div>
                    <div>
                        <h3 className="mb-2 text-sm font-medium">
                            {t('billing.admin.tenant.entries')}
                        </h3>
                        <ul className="grid gap-1 text-sm">
                            {data.account_entries.map((x) => (
                                <li key={x.id}>
                                    {fmtDate(x.created_at)} · {money(x.amount)}{' '}
                                    ·{' '}
                                    {t(
                                        `billing.admin.tenant.reasons.${x.reason}`,
                                    )}
                                    {x.note && ` · ${x.note}`}
                                </li>
                            ))}
                        </ul>
                    </div>
                </div>
            )}
        </section>
    );
}

function ActionForm({ tenantId }: { tenantId: string }) {
    const t = useT();
    const router = useRouter();
    const [pending, start] = useTransition();
    const [action, setAction] = useState<Action>('extend_days');
    const [days, setDays] = useState(7);
    const [months, setMonths] = useState(1);
    const [cycle, setCycle] = useState<Schemas['BillingCycle']>('monthly');
    const [status, setStatus] = useState<'active' | 'read_only' | 'canceled'>(
        'active',
    );
    const [amount, setAmount] = useState('');
    const [reason, setReason] = useState('');
    const apply = () =>
        start(async () => {
            const body: Schemas['PlatformBillingActionRequest'] = {
                action,
                reason: reason.trim(),
            };
            if (action === 'extend_days') body.days = days;
            if (action === 'comp_months') body.months = months;
            if (action === 'set_cycle') body.cycle = cycle;
            if (action === 'set_status') body.status = status;
            const k = toKurus(amount);
            if (action === 'set_custom_price') {
                body.price = amount.trim() && k !== null ? fromKurus(k) : null;
            }
            if (
                action === 'adjust_message_balance' ||
                action === 'adjust_account_credit'
            ) {
                if (k === null) return;
                body.amount = fromKurus(k);
            }
            if (notify(await billingActionAction(tenantId, body))) {
                setReason('');
                router.refresh();
            }
        });
    return (
        <div
            className="grid gap-3 rounded-xl border bg-muted/20 p-4"
            data-testid="billing-action"
        >
            <div className="grid gap-3 sm:grid-cols-[16rem_1fr]">
                <div className="grid gap-1.5">
                    <Label htmlFor="ba-action">
                        {t('billing.admin.tenant.action')}
                    </Label>
                    <Select
                        value={action}
                        onValueChange={(v) => setAction(v as Action)}
                    >
                        <SelectTrigger
                            id="ba-action"
                            data-testid="billing-action-select"
                        >
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {ACTIONS.map((a) => (
                                <SelectItem key={a} value={a}>
                                    {t(`billing.admin.tenant.actions.${a}`)}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>
                <div className="grid gap-1.5">
                    {action === 'extend_days' && (
                        <>
                            <Label htmlFor="ba-days">
                                {t('billing.admin.tenant.days')}
                            </Label>
                            <NumberField
                                id="ba-days"
                                min={1}
                                value={days}
                                onValueChange={(n) => setDays(n ?? 1)}
                                className="w-32"
                            />
                        </>
                    )}
                    {action === 'comp_months' && (
                        <>
                            <Label htmlFor="ba-months">
                                {t('billing.admin.tenant.months')}
                            </Label>
                            <NumberField
                                id="ba-months"
                                min={1}
                                value={months}
                                onValueChange={(n) => setMonths(n ?? 1)}
                                className="w-32"
                            />
                        </>
                    )}
                    {action === 'set_cycle' && (
                        <Select
                            value={cycle}
                            onValueChange={(v) =>
                                setCycle(v as Schemas['BillingCycle'])
                            }
                        >
                            <SelectTrigger className="w-44">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {(['monthly', 'yearly'] as const).map((c) => (
                                    <SelectItem key={c} value={c}>
                                        {t(`billing.cycle.${c}`)}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    )}
                    {action === 'set_status' && (
                        <Select
                            value={status}
                            onValueChange={(v) => setStatus(v as typeof status)}
                        >
                            <SelectTrigger
                                className="w-44"
                                data-testid="billing-status-select"
                            >
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {(
                                    ['active', 'read_only', 'canceled'] as const
                                ).map((c) => (
                                    <SelectItem key={c} value={c}>
                                        {t(`billing.status.${c}`)}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    )}
                    {(action === 'set_custom_price' ||
                        action === 'adjust_message_balance' ||
                        action === 'adjust_account_credit') && (
                        <>
                            <Label htmlFor="ba-amount">
                                {action === 'set_custom_price'
                                    ? t('billing.admin.tenant.price')
                                    : t('billing.admin.tenant.amount')}
                            </Label>
                            <Input
                                id="ba-amount"
                                inputMode="decimal"
                                value={amount}
                                onChange={(e) => setAmount(e.target.value)}
                                className="w-44"
                            />
                        </>
                    )}
                </div>
            </div>
            <div className="grid gap-1.5">
                <Label htmlFor="ba-reason">
                    {t('billing.admin.tenant.reason')}
                </Label>
                <Textarea
                    id="ba-reason"
                    value={reason}
                    onChange={(e) => setReason(e.target.value)}
                    maxLength={500}
                    rows={2}
                />
            </div>
            <div className="flex justify-end">
                <Button
                    size="sm"
                    disabled={pending || reason.trim().length < 3}
                    onClick={apply}
                    data-testid="billing-action-apply"
                >
                    {t('billing.admin.tenant.apply')}
                </Button>
            </div>
        </div>
    );
}
