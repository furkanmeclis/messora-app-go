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
import { api } from '@/lib/server/api';
import { safeNext } from '@/lib/server/actions';
import type { Schemas } from '@/lib/types';
import { LoginForm } from '../forms';

export const metadata: Metadata = { title: 'Sign in' };

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

    return (
        <Card>
            <CardHeader>
                <CardTitle>Sign in</CardTitle>
                <CardDescription>Welcome back to Messora.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
                {sp.reset && (
                    <Alert>
                        <CheckCircle2 />
                        <AlertDescription>
                            Your password was changed. Sign in with the new one.
                        </AlertDescription>
                    </Alert>
                )}
                {sp.expired && (
                    <Alert>
                        <Clock />
                        <AlertDescription>
                            The sign-in attempt expired. Please sign in again.
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
