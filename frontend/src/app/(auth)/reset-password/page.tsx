import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { ResetPasswordForm } from '../forms';

export const metadata: Metadata = { title: 'Choose a new password' };

export default async function ResetPasswordPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const token = (await searchParams).token;
    if (!token) redirect('/forgot-password');
    return (
        <Card>
            <CardHeader>
                <CardTitle>Choose a new password</CardTitle>
                <CardDescription>
                    You will be signed out on every device.
                </CardDescription>
            </CardHeader>
            <CardContent>
                <ResetPasswordForm token={token} />
            </CardContent>
        </Card>
    );
}
