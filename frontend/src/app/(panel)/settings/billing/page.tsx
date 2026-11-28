import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Schemas } from '@/lib/types';
import { BillingPage } from './billing-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('billing.title') };
}

export default async function BillingSettingsPage() {
    const me = await requirePermission('settings.read');
    const [overview, proformas, invoices] = await Promise.all([
        api<Schemas['BillingOverview']>('/billing').then((r) => r.data),
        api<Schemas['Proforma'][]>('/billing/proformas').then((r) => r.data),
        api<Schemas['Invoice'][]>('/billing/invoices').then((r) => r.data),
    ]);
    return (
        <div className="max-w-6xl">
            <BillingPage
                overview={overview}
                proformas={proformas}
                invoices={invoices}
                canManage={can(me, 'settings.update') && !me.support}
            />
        </div>
    );
}
