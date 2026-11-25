'use client';

import { usePathname } from 'next/navigation';
import { FileText, Gift } from 'lucide-react';
import { TabNav } from '@/components/tab-nav';
import { useT } from '@/i18n/client';

/** Templates | Offers tabs. */
export function TemplatesNav() {
    const t = useT();
    const pathname = usePathname();
    const offers = pathname.startsWith('/templates/offers');
    return (
        <TabNav
            className="mb-6"
            label={t('offers.nav.label')}
            items={[
                {
                    href: '/templates',
                    label: t('offers.nav.templates'),
                    icon: FileText,
                    active: !offers,
                },
                {
                    href: '/templates/offers',
                    label: t('offers.nav.offers'),
                    icon: Gift,
                    active: offers,
                },
            ]}
        />
    );
}
