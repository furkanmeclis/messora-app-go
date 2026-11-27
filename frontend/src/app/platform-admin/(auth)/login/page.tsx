import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { Info, ShieldCheck } from 'lucide-react';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { tryPlatformMe } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { FormAlert } from '@/app/(auth)/_components/fields';
import { AuthCard } from '@/app/(auth)/_components/shell';
import { PlatformLoginForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.login.metaTitle') };
}

export default async function PlatformLoginPage() {
    const me = await tryPlatformMe().catch(() => null);
    if (me && me !== 'invalid') redirect('/');
    const [t, caps] = await Promise.all([
        getT(),
        api<Schemas['PlatformAuthCapabilities']>(
            '/platform/auth/capabilities',
            { auth: 'none' },
        )
            .then((r) => r.data)
            .catch(() => ({ bootstrap_required: false })),
    ]);
    return (
        <AuthCard
            icon={ShieldCheck}
            title={t('platform.login.title')}
            description={t('platform.login.description')}
        >
            <div className="flex flex-col gap-5">
                {caps.bootstrap_required && (
                    <FormAlert tone="info">
                        <Info
                            className="mt-0.5 size-4 shrink-0 text-primary"
                            aria-hidden
                        />
                        <span>{t('platform.login.bootstrapHint')}</span>
                    </FormAlert>
                )}
                <PlatformLoginForm />
            </div>
        </AuthCard>
    );
}
