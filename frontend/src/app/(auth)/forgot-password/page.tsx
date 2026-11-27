import type { Metadata } from 'next';
import { ArrowLeft, Info, KeyRound } from 'lucide-react';
import { getT } from '@/i18n/server';
import { getCapabilities } from '@/lib/server/auth-context';
import { FormAlert } from '../_components/fields';
import { AuthCard, AuthLink } from '../_components/shell';
import { ForgotPasswordForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.forgot.metaTitle') };
}

export default async function ForgotPasswordPage() {
    const [caps, t] = await Promise.all([getCapabilities(), getT()]);
    return (
        <AuthCard
            icon={KeyRound}
            title={t('auth.forgot.title')}
            description={t('auth.forgot.description')}
            footer={
                <AuthLink href="/login" className="gap-1.5">
                    <ArrowLeft className="size-4" aria-hidden />
                    {t('auth.forgot.backToSignIn')}
                </AuthLink>
            }
        >
            {caps.password_reset ? (
                <ForgotPasswordForm />
            ) : (
                <FormAlert tone="info">
                    <Info
                        className="mt-0.5 size-4 shrink-0 text-primary"
                        aria-hidden
                    />
                    <span>{t('auth.forgot.noEmail')}</span>
                </FormAlert>
            )}
        </AuthCard>
    );
}
