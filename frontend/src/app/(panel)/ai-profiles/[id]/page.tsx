import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { getT } from '@/i18n/server';
import { api, ApiError } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import { UUID_RE } from '@/lib/server/stream';
import type { Schemas } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { ProfileEditor } from '../profile-editor';
import { loadEditorData } from '../editor-data';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('ai.profiles.title') };
}

export default async function AIProfilePage({
    params,
}: {
    params: Promise<{ id: string }>;
}) {
    const { id } = await params;
    if (!UUID_RE.test(id)) notFound();
    const me = await requirePermission('ai.read');
    let profile: Schemas['AIProfile'];
    try {
        profile = (await api<Schemas['AIProfile']>(`/ai/profiles/${id}`)).data;
    } catch (err) {
        if (err instanceof ApiError && err.status === 404) notFound();
        throw err;
    }
    const [data, t] = await Promise.all([loadEditorData(me), getT()]);
    return (
        <>
            <PageHeading
                title={profile.name}
                description={t('ai.profiles.description')}
            />
            <ProfileEditor
                profile={profile}
                data={data}
                canManage={can(me, 'ai.manage')}
            />
        </>
    );
}
