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
import { safeNext } from '@/lib/server/actions';
import { getMfaToken } from '@/lib/server/session';
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
        <Card>
            <CardHeader>
                <CardTitle>{t('auth.verify.title')}</CardTitle>
                <CardDescription>
                    {t('auth.verify.description')}
                </CardDescription>
            </CardHeader>
            <CardContent>
                <VerifyForm next={safeNext(sp.next)} />
            </CardContent>
        </Card>
    );
}
