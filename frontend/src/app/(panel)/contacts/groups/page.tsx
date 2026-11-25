import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { ContactGroup } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { GroupDialog, GroupTable } from './groups-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('contacts.groups.metaTitle') };
}

export default async function GroupsPage() {
    const me = await requirePermission('contacts.read');
    const groups = (await api<ContactGroup[]>('/groups')).data;
    const canManage = can(me, 'contacts.update');
    const t = await getT();
    return (
        <>
            <PageHeading
                title={t('contacts.nav.groups')}
                description={t('contacts.groups.description')}
                actions={canManage && <GroupDialog />}
            />
            <GroupTable groups={groups} canManage={canManage} />
        </>
    );
}
