import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { ArrowLeft, LockKeyhole } from 'lucide-react';
import { getT } from '@/i18n/server';
import { AuthCard, AuthLink } from '../_components/shell';
import { ResetPasswordForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.reset.title') };
}

export default async function ResetPasswordPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const token = (await searchParams).token;
    if (!token) redirect('/forgot-password');
    const t = await getT();
    return (
        <AuthCard
            icon={LockKeyhole}
            title={t('auth.reset.title')}
            description={t('auth.reset.description')}
            footer={
                <AuthLink href="/login" className="gap-1.5">
                    <ArrowLeft className="size-4" aria-hidden />
                    {t('auth.forgot.backToSignIn')}
                </AuthLink>
            }
        >
            <ResetPasswordForm token={token} />
        </AuthCard>
    );
}
