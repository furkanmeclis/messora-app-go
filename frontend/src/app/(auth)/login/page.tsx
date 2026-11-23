import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { CheckCircle2, Clock } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { safeNext } from '@/lib/server/actions';
import type { Schemas } from '@/lib/types';
import { LoginForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.login.title') };
}

export default async function LoginPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const sp = await searchParams;
    const caps = (
        await api<Schemas['AuthCapabilities']>('/auth/capabilities', {
            auth: 'none',
        })
    ).data;
    if (caps.setup_required) redirect('/setup');
    const t = await getT();

    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('auth.login.title')}</CardTitle>
                <CardDescription>{t('auth.login.description')}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
                {sp.reset && (
                    <Alert>
                        <CheckCircle2 />
                        <AlertDescription>
                            {t('auth.login.passwordChanged')}
                        </AlertDescription>
                    </Alert>
                )}
                {sp.expired && (
                    <Alert>
                        <Clock />
                        <AlertDescription>
                            {t('auth.login.expired')}
                        </AlertDescription>
                    </Alert>
                )}
                <LoginForm
                    next={safeNext(sp.next)}
                    canReset={caps.password_reset}
                />
            </CardContent>
        </Card>
    );
}
