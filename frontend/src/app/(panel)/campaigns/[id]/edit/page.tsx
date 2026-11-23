import type { Metadata } from 'next';
import { notFound, redirect } from 'next/navigation';
import { api, ApiError } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import { getT } from '@/i18n/server';
import type { Campaign } from '@/lib/types';
import { PageHeading } from '../../../panel-header';
import { loadWizardData } from '../../wizard-data';
import { CampaignWizard } from '../../wizard';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('campaigns.editCampaign') };
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export default async function EditCampaignPage({
    params,
}: {
    params: Promise<{ id: string }>;
}) {
    const { id } = await params;
    if (!UUID.test(id)) notFound();
    const me = await requirePermission('campaigns.create');
    let campaign: Campaign;
    try {
        campaign = (await api<Campaign>(`/campaigns/${id}`)).data;
    } catch (err) {
        if (err instanceof ApiError && err.status === 404) notFound();
        throw err;
    }
    // Only drafts and scheduled campaigns can change.
    if (campaign.status !== 'draft' && campaign.status !== 'scheduled') {
        redirect(`/campaigns/${id}`);
    }
    const [data, t] = await Promise.all([loadWizardData(me), getT()]);
    return (
        <>
            <PageHeading
                title={t('campaigns.editTitle', { name: campaign.name })}
                description={
                    campaign.status === 'scheduled'
                        ? t('campaigns.editScheduledHint')
                        : t('campaigns.editDraftHint')
                }
            />
            <CampaignWizard data={data} campaign={campaign} />
        </>
    );
}
