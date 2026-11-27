import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { getAuthContext, getCapabilities } from '@/lib/server/auth-context';
import { AuthCard, AuthLink } from '../_components/shell';
import { SignupForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.signup.metaTitle') };
}

/** Where people who want a workspace go while sign-up is closed. */
const CONTACT_URL = 'mailto:merhaba@messora.tech';

export default async function SignupPage() {
    const [t, { baseDomain }, caps] = await Promise.all([
        getT(),
        getAuthContext(),
        getCapabilities(),
    ]);
    return (
        <AuthCard
            title={t('auth.signup.title')}
            description={t('auth.signup.description')}
            footer={
                <>
                    {t('auth.signup.haveAccount')}{' '}
                    <AuthLink href="/login">{t('auth.signup.signin')}</AuthLink>
                </>
            }
        >
            <SignupForm
                baseDomain={baseDomain}
                contactUrl={CONTACT_URL}
                closed={!caps.signup_enabled}
            />
        </AuthCard>
    );
}
