import type { Metadata } from 'next';
import Link from 'next/link';
import { Repeat } from 'lucide-react';
import { getLocale, getT } from '@/i18n/server';
import { formatTL } from '@/lib/billing';
import { formatDate } from '@/lib/format';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { FilterLinks } from '../billing-ui';
import { SubscriptionBadge } from '../badges';
import { Heading } from '../heading';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('billing.admin.subs.title') };
}

const STATUSES = [
    'trial',
    'active',
    'past_due',
    'read_only',
    'canceled',
] as const;

/** Every company's subscription. */
export default async function SubscriptionsPage({
    searchParams,
}: {
    searchParams: Promise<{ status?: string }>;
}) {
    const { status: raw } = await searchParams;
    const status = (STATUSES as readonly string[]).includes(raw ?? '')
        ? raw
        : undefined;
    const [t, locale, rows] = await Promise.all([
        getT(),
        getLocale(),
        papi<Schemas['PlatformSubscription'][]>(
            `/platform/billing/subscriptions${status ? `?status=${status}` : ''}`,
        ).then((r) => r.data),
    ]);
    const money = (v: string) => formatTL(v, locale);
    return (
        <>
            <Heading
                icon={<Repeat />}
                title={t('billing.admin.subs.title')}
                description={t('billing.admin.subs.description')}
            />
            <div className="grid gap-4">
                <FilterLinks
                    base="/subscriptions"
                    current={status ?? 'all'}
                    options={[
                        ...STATUSES.map((s) => ({
                            value: s,
                            label: t(`billing.status.${s}`),
                        })),
                        { value: 'all', label: t('billing.admin.subs.all') },
                    ]}
                />
                <div className="overflow-x-auto rounded-xl border bg-card">
                    <table
                        className="w-full text-sm"
                        data-testid="subscriptions"
                    >
                        <thead className="border-b bg-muted/30 text-left text-muted-foreground">
                            <tr>
                                <th className="px-4 py-2 font-medium">
                                    {t('billing.admin.subs.company')}
                                </th>
                                <th className="px-4 py-2 font-medium">
                                    {t('billing.admin.subs.plan')}
                                </th>
                                <th className="px-4 py-2 font-medium">
                                    {t('billing.admin.subs.status')}
                                </th>
                                <th className="px-4 py-2 font-medium">
                                    {t('billing.admin.subs.periodEnd')}
                                </th>
                                <th className="px-4 py-2 text-right font-medium">
                                    {t('billing.admin.subs.monthly')}
                                </th>
                                <th className="px-4 py-2 text-right font-medium">
                                    {t('billing.admin.subs.open')}
                                </th>
                            </tr>
                        </thead>
                        <tbody>
                            {rows.length === 0 && (
                                <tr>
                                    <td
                                        colSpan={6}
                                        className="py-10 text-center text-muted-foreground"
                                    >
                                        {t('billing.admin.subs.empty')}
                                    </td>
                                </tr>
                            )}
                            {rows.map((r) => (
                                <tr
                                    key={r.tenant.id}
                                    className="border-b last:border-0"
                                >
                                    <td className="px-4 py-2">
                                        <Link
                                            href={`/tenants/${r.tenant.id}`}
                                            className="font-medium hover:underline"
                                        >
                                            {r.tenant.name}
                                        </Link>
                                        <div className="text-xs text-muted-foreground">
                                            {r.tenant.slug}
                                        </div>
                                    </td>
                                    <td className="px-4 py-2">
                                        {(locale === 'tr'
                                            ? r.plan_name_tr
                                            : r.plan_name_en) ||
                                            r.plan_key ||
                                            '—'}
                                        {r.billing_cycle && (
                                            <span className="text-muted-foreground">
                                                {' · '}
                                                {t(
                                                    `billing.cycle.${r.billing_cycle}`,
                                                )}
                                            </span>
                                        )}
                                    </td>
                                    <td className="px-4 py-2">
                                        <SubscriptionBadge status={r.status} />
                                    </td>
                                    <td className="px-4 py-2">
                                        {formatDate(
                                            r.period_end ?? r.trial_ends_at,
                                        )}
                                    </td>
                                    <td className="px-4 py-2 text-right">
                                        {money(r.monthly_value)}
                                    </td>
                                    <td className="px-4 py-2 text-right">
                                        {money(r.open_amount)}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            </div>
        </>
    );
}
