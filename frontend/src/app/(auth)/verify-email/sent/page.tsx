import type { Metadata } from 'next';
import { MailOpen } from 'lucide-react';
import { getT } from '@/i18n/server';
import { AuthCard, AuthLink } from '../../_components/shell';
import { signupEmail } from '@/lib/server/auth-context';
import { ResendVerificationForm } from '../../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.verifyEmail.sentMetaTitle') };
}

export default async function VerificationSentPage() {
    const [t, email] = await Promise.all([getT(), signupEmail()]);
    return (
        <AuthCard
            icon={MailOpen}
            title={t('auth.verifyEmail.sentTitle')}
            description={
                <>
                    <p>
                        {email
                            ? t('auth.verifyEmail.sentBody', { email })
                            : t('auth.verifyEmail.sentBodyGeneric')}
                    </p>
                    <p className="mt-2 text-sm">
                        {t('auth.verifyEmail.sentHint')}
                    </p>
                </>
            }
            footer={
                <AuthLink href="/signup">
                    {t('auth.verifyEmail.wrongEmail')}
                </AuthLink>
            }
        >
            <ResendVerificationForm email={email} />
        </AuthCard>
    );
}
