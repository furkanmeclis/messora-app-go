import type { Metadata } from 'next';
import Link from 'next/link';
import { TrendingUp } from 'lucide-react';
import { getLocale, getT } from '@/i18n/server';
import { formatTL } from '@/lib/billing';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Stat } from '../billing-ui';
import { Heading } from '../heading';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('billing.admin.overview.title') };
}

/** Recurring revenue, collections and what waits for the admin. */
export default async function BillingOverviewPage() {
    const [t, locale, o] = await Promise.all([
        getT(),
        getLocale(),
        papi<Schemas['PlatformBillingOverview']>(
            '/platform/billing/overview',
        ).then((r) => r.data),
    ]);
    const money = (v: string) => formatTL(v, locale);
    return (
        <>
            <Heading
                icon={<TrendingUp />}
                title={t('billing.admin.overview.title')}
                description={t('billing.admin.overview.description')}
            />
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <Stat
                    label={t('billing.admin.overview.mrr')}
                    value={money(o.mrr)}
                    testId="billing-mrr"
                />
                <Stat
                    label={t('billing.admin.overview.arr')}
                    value={money(o.arr)}
                />
                <Stat
                    label={t('billing.admin.overview.collectedThisMonth')}
                    value={money(o.collected_this_month)}
                />
                <Stat
                    label={t('billing.admin.overview.collectedLastMonth')}
                    value={money(o.collected_last_month)}
                />
                <Stat
                    label={t('billing.admin.overview.open')}
                    value={money(o.open_amount)}
                />
                <Link href="/payments?status=unmatched">
                    <Stat
                        label={t('billing.admin.overview.unmatched')}
                        value={String(o.unmatched_transfers)}
                        tone={o.unmatched_transfers > 0 ? 'warning' : undefined}
                    />
                </Link>
                <Link href="/invoices?status=pending">
                    <Stat
                        label={t('billing.admin.overview.pendingInvoices')}
                        value={String(o.pending_invoices)}
                        tone={o.pending_invoices > 0 ? 'warning' : undefined}
                    />
                </Link>
            </div>
            <h2 className="mt-8 mb-3 font-semibold">
                {t('billing.admin.overview.counts')}
            </h2>
            <div className="grid gap-4 sm:grid-cols-3 lg:grid-cols-5">
                {(
                    [
                        'trial',
                        'active',
                        'past_due',
                        'read_only',
                        'canceled',
                    ] as const
                ).map((s) => (
                    <Link key={s} href={`/subscriptions?status=${s}`}>
                        <Stat
                            label={t(`billing.status.${s}`)}
                            value={String(o.counts[s])}
                        />
                    </Link>
                ))}
            </div>
        </>
    );
}
