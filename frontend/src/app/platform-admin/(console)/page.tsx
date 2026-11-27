import type { Metadata } from 'next';
import Link from 'next/link';
import { Gauge } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { getT } from '@/i18n/server';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Heading } from './heading';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.overview.title') };
}

export default async function OverviewPage() {
    const [t, o] = await Promise.all([
        getT(),
        papi<Schemas['PlatformOverview']>('/platform/overview').then(
            (r) => r.data,
        ),
    ]);
    const tiles = [
        {
            label: t('platform.overview.tenants'),
            value: o.tenants,
            href: '/tenants',
        },
        {
            label: t('platform.overview.active'),
            value: o.active_tenants,
            href: '/tenants?status=active',
        },
        {
            label: t('platform.overview.suspended'),
            value: o.suspended_tenants,
            href: '/tenants?status=suspended',
        },
        {
            label: t('platform.overview.trial'),
            value: o.trial_tenants,
            href: '/tenants',
        },
        { label: t('platform.overview.users'), value: o.users, href: '/users' },
        {
            label: t('platform.overview.signups'),
            value: o.signups_7d,
            href: '/tenants',
        },
        {
            label: t('platform.overview.support'),
            value: o.active_support_sessions,
            href: '/support',
        },
    ];
    return (
        <>
            <Heading
                icon={<Gauge />}
                title={t('platform.overview.title')}
                description={t('platform.overview.description')}
            />
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
                {tiles.map((tile) => (
                    <Link
                        key={tile.label}
                        href={tile.href}
                        className="rounded-xl focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
                    >
                        <Card className="h-full transition-colors hover:border-primary/40">
                            <CardHeader>
                                <CardTitle className="text-sm font-medium text-muted-foreground">
                                    {tile.label}
                                </CardTitle>
                            </CardHeader>
                            <CardContent>
                                <span
                                    className="font-display text-3xl font-bold tabular-nums"
                                    data-testid="overview-tile"
                                >
                                    {tile.value}
                                </span>
                            </CardContent>
                        </Card>
                    </Link>
                ))}
            </div>
        </>
    );
}
