import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { SmtpSettings } from '@/lib/types';
import { SmtpForm } from './smtp-form';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('settings.tabs.email') };
}

export default async function EmailSettingsPage() {
    const me = await requirePermission('settings.read');
    const smtp = (await api<SmtpSettings>('/settings/smtp')).data;
    return (
        <div className="max-w-5xl">
            <SmtpForm
                smtp={smtp}
                readOnly={!can(me, 'settings.update')}
                defaultTestTo={me.user.email}
            />
        </div>
    );
}
