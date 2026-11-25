import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { Tag } from '@/lib/types';
import { PageHeading } from '../../../panel-header';
import { AudienceInsightsSection, NotFoundAudience } from '../../audience';
import { UUID_RE } from '../../shared';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('contacts.nav.tags') };
}

export default async function TagPage({
    params,
}: {
    params: Promise<{ tagId: string }>;
}) {
    const me = await requirePermission('contacts.read');
    const { tagId } = await params;
    const t = await getT();
    const tag = UUID_RE.test(tagId)
        ? (await api<Tag[]>('/tags')).data.find((x) => x.id === tagId)
        : undefined;
    if (!tag) {
        return (
            <NotFoundAudience
                title={t('insights.tag.notFound')}
                back={{ href: '/contacts/tags', label: t('insights.tag.back') }}
            />
        );
    }
    return (
        <>
            <PageHeading
                title={`#${tag.name}`}
                description={t('insights.tag.summary', {
                    count: tag.contact_count,
                })}
            />
            <AudienceInsightsSection
                me={me}
                path={`/tags/${tag.id}`}
                contactsHref={`/contacts?tag_id=${tag.id}`}
            />
        </>
    );
}
