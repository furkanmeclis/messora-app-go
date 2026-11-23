import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { Terminal } from 'lucide-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import type { Schemas } from '@/lib/types';
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
        <Card>
            <CardHeader>
                <CardTitle>{t('auth.setup.title')}</CardTitle>
                <CardDescription>{t('auth.setup.description')}</CardDescription>
            </CardHeader>
            <CardContent>
                {token ? (
                    <SetupForm token={token} />
                ) : (
                    <Alert>
                        <Terminal />
                        <AlertTitle>{t('auth.setup.openLinkTitle')}</AlertTitle>
                        <AlertDescription>
                            <p>{t('auth.setup.openLinkLogs')}</p>
                            <code className="mt-2 block rounded bg-muted p-2 text-xs">
                                docker compose logs messora-api | grep setup_url
                            </code>
                            <p className="mt-2">{t('auth.setup.newLink')}</p>
                            <code className="mt-2 block rounded bg-muted p-2 text-xs">
                                docker compose exec messora-api messora setup
                                token
                            </code>
                        </AlertDescription>
                    </Alert>
                )}
            </CardContent>
        </Card>
    );
}
