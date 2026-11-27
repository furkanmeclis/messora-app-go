import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { ShieldCheck } from 'lucide-react';
import { getT } from '@/i18n/server';
import { safeNext } from '@/lib/server/actions';
import { getMfaToken } from '@/lib/server/session';
import { AuthCard, AuthLink } from '../../_components/shell';
import { VerifyForm } from '../../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.verify.title') };
}

export default async function VerifyPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    if (!(await getMfaToken())) redirect('/login');
    const sp = await searchParams;
    const t = await getT();
    return (
        <AuthCard
            icon={ShieldCheck}
            title={t('auth.verify.title')}
            description={t('auth.verify.description')}
            footer={
                <AuthLink href="/logout" prefetch={false}>
                    {t('auth.verify.backToSignIn')}
                </AuthLink>
            }
        >
            <VerifyForm next={safeNext(sp.next)} />
        </AuthCard>
    );
}
