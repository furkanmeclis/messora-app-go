import type { Metadata } from 'next';
import Link from 'next/link';
import { Info } from 'lucide-react';
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
import type { Schemas } from '@/lib/types';
import { ForgotPasswordForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.forgot.metaTitle') };
}

export default async function ForgotPasswordPage() {
    const caps = (
        await api<Schemas['AuthCapabilities']>('/auth/capabilities', {
            auth: 'none',
        })
    ).data;
    const t = await getT();
    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('auth.forgot.title')}</CardTitle>
                <CardDescription>
                    {t('auth.forgot.description')}
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
                {caps.password_reset ? (
                    <ForgotPasswordForm />
                ) : (
                    <Alert>
                        <Info />
                        <AlertDescription>
                            {t('auth.forgot.noEmail')}
                        </AlertDescription>
                    </Alert>
                )}
                <p className="text-center text-sm">
                    <Link
                        href="/login"
                        className="text-muted-foreground underline-offset-4 hover:underline"
                    >
                        {t('auth.forgot.backToSignIn')}
                    </Link>
                </p>
            </CardContent>
        </Card>
    );
}
