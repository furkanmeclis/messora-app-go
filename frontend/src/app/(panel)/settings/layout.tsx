import { getT } from '@/i18n/server';
import { requireMe } from '@/lib/server/auth';
import { PageHeading } from '../panel-header';
import { SettingsTabs } from './settings-tabs';

export default async function SettingsLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    const [me, t] = await Promise.all([requireMe(), getT()]);
    const has = (p: string) => me.permissions.includes(p);
    const tabs = [
        has('settings.read') && {
            href: '/settings',
            label: t('settings.tabs.general'),
            icon: 'general',
        },
        has('settings.read') && {
            href: '/settings/appearance',
            label: t('settings.tabs.appearance'),
            icon: 'appearance',
        },
        has('settings.read') && {
            href: '/settings/pwa',
            label: t('pwa.tab'),
            icon: 'pwa',
        },
        has('settings.read') && {
            href: '/settings/email',
            label: t('settings.tabs.email'),
            icon: 'email',
        },
        has('settings.read') && {
            href: '/settings/consent',
            label: t('consent.settings.tab'),
            icon: 'consent',
        },
        has('settings.read') && {
            href: '/settings/security',
            label: t('settings.tabs.security'),
            icon: 'security',
        },
        has('settings.read') && {
            href: '/settings/license',
            label: t('license.tab'),
            icon: 'license',
        },
        has('settings.read') && {
            href: '/settings/meta-rates',
            label: t('settings.tabs.metaRates'),
            icon: 'metaRates',
        },
        has('webhooks.manage') && {
            href: '/settings/webhooks',
            label: t('settings.tabs.webhooks'),
            icon: 'webhooks',
        },
    ].filter(Boolean) as { href: string; label: string; icon: string }[];
    return (
        <>
            <PageHeading
                title={t('settings.title')}
                description={t('settings.description')}
            />
            <SettingsTabs tabs={tabs} />
            <div className="mt-6">{children}</div>
        </>
    );
}
