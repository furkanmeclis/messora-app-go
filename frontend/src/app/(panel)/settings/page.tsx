import type { Metadata } from 'next';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Settings } from '@/lib/types';
import { PageHeading } from '../panel-header';
import { SettingsForm } from './settings-form';

export const metadata: Metadata = { title: 'Settings' };

export default async function SettingsPage() {
    const me = await requirePermission('settings.read');
    const settings = (await api<Settings>('/settings')).data;
    return (
        <>
            <PageHeading
                title="Settings"
                description="Workspace-wide configuration."
            />
            <Card className="max-w-2xl">
                <CardHeader>
                    <CardTitle>Workspace</CardTitle>
                    <CardDescription>
                        {can(me, 'settings.update')
                            ? 'Changes apply to everyone and are recorded in the audit log.'
                            : 'You can view these settings; changing them needs the settings.update permission.'}
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <SettingsForm
                        settings={settings}
                        readOnly={!can(me, 'settings.update')}
                    />
                </CardContent>
            </Card>
        </>
    );
}
