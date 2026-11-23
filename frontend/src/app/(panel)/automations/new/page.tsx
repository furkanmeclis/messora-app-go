import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { requirePermission } from '@/lib/server/auth';
import { PageHeading } from '../../panel-header';
import { loadEditorOptions } from '../data';
import { AutomationEditor } from '../editor';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('automations.new.title') };
}

export default async function NewAutomationPage() {
    const me = await requirePermission('automations.manage');
    const t = await getT();
    const options = await loadEditorOptions(me);
    return (
        <>
            <PageHeading
                title={t('automations.new.title')}
                description={t('automations.new.description')}
            />
            <AutomationEditor automation={null} options={options} canManage />
        </>
    );
}
