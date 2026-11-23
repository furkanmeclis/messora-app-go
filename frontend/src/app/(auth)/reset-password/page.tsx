import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { getT } from '@/i18n/server';
import { ResetPasswordForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.reset.title') };
}

export default async function ResetPasswordPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const token = (await searchParams).token;
    if (!token) redirect('/forgot-password');
    const t = await getT();
    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('auth.reset.title')}</CardTitle>
                <CardDescription>{t('auth.reset.description')}</CardDescription>
            </CardHeader>
            <CardContent>
                <ResetPasswordForm token={token} />
            </CardContent>
        </Card>
    );
}
