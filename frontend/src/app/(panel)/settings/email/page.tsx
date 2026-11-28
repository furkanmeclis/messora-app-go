import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Schemas, SmtpSettings } from '@/lib/types';
import { EmailSender } from './sender';
import { SmtpForm } from './smtp-form';
import { CustomSmtpLocked } from './locked';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('settings.tabs.email') };
}

/**
 * How the workspace's e-mails go out: Messora's sender (every plan) and,
 * on Enterprise plans, the company's own SMTP server.
 */
export default async function EmailSettingsPage() {
    const me = await requirePermission('settings.read');
    const email = (await api<Schemas['EmailSettings']>('/settings/email')).data;
    const smtp = email.custom_smtp_allowed
        ? (await api<SmtpSettings>('/settings/smtp')).data
        : null;
    const readOnly = !can(me, 'settings.update');
    return (
        <div className="grid max-w-5xl gap-6">
            <EmailSender email={email} readOnly={readOnly} />
            {smtp ? (
                <SmtpForm
                    smtp={smtp}
                    readOnly={readOnly}
                    defaultTestTo={me.user.email}
                    customHint
                />
            ) : (
                <CustomSmtpLocked />
            )}
        </div>
    );
}
