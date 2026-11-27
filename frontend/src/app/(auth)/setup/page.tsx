import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { Rocket, Terminal } from 'lucide-react';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import type { Schemas } from '@/lib/types';
import { FormAlert } from '../_components/fields';
import { AuthCard } from '../_components/shell';
import { SetupForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.setup.title') };
}

export default async function SetupPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const status = (
        await api<Schemas['SetupStatus']>('/setup/status', { auth: 'none' })
    ).data;
    if (!status.setup_required) redirect('/login');
    const token = (await searchParams).token ?? '';
    const t = await getT();

    return (
        <AuthCard
            icon={Rocket}
            title={t('auth.setup.title')}
            description={t('auth.setup.description')}
        >
            {token ? (
                <SetupForm token={token} />
            ) : (
                <FormAlert tone="info">
                    <Terminal
                        className="mt-0.5 size-4 shrink-0 text-primary"
                        aria-hidden
                    />
                    <div className="min-w-0">
                        <p className="font-semibold">
                            {t('auth.setup.openLinkTitle')}
                        </p>
                        <p className="mt-1">{t('auth.setup.openLinkLogs')}</p>
                        <code className="mt-2 block overflow-x-auto rounded-lg bg-muted p-2 text-xs">
                            docker compose logs messora-api | grep setup_url
                        </code>
                        <p className="mt-2">{t('auth.setup.newLink')}</p>
                        <code className="mt-2 block overflow-x-auto rounded-lg bg-muted p-2 text-xs">
                            docker compose exec messora-api messora setup token
                        </code>
                    </div>
                </FormAlert>
            )}
        </AuthCard>
    );
}
