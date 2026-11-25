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

export default async function NewCampaignPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const me = await requirePermission('campaigns.create');
    // "Use in a campaign" from the templates page: name|language.
    const template = ((await searchParams).template ?? '').slice(0, 600);
    const [data, t] = await Promise.all([loadWizardData(me), getT()]);
    return (
        <>
            <PageHeading
                title={t('campaigns.newCampaign')}
                description={t('campaigns.newDescription')}
            />
            <CampaignWizard data={data} initialTemplate={template} />
        </>
    );
}
