import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { safeNext } from '@/lib/server/actions';
import { getMfaToken } from '@/lib/server/session';
import { VerifyForm } from '../../forms';

export const metadata: Metadata = { title: 'Two-factor authentication' };

export default async function VerifyPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    if (!(await getMfaToken())) redirect('/login');
    const sp = await searchParams;
    return (
        <Card>
            <CardHeader>
                <CardTitle>Two-factor authentication</CardTitle>
                <CardDescription>
                    Enter the 6-digit code from your authenticator app.
                </CardDescription>
            </CardHeader>
            <CardContent>
                <VerifyForm next={safeNext(sp.next)} />
            </CardContent>
        </Card>
    );
}
