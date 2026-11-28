import type { Metadata } from 'next';
import { AlertCircle, ShieldPlus } from 'lucide-react';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import type { Schemas } from '@/lib/types';
import { FormAlert } from '@/app/(auth)/_components/fields';
import { AuthCard, AuthLink } from '@/app/(auth)/_components/shell';
import { BootstrapForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.bootstrap.metaTitle') };
}

/** Redeems the one-time link `messora migrate` prints without an admin. */
export default async function BootstrapPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const token = (await searchParams).token ?? '';
    const [t, caps] = await Promise.all([
        getT(),
        api<Schemas['PlatformAuthCapabilities']>(
            '/platform/auth/capabilities',
            { auth: 'none' },
        )
            .then((r) => r.data)
            .catch(() => ({ bootstrap_required: true })),
    ]);
    const problem = !caps.bootstrap_required
        ? t('platform.bootstrap.done')
        : !token
          ? t('platform.bootstrap.missing')
          : null;
    return (
        <AuthCard
            icon={ShieldPlus}
            title={t('platform.bootstrap.title')}
            description={t('platform.bootstrap.description')}
            footer={
                problem ? (
                    <AuthLink href="/login">
                        {t('platform.login.submit')}
                    </AuthLink>
                ) : undefined
            }
        >
            {problem ? (
                <FormAlert>
                    <AlertCircle
                        className="mt-0.5 size-4 shrink-0"
                        aria-hidden
                    />
                    <span>{problem}</span>
                </FormAlert>
            ) : (
                <BootstrapForm token={token} />
            )}
        </AuthCard>
    );
}
