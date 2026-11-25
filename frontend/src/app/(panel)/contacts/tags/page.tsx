import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Tag } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { TagDialog, TagTable } from './tags-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('contacts.nav.tags') };
}

export default async function TagsPage() {
    const me = await requirePermission('contacts.read');
    const tags = (await api<Tag[]>('/tags')).data;
    const canManage = can(me, 'contacts.update');
    const t = await getT();
    return (
        <>
            <PageHeading
                title={t('contacts.nav.tags')}
                description={t('contacts.tags.description')}
                actions={canManage && <TagDialog />}
            />
            <TagTable tags={tags} canManage={canManage} />
        </>
    );
}
