import type { Metadata } from 'next';
import { requirePermission } from '@/lib/server/auth';
import { getT } from '@/i18n/server';
import { PageHeading } from '../../panel-header';
import { loadWizardData } from '../wizard-data';
import { CampaignWizard } from '../wizard';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('campaigns.newCampaign') };
}

export default async function NewCampaignPage() {
    const me = await requirePermission('campaigns.create');
    const [data, t] = await Promise.all([loadWizardData(me), getT()]);
    return (
        <>
            <PageHeading
                title={t('campaigns.newCampaign')}
                description={t('campaigns.newDescription')}
            />
            <CampaignWizard data={data} />
        </>
    );
}
