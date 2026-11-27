import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { MailCheck } from 'lucide-react';
import { getT } from '@/i18n/server';
import { AuthCard } from '../_components/shell';
import { VerifyEmail } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.verifyEmail.metaTitle') };
}

export default async function VerifyEmailPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const token = (await searchParams).token;
    if (!token) redirect('/verify-email/sent');
    const t = await getT();
    return (
        <AuthCard icon={MailCheck} title={t('auth.verifyEmail.metaTitle')}>
            <VerifyEmail token={token} />
        </AuthCard>
    );
}
