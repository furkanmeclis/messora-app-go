import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { requirePermission } from '@/lib/server/auth';
import { PageHeading } from '../../panel-header';
import { ProfileEditor } from '../profile-editor';
import { loadEditorData } from '../editor-data';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('ai.profiles.new') };
}

export default async function NewAIProfilePage() {
    const me = await requirePermission('ai.manage');
    const [data, t] = await Promise.all([loadEditorData(me), getT()]);
    return (
        <>
            <PageHeading
                title={t('ai.profiles.new')}
                description={t('ai.profiles.description')}
            />
            <ProfileEditor profile={null} data={data} canManage />
        </>
    );
}
