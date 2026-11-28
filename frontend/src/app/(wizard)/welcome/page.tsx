import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requireMe } from '@/lib/server/auth';
import { hasFeature } from '@/lib/plan';
import type { Settings, SmtpSettings, WhatsAppAccount } from '@/lib/types';
import { Wizard } from './wizard';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('wizard.welcome') };
}

export default async function WelcomePage() {
    const me = await requireMe();
    const [settings, smtp, accounts] = await Promise.all([
        api<Settings>('/settings').then((r) => r.data),
        api<SmtpSettings>('/settings/smtp').then((r) => r.data),
        api<WhatsAppAccount[]>('/channels/accounts')
            .then((r) => r.data)
            .catch(() => [] as WhatsAppAccount[]),
    ]);
    return (
        <Wizard
            settings={settings}
            smtp={smtp}
            accounts={accounts}
            ownerEmail={me.user.email}
            customSmtp={hasFeature(me, 'custom_smtp')}
        />
    );
}
