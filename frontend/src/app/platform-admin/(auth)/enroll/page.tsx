import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { ShieldCheck } from 'lucide-react';
import { getT } from '@/i18n/server';
import { requirePlatformMe } from '@/lib/server/platform';
import { AuthCard } from '@/app/(auth)/_components/shell';
import { EnrollForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.enroll.metaTitle') };
}

/** Platform admins must turn two-factor authentication on first. */
export default async function EnrollPage() {
    const me = await requirePlatformMe({ allowMfaPending: true });
    if (!me.mfa_required) redirect('/');
    const t = await getT();
    return (
        <AuthCard
            icon={ShieldCheck}
            title={t('platform.enroll.title')}
            description={t('platform.enroll.description')}
        >
            <EnrollForm />
        </AuthCard>
    );
}
