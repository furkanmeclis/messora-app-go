import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { ContactGroup, Tag } from '@/lib/types';
import { PageHeading } from '../../../panel-header';
import { SegmentBuilder } from '../segment-builder';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('contacts.segments.new') };
}

export default async function NewSegmentPage() {
    const me = await requirePermission('segments.manage');
    const canContacts = can(me, 'contacts.read');
    const t = await getT();
    const [tags, groups] = canContacts
        ? await Promise.all([
              api<Tag[]>('/tags').then((r) => r.data),
              api<ContactGroup[]>('/groups').then((r) => r.data),
          ])
        : [[], []];
    return (
        <>
            <PageHeading
                title={t('contacts.segments.new')}
                description={t('contacts.segments.newHint')}
            />
            <SegmentBuilder
                tags={tags.map((tag) => tag.name)}
                groups={groups.map((g) => ({ id: g.id, name: g.name }))}
                canManage
                canViewContacts={canContacts}
            />
        </>
    );
}
