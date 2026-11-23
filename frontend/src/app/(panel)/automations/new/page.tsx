import type { Metadata } from 'next';
import { requirePermission } from '@/lib/server/auth';
import { PageHeading } from '../../panel-header';
import { loadEditorOptions } from '../data';
import { AutomationEditor } from '../editor';

export const metadata: Metadata = { title: 'New automation' };

export default async function NewAutomationPage() {
    const me = await requirePermission('automations.manage');
    const options = await loadEditorOptions(me);
    return (
        <>
            <PageHeading
                title="New automation"
                description="Choose what starts it and what it does. It stays off until you turn it on."
            />
            <AutomationEditor automation={null} options={options} canManage />
        </>
    );
}
