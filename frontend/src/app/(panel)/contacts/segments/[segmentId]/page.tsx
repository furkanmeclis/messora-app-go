import type { Metadata } from 'next';
import Link from 'next/link';
import { ChartColumn, Filter } from 'lucide-react';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { AudienceInsightsSection } from '../../audience';
import { Button } from '@/components/ui/button';
import {
    Empty,
    EmptyContent,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { getT } from '@/i18n/server';
import { api, ApiError } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { ContactGroup, Segment, Tag } from '@/lib/types';
import { PageHeading } from '../../../panel-header';
import { UUID_RE } from '../../shared';
import { SegmentBuilder } from '../segment-builder';
import { NUMBER_LOCALE } from '@/lib/format';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('contacts.segments.metaTitle') };
}

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
    const t = await getT();
    if (!segment) {
        return (
            <Empty className="py-16">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <Filter />
                    </EmptyMedia>
                    <EmptyTitle>{t('contacts.segments.notFound')}</EmptyTitle>
                    <EmptyDescription>
                        {t('contacts.detail.notFoundHint')}
                    </EmptyDescription>
                </EmptyHeader>
                <EmptyContent>
                    <Button variant="outline" asChild>
                        <Link href="/contacts/segments">
                            {t('contacts.segments.back')}
                        </Link>
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
                        ? t('contacts.segments.invalid')
                        : t('contacts.segments.matchNow', {
                              count,
                              formatted: count.toLocaleString(NUMBER_LOCALE),
                          })
                }
            />
            <Tabs defaultValue={count == null ? 'definition' : 'insights'}>
                <TabsList>
                    <TabsTrigger value="insights" disabled={count == null}>
                        <ChartColumn /> {t('insights.tabs.insights')}
                    </TabsTrigger>
                    <TabsTrigger value="definition">
                        <Filter /> {t('insights.tabs.definition')}
                    </TabsTrigger>
                </TabsList>
                <TabsContent value="insights" className="pt-4">
                    {count != null && (
                        <AudienceInsightsSection
                            me={me}
                            path={`/segments/${segment.id}`}
                            contactsHref={
                                canContacts
                                    ? `/contacts?segment_id=${segment.id}`
                                    : undefined
                            }
                        />
                    )}
                </TabsContent>
                <TabsContent value="definition" className="pt-4">
                    <SegmentBuilder
                        key={segment.updated_at}
                        segment={segment}
                        tags={tags.map((tag) => tag.name)}
                        groups={groups.map((g) => ({ id: g.id, name: g.name }))}
                        canManage={can(me, 'segments.manage')}
                        canViewContacts={canContacts}
                    />
                </TabsContent>
            </Tabs>
        </>
    );
}
