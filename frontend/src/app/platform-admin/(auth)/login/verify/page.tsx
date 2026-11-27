import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { ShieldCheck } from 'lucide-react';
import { getT } from '@/i18n/server';
import { getPlatformMfaToken } from '@/lib/server/session';
import { AuthCard } from '@/app/(auth)/_components/shell';
import { PlatformVerifyForm } from '../../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.login.verifyTitle') };
}

export default async function PlatformVerifyPage() {
    if (!(await getPlatformMfaToken())) redirect('/login');
    const t = await getT();
    return (
        <AuthCard
            icon={ShieldCheck}
            title={t('platform.login.verifyTitle')}
            description={t('platform.login.verifyDescription')}
        >
            <PlatformVerifyForm />
        </AuthCard>
    );
}
