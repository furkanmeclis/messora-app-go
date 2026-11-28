'use client';

import { useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { FileDown } from 'lucide-react';
import { toast } from 'sonner';
import { useMoney } from '@/components/billing';
import { useFormatDate } from '@/components/display-preferences';
import { notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { DatePicker } from '@/components/ui/date-picker';
import { FileDrop } from '@/components/ui/file-drop';
import { Input } from '@/components/ui/input';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { recordInvoiceAction } from '../../billing-actions';

type Invoice = Schemas['Invoice'];

/** Invoice records: official number, PDF upload and download (admin). */
export function InvoicesTable({ rows }: { rows: Invoice[] }) {
    const t = useT();
    if (rows.length === 0) {
        return (
            <p className="rounded-xl border bg-card py-10 text-center text-sm text-muted-foreground">
                {t('billing.admin.invoices.empty')}
            </p>
        );
    }
    return (
        <div className="grid gap-3" data-testid="admin-invoices">
            {rows.map((i) => (
                <InvoiceRow key={i.id} invoice={i} />
            ))}
        </div>
    );
}

function InvoiceRow({ invoice: i }: { invoice: Invoice }) {
    const t = useT();
    const money = useMoney();
    const fmtDate = useFormatDate();
    const router = useRouter();
    const [number, setNumber] = useState(i.official_number ?? '');
    const [issuedOn, setIssuedOn] = useState(
        i.issued_on ?? new Date().toISOString().slice(0, 10),
    );
    const [pending, start] = useTransition();
    const record = () =>
        start(async () => {
            if (
                notify(
                    await recordInvoiceAction(i.id, {
                        official_number: number.trim(),
                        issued_on: issuedOn,
                    }),
                )
            ) {
                router.refresh();
            }
        });
    const upload = (file: File) =>
        start(async () => {
            const res = await fetch(`/bff/billing/invoices/${i.id}/pdf`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/pdf' },
                body: file,
            });
            if (!res.ok) {
                const body = (await res.json().catch(() => null)) as {
                    error?: { message?: string };
                } | null;
                toast.error(body?.error?.message ?? `HTTP ${res.status}`);
                return;
            }
            toast.success(t('billing.admin.invoices.uploaded'));
            router.refresh();
        });
    return (
        <article
            className="grid gap-3 rounded-2xl border bg-card p-4 shadow-xs lg:grid-cols-[1fr_auto]"
            data-testid="admin-invoice"
            data-status={i.status}
        >
            <div className="grid gap-1">
                <div className="flex flex-wrap items-center gap-2">
                    <span className="font-semibold">{i.proforma_number}</span>
                    {i.tenant && (
                        <Link
                            href={`/tenants/${i.tenant.id}`}
                            className="text-sm hover:underline"
                        >
                            {i.tenant.name}
                        </Link>
                    )}
                    <Badge variant="outline">
                        {t(`billing.docs.invoiceState.${i.status}`)}
                    </Badge>
                    <Badge variant="outline">{i.issuer}</Badge>
                </div>
                <div className="text-sm text-muted-foreground">
                    {money(i.total)} · {fmtDate(i.created_at)}
                    {i.official_number &&
                        ` · ${t('billing.docs.official', { number: i.official_number })}`}
                </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
                <Input
                    aria-label={t('billing.admin.invoices.number')}
                    placeholder={t('billing.admin.invoices.number')}
                    value={number}
                    onChange={(e) => setNumber(e.target.value)}
                    className="w-52"
                />
                <DatePicker
                    value={issuedOn}
                    onValueChange={setIssuedOn}
                    placeholder={t('billing.admin.invoices.issuedOn')}
                    className="w-40"
                />
                <Button
                    size="sm"
                    disabled={pending || !number.trim() || !issuedOn}
                    onClick={record}
                >
                    {t('billing.admin.invoices.record')}
                </Button>
                <FileDrop
                    compact
                    name="invoice-pdf"
                    accept="application/pdf,.pdf"
                    file={null}
                    disabled={pending}
                    hint={t('billing.admin.invoices.upload')}
                    onFile={(f) => f && upload(f)}
                    className="w-56"
                />
                {i.has_pdf && (
                    <Button asChild size="icon-xs" variant="ghost">
                        <a
                            href={`/bff/billing/invoices/${i.id}/pdf`}
                            download
                            aria-label={t('billing.docs.download')}
                        >
                            <FileDown />
                        </a>
                    </Button>
                )}
            </div>
        </article>
    );
}
