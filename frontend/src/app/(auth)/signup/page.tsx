import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { getAuthContext } from '@/lib/server/auth-context';
import { AuthCard, AuthLink } from '../_components/shell';
import { SignupForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.signup.metaTitle') };
}

/** Where "coming soon" points people who want a workspace now. */
const CONTACT_URL = 'mailto:merhaba@messora.tech';

export default async function SignupPage() {
    const [t, { baseDomain }] = await Promise.all([getT(), getAuthContext()]);
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
            <SignupForm baseDomain={baseDomain} contactUrl={CONTACT_URL} />
        </AuthCard>
    );
}
