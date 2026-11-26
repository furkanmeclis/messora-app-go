'use client';

import { Activity, ServerCog } from 'lucide-react';
import { TabNav } from '@/components/tab-nav';
import { useT } from '@/i18n/client';

/** Operations / System tabs of the overview. */
export function OverviewTabs({
    tab,
    failedJobs,
}: {
    tab: 'ops' | 'system';
    failedJobs?: number;
}) {
    const t = useT();
    return (
        <TabNav
            className="mb-6"
            label={t('overview.ops.tabs.label')}
            items={[
                {
                    href: '/',
                    label: t('overview.ops.tabs.ops'),
                    icon: Activity,
                    active: tab === 'ops',
                },
                {
                    href: '/?tab=system',
                    label: t('overview.ops.tabs.system'),
                    icon: ServerCog,
                    active: tab === 'system',
                    count: failedJobs || undefined,
                },
            ]}
        />
    );
}
