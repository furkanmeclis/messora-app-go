import type { Metadata } from 'next';
import Link from 'next/link';
import { Filter } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
    Empty,
    EmptyContent,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { api, ApiError } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { ContactGroup, Segment, Tag } from '@/lib/types';
import { PageHeading } from '../../../panel-header';
import { UUID_RE } from '../../shared';
import { SegmentBuilder } from '../segment-builder';

export const metadata: Metadata = { title: 'Segment' };

async function loadSegment(id: string): Promise<Segment | null> {
    if (!UUID_RE.test(id)) return null;
    try {
        return (await api<Segment>(`/segments/${id}`)).data;
    } catch (err) {
        if (err instanceof ApiError && err.status === 404) return null;
        throw err;
    }
}

export default async function SegmentPage({
    params,
}: {
    params: Promise<{ segmentId: string }>;
}) {
    const me = await requirePermission('segments.read');
    const { segmentId } = await params;
    const segment = await loadSegment(segmentId);
    if (!segment) {
        return (
            <Empty className="py-16">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <Filter />
                    </EmptyMedia>
                    <EmptyTitle>Segment not found</EmptyTitle>
                    <EmptyDescription>
                        It may have been deleted, or the link is wrong.
                    </EmptyDescription>
                </EmptyHeader>
                <EmptyContent>
                    <Button variant="outline" asChild>
                        <Link href="/contacts/segments">Back to segments</Link>
                    </Button>
                </EmptyContent>
            </Empty>
        );
    }
    const canContacts = can(me, 'contacts.read');
    const [tags, groups] = canContacts
        ? await Promise.all([
              api<Tag[]>('/tags').then((r) => r.data),
              api<ContactGroup[]>('/groups').then((r) => r.data),
          ])
        : [[], []];
    const count = segment.contact_count;
    return (
        <>
            <PageHeading
                title={segment.name}
                description={
                    count == null
                        ? 'The saved rules are no longer valid; fix them below.'
                        : `${count.toLocaleString()} contact${count === 1 ? '' : 's'} match right now.`
                }
            />
            <SegmentBuilder
                key={segment.updated_at}
                segment={segment}
                tags={tags.map((t) => t.name)}
                groups={groups.map((g) => ({ id: g.id, name: g.name }))}
                canManage={can(me, 'segments.manage')}
                canViewContacts={canContacts}
            />
        </>
    );
}
