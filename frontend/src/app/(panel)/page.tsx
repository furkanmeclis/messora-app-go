import type { Metadata } from 'next';
import Link from 'next/link';
import { ShieldAlert } from 'lucide-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { getT } from '@/i18n/server';
import { api, ApiError } from '@/lib/server/api';
import { errorMessage } from '@/lib/server/actions';
import { can, requireMe } from '@/lib/server/auth';
import type { SystemStatus } from '@/lib/types';
import { OpsCenter } from './ops-center';
import { OverviewTabs } from './overview-tabs';
import type { OpsOverview } from './ops-actions';
import { PageHeading } from './panel-header';
import { RefreshButton } from './refresh-button';
import { SystemStatusSection } from './system-status';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('overview.title') };
}

/**
 * The operations center: what needs attention now, live (ops-center.tsx).
 * The health of the installation is on the System tab.
 */
export default async function OverviewPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const me = await requireMe();
    const t = await getT();
    const sp = await searchParams;
    const canSystem = can(me, 'settings.read');
    const tab = sp.tab === 'system' && canSystem ? 'system' : 'ops';
    const tz = me.user.timezone || 'UTC';

    let status: SystemStatus | null = null;
    let overview: OpsOverview | null = null;
    let loadError: string | null = null;
    try {
        if (tab === 'system') {
            status = (await api<SystemStatus>('/system/status')).data;
        } else {
            overview = (
                await api<OpsOverview>(
                    `/ops/overview?tz=${encodeURIComponent(tz)}`,
                )
            ).data;
        }
    } catch (err) {
        if (!(err instanceof ApiError)) throw err;
        loadError = await errorMessage(err);
    }
    const firstName = me.user.name.split(' ')[0] ?? '';

    return (
        <>
            <PageHeading
                title={t('overview.welcome', { name: firstName })}
                description={t('overview.description')}
                actions={tab === 'system' && status && <RefreshButton />}
            />

            {!me.user.mfa_enabled && (
                <Alert className="mb-6">
                    <ShieldAlert />
                    <AlertTitle>{t('overview.protect.title')}</AlertTitle>
                    <AlertDescription>
                        <p>{t('overview.protect.body')}</p>
                        <Button asChild size="sm" className="mt-2">
                            <Link href="/account/security">
                                {t('overview.protect.action')}
                            </Link>
                        </Button>
                    </AlertDescription>
                </Alert>
            )}

            {canSystem && (
                <OverviewTabs tab={tab} failedJobs={status?.failed_jobs} />
            )}

            {loadError && (
                <Alert variant="destructive" className="mb-6">
                    <AlertTitle>
                        {tab === 'system'
                            ? t('overview.status.unavailable')
                            : t('overview.ops.unavailable')}
                    </AlertTitle>
                    <AlertDescription>{loadError}</AlertDescription>
                </Alert>
            )}

            {tab === 'system' && status && (
                <SystemStatusSection status={status} t={t} />
            )}
            {tab === 'ops' && overview && (
                <OpsCenter
                    initial={overview}
                    userId={me.user.id}
                    timezone={tz}
                    perms={{
                        pause: can(me, 'campaigns.pause'),
                        start: can(me, 'campaigns.start'),
                        reply: can(me, 'conversations.reply'),
                        assign: can(me, 'conversations.assign'),
                        automations: can(me, 'automations.read'),
                        channels: can(me, 'channels.read'),
                        aiUsage: can(me, 'ai.read'),
                    }}
                />
            )}
        </>
    );
}
