import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { ContactGroup } from '@/lib/types';
import { PageHeading } from '../../../panel-header';
import { AudienceInsightsSection, NotFoundAudience } from '../../audience';
import { UUID_RE } from '../../shared';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('contacts.groups.metaTitle') };
}

export default async function GroupPage({
    params,
}: {
    params: Promise<{ groupId: string }>;
}) {
    const me = await requirePermission('contacts.read');
    const { groupId } = await params;
    const t = await getT();
    const group = UUID_RE.test(groupId)
        ? (await api<ContactGroup[]>('/groups')).data.find(
              (g) => g.id === groupId,
          )
        : undefined;
    if (!group) {
        return (
            <NotFoundAudience
                title={t('insights.group.notFound')}
                back={{
                    href: '/contacts/groups',
                    label: t('insights.group.back'),
                }}
            />
        );
    }
    return (
        <>
            <PageHeading
                title={group.name}
                description={[
                    t('insights.group.summary', { count: group.member_count }),
                    group.description,
                ]
                    .filter(Boolean)
                    .join(' · ')}
            />
            <AudienceInsightsSection
                me={me}
                path={`/groups/${group.id}`}
                contactsHref={`/contacts?group_id=${group.id}`}
            />
        </>
    );
}
