'use client';

import { Copy, FileDown } from 'lucide-react';
import { toast } from 'sonner';
import { useFormatDate } from '@/components/display-preferences';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { useLocale, useT } from '@/i18n/client';
import { formatIBAN, formatTL } from '@/lib/billing';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';

/** Money in the viewer's language ("3.588,00 TL"). */
export function useMoney() {
    const locale = useLocale();
    return (v: string | number | null | undefined) => formatTL(v, locale);
}

const STATUS_TONE: Record<string, string> = {
    trial: 'border-primary/40 bg-primary/10 text-primary',
    active: 'border-success/40 bg-success/10 text-success',
    past_due: 'border-warning/50 bg-warning/10 text-warning',
    read_only: 'border-destructive/40 bg-destructive/10 text-destructive',
    canceled: 'text-muted-foreground',
};

export function BillingStatusBadge({
    status,
}: {
    status: Schemas['SubscriptionStatus'];
}) {
    const t = useT();
    return (
        <Badge
            variant="outline"
            className={cn(STATUS_TONE[status])}
            data-testid="billing-status"
            data-status={status}
        >
            {t(`billing.status.${status}`)}
        </Badge>
    );
}

const PROFORMA_TONE: Record<string, string> = {
    open: 'border-warning/50 bg-warning/10 text-warning',
    partially_paid: 'border-primary/40 bg-primary/10 text-primary',
    paid: 'border-success/40 bg-success/10 text-success',
    void: 'text-muted-foreground',
};

export function ProformaStatusBadge({
    status,
}: {
    status: Schemas['ProformaStatus'];
}) {
    const t = useT();
    return (
        <Badge
            variant="outline"
            className={cn(PROFORMA_TONE[status])}
            data-status={status}
        >
            {t(`billing.proforma.state.${status}`)}
        </Badge>
    );
}

/** Copies value to the clipboard with a short confirmation. */
export function CopyButton({
    value,
    label,
    testId,
}: {
    value: string;
    label?: string;
    testId?: string;
}) {
    const t = useT();
    return (
        <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            aria-label={label ?? t('billing.proforma.copy')}
            title={label ?? t('billing.proforma.copy')}
            data-testid={testId}
            onClick={() =>
                navigator.clipboard
                    .writeText(value)
                    .then(() => toast.success(t('billing.proforma.copied')))
                    .catch(() => undefined)
            }
        >
            <Copy />
        </Button>
    );
}

function Line({
    label,
    value,
    copy,
    mono,
    testId,
}: {
    label: string;
    value: string;
    copy?: string;
    mono?: boolean;
    testId?: string;
}) {
    return (
        <div className="flex items-center justify-between gap-3 py-1.5 text-sm">
            <span className="text-muted-foreground">{label}</span>
            <span className="flex min-w-0 items-center gap-1">
                <span
                    className={cn('truncate font-medium', mono && 'font-mono')}
                    data-testid={testId}
                >
                    {value}
                </span>
                {copy && <CopyButton value={copy} />}
            </span>
        </div>
    );
}

type Bank = { bank_name?: string; account_holder?: string; iban?: string };

/**
 * An open proforma with what is needed to pay it by bank transfer: the
 * KDV-inclusive amount, the due date, the reference and Messora's IBANs
 * (from the proforma's seller snapshot), each with a copy button.
 */
export function ProformaPayCard({
    proforma: f,
    pdfHref,
    actions,
}: {
    proforma: Schemas['Proforma'];
    pdfHref: string;
    actions?: React.ReactNode;
}) {
    const t = useT();
    const money = useMoney();
    const fmtDate = useFormatDate();
    const banks = (
        (f.seller as { bank_accounts?: Bank[] }).bank_accounts ?? []
    ).filter((b) => b.iban);
    const holderFallback = String((f.seller as { name?: string }).name ?? '');
    const paid = Number(f.paid) > 0;
    return (
        <article
            className="overflow-hidden rounded-2xl border bg-card shadow-xs"
            data-testid="proforma-card"
            data-reference={f.reference}
            data-id={f.id}
        >
            <header className="flex flex-wrap items-center justify-between gap-2 border-b bg-muted/30 px-5 py-3">
                <div className="flex flex-wrap items-center gap-2">
                    <span className="font-semibold">
                        {t('billing.proforma.number', { number: f.number })}
                    </span>
                    <Badge variant="outline">
                        {t(`billing.proforma.kind.${f.kind}`)}
                    </Badge>
                    {f.renewal && (
                        <Badge variant="outline">
                            {t('billing.proforma.renewal')}
                        </Badge>
                    )}
                    <ProformaStatusBadge status={f.status} />
                </div>
                <span className="text-sm text-muted-foreground">
                    {t('billing.proforma.due', { date: fmtDate(f.due_at) })}
                </span>
            </header>
            <div className="grid gap-6 px-5 py-4 md:grid-cols-2">
                <div>
                    {f.lines.map((l, i) => (
                        <p key={i} className="mb-2 text-sm">
                            {l.description_tr}
                        </p>
                    ))}
                    <div className="divide-y">
                        <Line
                            label={t('billing.proforma.net')}
                            value={money(f.net)}
                        />
                        <Line
                            label={t('billing.proforma.vat', {
                                rate: f.vat_rate_percent,
                            })}
                            value={money(f.vat)}
                        />
                        <Line
                            label={t('billing.proforma.total')}
                            value={money(f.total)}
                            testId="proforma-total"
                        />
                        {paid && (
                            <>
                                <Line
                                    label={t('billing.proforma.paid')}
                                    value={money(f.paid)}
                                />
                                <Line
                                    label={t('billing.proforma.remaining')}
                                    value={money(f.due)}
                                />
                            </>
                        )}
                    </div>
                </div>
                <div>
                    <div className="mb-3 rounded-xl border-l-4 border-l-mint bg-muted/40 px-4 py-3">
                        <div className="text-xs font-medium text-muted-foreground uppercase">
                            {t('billing.proforma.reference')}
                        </div>
                        <div className="flex items-center gap-1">
                            <span
                                className="font-mono text-lg font-semibold tracking-wide text-primary"
                                data-testid="proforma-reference"
                            >
                                {f.reference_display}
                            </span>
                            <CopyButton
                                value={f.reference}
                                testId="copy-reference"
                            />
                        </div>
                        <p className="mt-1 text-xs text-muted-foreground">
                            {t('billing.proforma.referenceHint')}
                        </p>
                    </div>
                    {banks.map((b, i) => (
                        <div key={i} className="divide-y" data-testid="bank">
                            <Line
                                label={t('billing.proforma.bank')}
                                value={b.bank_name ?? ''}
                            />
                            <Line
                                label={t('billing.proforma.holder')}
                                value={b.account_holder || holderFallback}
                                copy={b.account_holder || holderFallback}
                            />
                            <Line
                                label={t('billing.proforma.iban')}
                                value={formatIBAN(b.iban ?? '')}
                                copy={(b.iban ?? '').replace(/\s+/g, '')}
                                mono
                                testId="bank-iban"
                            />
                        </div>
                    ))}
                </div>
            </div>
            <footer className="flex flex-wrap items-center justify-end gap-2 border-t px-5 py-3">
                {actions}
                <Button asChild variant="outline" size="sm">
                    <a href={pdfHref} download data-testid="proforma-pdf">
                        <FileDown /> {t('billing.proforma.pdf')}
                    </a>
                </Button>
            </footer>
        </article>
    );
}
