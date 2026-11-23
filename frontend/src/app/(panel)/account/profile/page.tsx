import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { requireMe } from '@/lib/server/auth';
import { getDisplayPrefs } from '@/lib/server/display';
import { PageHeading } from '../../panel-header';
import { ProfileHero } from '../profile-hero';
import { AccountAside, ProfileForm } from './profile-form';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('account.profile.title') };
}

export default async function ProfilePage() {
    const [me, t, prefs] = await Promise.all([
        requireMe(),
        getT(),
        getDisplayPrefs(),
    ]);
    return (
        <>
            <PageHeading
                title={t('account.profile.title')}
                description={t('account.profile.description')}
            />
            <ProfileHero user={me.user} prefs={prefs} />
            <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
                <ProfileForm user={me.user} />
                <AccountAside
                    permissionCount={me.permissions.length}
                    mfaEnabled={me.user.mfa_enabled}
                />
            </div>
        </>
    );
}
