import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { WhatsAppAccount } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { TemplateBuilder } from './builder';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('templates.create.title') };
}

export default async function NewTemplatePage() {
    const me = await requirePermission('templates.manage');
    // Templates belong to a WhatsApp account; listing them needs channels.read.
    if (!can(me, 'channels.read')) redirect('/templates');
    const accounts = (
        await api<WhatsAppAccount[]>('/channels/accounts')
    ).data.map((a) => ({ id: a.id, name: a.name }));
    const t = await getT();
    return (
        <>
            <PageHeading
                title={t('templates.create.title')}
                description={t('templates.builder.description')}
            />
            <TemplateBuilder accounts={accounts} />
        </>
    );
}
