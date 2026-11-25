import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Offer, Template } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { NewOfferButton, OffersLive, OffersTable } from './offers-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('offers.title') };
}

export default async function OffersPage() {
    const me = await requirePermission('templates.read');
    const [offers, templates] = await Promise.all([
        api<Offer[]>('/offers').then((r) => r.data),
        api<Template[]>('/templates').then((r) => r.data),
    ]);
    const manage = can(me, 'templates.manage');
    const t = await getT();
    return (
        <>
            <PageHeading
                title={t('offers.title')}
                description={t('offers.description')}
                actions={manage && <NewOfferButton templates={templates} />}
            />
            {can(me, 'campaigns.read') && <OffersLive />}
            <OffersTable
                offers={offers}
                templates={templates}
                manage={manage}
            />
        </>
    );
}
