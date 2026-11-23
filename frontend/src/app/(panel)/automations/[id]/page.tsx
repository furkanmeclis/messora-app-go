import type { Metadata } from 'next';
import { can, requirePermission } from '@/lib/server/auth';
import { PageHeading } from '../../panel-header';
import { loadAutomation, loadEditorOptions } from '../data';
import { AutomationEditor } from '../editor';

export const metadata: Metadata = { title: 'Automation' };

export default async function AutomationPage({
    params,
}: {
    params: Promise<{ id: string }>;
}) {
    const me = await requirePermission('automations.read');
    const { id } = await params;
    const [automation, options] = await Promise.all([
        loadAutomation(id),
        loadEditorOptions(me),
    ]);
    return (
        <>
            <PageHeading
                title={automation.name}
                description={`Version ${automation.version} · ${automation.active ? 'Active' : 'Inactive'}`}
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
