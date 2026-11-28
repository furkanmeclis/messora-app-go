import type { Metadata } from 'next';
import { Landmark } from 'lucide-react';
import { getT } from '@/i18n/server';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Heading } from '../heading';
import { BillingSettingsForm } from './settings-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('billing.admin.settings.title') };
}

/** Seller identity, bank accounts and prices. */
export default async function BillingSettingsPage() {
    const [t, settings] = await Promise.all([
        getT(),
        papi<Schemas['PlatformBillingSettings']>(
            '/platform/billing/settings',
        ).then((r) => r.data),
    ]);
    return (
        <>
            <Heading
                icon={<Landmark />}
                title={t('billing.admin.settings.title')}
                description={t('billing.admin.settings.description')}
            />
            <BillingSettingsForm settings={settings} />
        </>
    );
}
