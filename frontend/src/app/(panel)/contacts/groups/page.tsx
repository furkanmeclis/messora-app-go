import type { Metadata } from 'next';
import { Card, CardContent } from '@/components/ui/card';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { ContactGroup } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { GroupDialog, GroupTable } from './groups-client';

export const metadata: Metadata = { title: 'Contact groups' };

export default async function GroupsPage() {
    const me = await requirePermission('contacts.read');
    const groups = (await api<ContactGroup[]>('/groups')).data;
    const canManage = can(me, 'contacts.update');
    return (
        <>
            <PageHeading
                title="Groups"
                description="Hand-picked lists of contacts, for campaigns and filters."
                actions={canManage && <GroupDialog />}
            />
            <Card>
                <CardContent className="p-0">
                    <GroupTable groups={groups} canManage={canManage} />
                </CardContent>
            </Card>
        </>
    );
}
