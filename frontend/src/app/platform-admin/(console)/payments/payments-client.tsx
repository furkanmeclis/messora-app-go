'use client';

import { useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Check } from 'lucide-react';
import { toast } from 'sonner';
import { useMoney } from '@/components/billing';
import { useFormatDate } from '@/components/display-preferences';
import { notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { DatePicker } from '@/components/ui/date-picker';
import { FileDrop } from '@/components/ui/file-drop';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useT } from '@/i18n/client';
import { fromKurus, toKurus } from '@/lib/billing';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';
import {
    ignoreTransferAction,
    matchTransferAction,
    recordTransferAction,
} from '../../billing-actions';
import { FilterLinks, ReasonButton } from '../billing-ui';

type Transfer = Schemas['BankTransfer'];
type Proforma = Schemas['Proforma'];

const today = () => new Date().toISOString().slice(0, 10);

export function PaymentsClient({
    transfers,
    status,
}: {
    transfers: Transfer[];
    status: string;
}) {
    const t = useT();
    return (
        <div className="grid gap-6">
            <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
                <RecordForm />
                <ImportCSV />
            </div>
            <FilterLinks
                base="/payments"
                current={status}
                options={(
                    ['unmatched', 'matched', 'ignored', 'all'] as const
                ).map((s) => ({
                    value: s,
                    label: t(`billing.admin.payments.filter.${s}`),
                }))}
            />
            {transfers.length === 0 ? (
                <p className="rounded-xl border bg-card py-10 text-center text-sm text-muted-foreground">
                    {t('billing.admin.payments.empty')}
                </p>
            ) : (
                <div className="grid gap-3" data-testid="transfers">
                    {transfers.map((tr) => (
                        <TransferCard key={tr.id} transfer={tr} />
                    ))}
                </div>
            )}
        </div>
    );
}

function RecordForm() {
    const t = useT();
    const router = useRouter();
    const [pending, start] = useTransition();
    const empty = {
        amount: '',
        received_on: today(),
        sender_name: '',
        sender_iban: '',
        description: '',
        reference: '',
        note: '',
    };
    const [v, setV] = useState(empty);
    const set = (k: keyof typeof v) => (e: { target: { value: string } }) =>
        setV({ ...v, [k]: e.target.value });
    const kurus = toKurus(v.amount);
    const submit = () =>
        start(async () => {
            if (kurus === null || kurus <= 0) return;
            const r = await recordTransferAction({
                amount: fromKurus(kurus),
                received_on: v.received_on,
                sender_name: v.sender_name || undefined,
                sender_iban: v.sender_iban || undefined,
                description: v.description || undefined,
                reference: v.reference || undefined,
                note: v.note || undefined,
            });
            if (notify(r)) {
                setV(empty);
                router.refresh();
            }
        });
    const field = (
        k: keyof typeof v,
        label: string,
        props: React.ComponentProps<typeof Input> = {},
    ) => (
        <div className="grid gap-1.5">
            <Label htmlFor={`tr-${k}`}>{label}</Label>
            <Input
                id={`tr-${k}`}
                name={k}
                value={v[k]}
                onChange={set(k)}
                {...props}
            />
        </div>
    );
    return (
        <section
            className="rounded-2xl border bg-card p-5 shadow-xs"
            data-testid="record-transfer"
        >
            <h2 className="mb-4 font-semibold">
                {t('billing.admin.payments.record')}
            </h2>
            <div className="grid gap-3 sm:grid-cols-3">
                {field('amount', t('billing.admin.payments.amount'), {
                    inputMode: 'decimal',
                    placeholder: '3588.00',
                })}
                <div className="grid gap-1.5">
                    <Label htmlFor="tr-received_on">
                        {t('billing.admin.payments.receivedOn')}
                    </Label>
                    <DatePicker
                        id="tr-received_on"
                        value={v.received_on}
                        onValueChange={(d) => setV({ ...v, received_on: d })}
                    />
                </div>
                {field('sender_name', t('billing.admin.payments.sender'))}
            </div>
            <div className="mt-3 grid gap-3 sm:grid-cols-2">
                {field(
                    'description',
                    t('billing.admin.payments.descriptionField'),
                )}
                {field('reference', t('billing.admin.payments.reference'))}
                {field('sender_iban', t('billing.admin.payments.senderIban'))}
                {field('note', t('billing.admin.payments.note'))}
            </div>
            <div className="mt-4 flex justify-end">
                <Button
                    onClick={submit}
                    disabled={pending || kurus === null || kurus <= 0}
                    data-testid="record-transfer-submit"
                >
                    {t('billing.admin.payments.save')}
                </Button>
            </div>
        </section>
    );
}

function ImportCSV() {
    const t = useT();
    const router = useRouter();
    const [busy, setBusy] = useState(false);
    const upload = async (file: File) => {
        setBusy(true);
        try {
            const res = await fetch('/bff/billing/transfers', {
                method: 'POST',
                headers: { 'Content-Type': 'text/csv' },
                body: file,
            });
            const body = (await res.json().catch(() => null)) as {
                data?: Schemas['BankTransferImport'];
                error?: { message?: string };
            } | null;
            if (!res.ok || !body?.data) {
                toast.error(body?.error?.message ?? `HTTP ${res.status}`);
                return;
            }
            const d = body.data;
            toast.success(
                t('billing.admin.payments.imported', {
                    created: d.created,
                    duplicates: d.duplicates,
                }),
            );
            for (const e of d.errors.slice(0, 5)) {
                toast.warning(
                    t('billing.admin.payments.importErrors', {
                        line: e.line,
                        message: e.message,
                    }),
                );
            }
            router.refresh();
        } finally {
            setBusy(false);
        }
    };
    return (
        <section className="rounded-2xl border bg-card p-5 shadow-xs">
            <h2 className="mb-2 font-semibold">
                {t('billing.admin.payments.import')}
            </h2>
            <p className="mb-4 text-sm text-muted-foreground">
                {t('billing.admin.payments.importHint')}
            </p>
            <FileDrop
                name="statement"
                accept=".csv,text/csv"
                file={null}
                disabled={busy}
                onFile={(f) => {
                    if (f) void upload(f);
                }}
            />
        </section>
    );
}

function ProformaLine({ f }: { f: Proforma }) {
    const t = useT();
    const money = useMoney();
    const fmtDate = useFormatDate();
    return (
        <span className="text-sm">
            {f.tenant && (
                <Link
                    href={`/tenants/${f.tenant.id}`}
                    className="font-medium hover:underline"
                >
                    {f.tenant.name}
                </Link>
            )}{' '}
            · {f.number} · {t(`billing.proforma.kind.${f.kind}`)} ·{' '}
            {money(f.due)} ·{' '}
            {t('billing.proforma.due', { date: fmtDate(f.due_at) })}
        </span>
    );
}

function TransferCard({ transfer: tr }: { transfer: Transfer }) {
    const t = useT();
    const money = useMoney();
    const fmtDate = useFormatDate();
    const router = useRouter();
    const [pending, start] = useTransition();
    const [proformaId, setProformaId] = useState('');
    const [tenantId, setTenantId] = useState('');
    const match = (body: Schemas['MatchBankTransferRequest']) =>
        start(async () => {
            if (notify(await matchTransferAction(tr.id, body)))
                router.refresh();
        });
    const s = tr.suggestion;
    return (
        <article
            className="rounded-2xl border bg-card p-4 shadow-xs"
            data-testid="transfer"
            data-status={tr.status}
            data-reference={tr.reference ?? ''}
        >
            <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="grid gap-1">
                    <div className="flex flex-wrap items-center gap-2">
                        <span className="text-lg font-semibold">
                            {money(tr.amount)}
                        </span>
                        <span className="text-sm text-muted-foreground">
                            {fmtDate(tr.received_on)}
                            {tr.sender_name && ` · ${tr.sender_name}`}
                        </span>
                        <Badge variant="outline">
                            {t(`billing.admin.payments.status.${tr.status}`)}
                        </Badge>
                        {tr.source === 'csv' && (
                            <Badge variant="outline">CSV</Badge>
                        )}
                    </div>
                    {tr.description && (
                        <p className="text-sm break-all">{tr.description}</p>
                    )}
                    {tr.reference && (
                        <p className="font-mono text-xs text-muted-foreground">
                            {tr.reference}
                        </p>
                    )}
                    {tr.status === 'matched' && tr.tenant && (
                        <p className="text-sm text-success">
                            {t('billing.admin.payments.matchedTo', {
                                tenant: tr.tenant.name,
                            })}
                        </p>
                    )}
                    {tr.note && (
                        <p className="text-xs text-muted-foreground">
                            {tr.note}
                        </p>
                    )}
                </div>
                {tr.status === 'unmatched' && (
                    <ReasonButton
                        label={t('billing.admin.payments.ignore')}
                        title={t('billing.admin.payments.ignoreReason')}
                        variant="ghost"
                        onConfirm={async (reason) => {
                            const ok = notify(
                                await ignoreTransferAction(tr.id, reason),
                            );
                            if (ok) router.refresh();
                            return ok;
                        }}
                    />
                )}
            </div>
            {tr.status === 'unmatched' && (
                <div className="mt-3 grid gap-3 border-t pt-3">
                    {tr.suggested_proforma ? (
                        <div
                            className={cn(
                                'flex flex-wrap items-center gap-3 rounded-xl px-3 py-2',
                                s === 'reference_amount'
                                    ? 'bg-success/10'
                                    : 'bg-warning/10',
                            )}
                            data-testid="suggestion"
                            data-suggestion={s ?? ''}
                        >
                            <Badge
                                variant="outline"
                                className={cn(
                                    s === 'reference_amount'
                                        ? 'border-success/40 text-success'
                                        : 'border-warning/50 text-warning',
                                )}
                            >
                                {s &&
                                    t(`billing.admin.payments.suggestion.${s}`)}
                            </Badge>
                            <ProformaLine f={tr.suggested_proforma} />
                            <Button
                                size="sm"
                                className="ml-auto"
                                disabled={pending}
                                data-testid="confirm-match"
                                onClick={() =>
                                    match({
                                        proforma_id: tr.suggested_proforma!.id,
                                    })
                                }
                            >
                                <Check /> {t('billing.admin.payments.confirm')}
                            </Button>
                        </div>
                    ) : tr.candidates.length > 0 ? (
                        <div className="grid gap-2" data-testid="candidates">
                            <p className="text-sm text-muted-foreground">
                                {t('billing.admin.payments.candidates')}
                            </p>
                            {tr.candidates.map((c) => (
                                <div
                                    key={c.id}
                                    className="flex flex-wrap items-center gap-3 rounded-xl bg-muted/40 px-3 py-2"
                                >
                                    <ProformaLine f={c} />
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        className="ml-auto"
                                        disabled={pending}
                                        onClick={() =>
                                            match({ proforma_id: c.id })
                                        }
                                    >
                                        {t('billing.admin.payments.confirm')}
                                    </Button>
                                </div>
                            ))}
                        </div>
                    ) : (
                        <p className="text-sm text-muted-foreground">
                            {t('billing.admin.payments.noSuggestion')}
                        </p>
                    )}
                    <div className="grid gap-3 sm:grid-cols-2">
                        <div className="flex items-end gap-2">
                            <div className="grid flex-1 gap-1.5">
                                <Label htmlFor={`pf-${tr.id}`}>
                                    {t('billing.admin.payments.matchTo')}
                                </Label>
                                <Input
                                    id={`pf-${tr.id}`}
                                    placeholder={t(
                                        'billing.admin.payments.proformaId',
                                    )}
                                    value={proformaId}
                                    onChange={(e) =>
                                        setProformaId(e.target.value)
                                    }
                                    className="font-mono text-xs"
                                />
                            </div>
                            <Button
                                size="sm"
                                variant="outline"
                                disabled={pending || !proformaId.trim()}
                                onClick={() =>
                                    match({ proforma_id: proformaId.trim() })
                                }
                            >
                                {t('billing.admin.payments.confirm')}
                            </Button>
                        </div>
                        <div className="flex items-end gap-2">
                            <div className="grid flex-1 gap-1.5">
                                <Label htmlFor={`tn-${tr.id}`}>
                                    {t('billing.admin.payments.toCredit')}
                                </Label>
                                <Input
                                    id={`tn-${tr.id}`}
                                    placeholder={t(
                                        'billing.admin.payments.tenantId',
                                    )}
                                    value={tenantId}
                                    onChange={(e) =>
                                        setTenantId(e.target.value)
                                    }
                                    className="font-mono text-xs"
                                />
                            </div>
                            <Button
                                size="sm"
                                variant="outline"
                                disabled={pending || !tenantId.trim()}
                                onClick={() =>
                                    match({ tenant_id: tenantId.trim() })
                                }
                            >
                                {t('billing.admin.payments.confirm')}
                            </Button>
                        </div>
                    </div>
                </div>
            )}
        </article>
    );
}
