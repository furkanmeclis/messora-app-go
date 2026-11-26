'use client';

import { usePathname } from 'next/navigation';
import { ChartColumn, Cpu, PlugZap, SlidersHorizontal } from 'lucide-react';
import { TabNav } from '@/components/tab-nav';
import { useT } from '@/i18n/client';

/** Sub-sections of Settings → AI. */
export function AITabs() {
    const t = useT();
    const path = usePathname();
    return (
        <TabNav
            label={t('ai.title')}
            className="mb-6"
            items={[
                {
                    href: '/settings/ai',
                    label: t('ai.sub.general'),
                    icon: SlidersHorizontal,
                    active: path === '/settings/ai',
                },
                {
                    href: '/settings/ai/providers',
                    label: t('ai.sub.providers'),
                    icon: PlugZap,
                    active: path.startsWith('/settings/ai/providers'),
                },
                {
                    href: '/settings/ai/models',
                    label: t('ai.sub.models'),
                    icon: Cpu,
                    active: path.startsWith('/settings/ai/models'),
                },
                {
                    href: '/settings/ai/usage',
                    label: t('ai.sub.usage'),
                    icon: ChartColumn,
                    active: path.startsWith('/settings/ai/usage'),
                },
            ]}
        />
    );
}
