import type { Metadata } from 'next';
import { notFound, redirect } from 'next/navigation';
import { api, ApiError } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { Campaign } from '@/lib/types';
import { PageHeading } from '../../../panel-header';
import { loadWizardData } from '../../wizard-data';
import { CampaignWizard } from '../../wizard';

export const metadata: Metadata = { title: 'Edit campaign' };

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
    const data = await loadWizardData(me);
    return (
        <>
            <PageHeading
                title={`Edit ${campaign.name}`}
                description={
                    campaign.status === 'scheduled'
                        ? 'This campaign is scheduled; changes apply when it starts.'
                        : 'Draft campaign.'
                }
            />
            <CampaignWizard data={data} campaign={campaign} />
        </>
    );
}
