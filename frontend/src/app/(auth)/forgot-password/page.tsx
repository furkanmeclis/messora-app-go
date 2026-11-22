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
import { api } from '@/lib/server/api';
import type { Schemas } from '@/lib/types';
import { ForgotPasswordForm } from '../forms';

export const metadata: Metadata = { title: 'Reset password' };

export default async function ForgotPasswordPage() {
    const caps = (
        await api<Schemas['AuthCapabilities']>('/auth/capabilities', {
            auth: 'none',
        })
    ).data;
    return (
        <Card>
            <CardHeader>
                <CardTitle>Reset your password</CardTitle>
                <CardDescription>
                    We will e-mail you a link to choose a new password.
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
                {caps.password_reset ? (
                    <ForgotPasswordForm />
                ) : (
                    <Alert>
                        <Info />
                        <AlertDescription>
                            E-mail is not configured on this installation. Ask
                            an owner or administrator to reset your access.
                        </AlertDescription>
                    </Alert>
                )}
                <p className="text-center text-sm">
                    <Link
                        href="/login"
                        className="text-muted-foreground underline-offset-4 hover:underline"
                    >
                        Back to sign in
                    </Link>
                </p>
            </CardContent>
        </Card>
    );
}
