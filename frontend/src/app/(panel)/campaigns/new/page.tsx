import type { Metadata } from 'next';
import { requirePermission } from '@/lib/server/auth';
import { PageHeading } from '../../panel-header';
import { loadWizardData } from '../wizard-data';
import { CampaignWizard } from '../wizard';

export const metadata: Metadata = { title: 'New campaign' };

export default async function NewCampaignPage() {
    const me = await requirePermission('campaigns.create');
    const data = await loadWizardData(me);
    return (
        <>
            <PageHeading
                title="New campaign"
                description="Choose a sender, an approved template, the values for its variables and who receives it."
            />
            <CampaignWizard data={data} />
        </>
    );
}
