import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { can, requirePermission } from '@/lib/server/auth';
import { PageHeading } from '../../panel-header';
import { loadAutomation, loadEditorOptions } from '../data';
import { AutomationEditor } from '../editor';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('automations.editor.metaTitle') };
}

export default async function AutomationPage({
    params,
}: {
    params: Promise<{ id: string }>;
}) {
    const me = await requirePermission('automations.read');
    const t = await getT();
    const { id } = await params;
    const [automation, options] = await Promise.all([
        loadAutomation(id),
        loadEditorOptions(me),
    ]);
    return (
        <>
            <PageHeading
                title={automation.name}
                description={t('automations.editor.version', {
                    version: automation.version,
                    status: automation.active
                        ? t('automations.status.active')
                        : t('automations.status.inactive'),
                })}
            />
            <AutomationEditor
                key={`${automation.id}-${automation.version}`}
                automation={automation}
                options={options}
                canManage={can(me, 'automations.manage')}
            />
        </>
    );
}
