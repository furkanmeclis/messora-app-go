import type { Metadata } from 'next';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { ContactGroup, Tag } from '@/lib/types';
import { PageHeading } from '../../../panel-header';
import { SegmentBuilder } from '../segment-builder';

export const metadata: Metadata = { title: 'New segment' };

export default async function NewSegmentPage() {
    const me = await requirePermission('segments.manage');
    const canContacts = can(me, 'contacts.read');
    const [tags, groups] = canContacts
        ? await Promise.all([
              api<Tag[]>('/tags').then((r) => r.data),
              api<ContactGroup[]>('/groups').then((r) => r.data),
          ])
        : [[], []];
    return (
        <>
            <PageHeading
                title="New segment"
                description="Describe who belongs in it; the count updates as you go."
            />
            <SegmentBuilder
                tags={tags.map((t) => t.name)}
                groups={groups.map((g) => ({ id: g.id, name: g.name }))}
                canManage
                canViewContacts={canContacts}
            />
        </>
    );
}
