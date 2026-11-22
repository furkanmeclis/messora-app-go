import type { Metadata } from 'next';
import { AlertCircle } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { api, ApiError } from '@/lib/server/api';
import type { Schemas } from '@/lib/types';
import { AcceptInvitationForm } from '../forms';

export const metadata: Metadata = { title: 'Accept invitation' };

export default async function InvitePage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const token = (await searchParams).token ?? '';
    let invitation: Schemas['InvitationPreview'] | null = null;
    let error = 'This invitation link is incomplete.';
    if (token) {
        try {
            invitation = (
                await api<Schemas['InvitationPreview']>('/invitations/lookup', {
                    method: 'POST',
                    auth: 'none',
                    body: { token },
                })
            ).data;
        } catch (err) {
            if (!(err instanceof ApiError)) throw err;
            error = err.message;
        }
    }
    return (
        <Card>
            <CardHeader>
                <CardTitle>Join Messora</CardTitle>
                <CardDescription>
                    {invitation
                        ? `You were invited as ${invitation.role.name}.`
                        : 'Accept your invitation.'}
                </CardDescription>
            </CardHeader>
            <CardContent>
                {invitation ? (
                    <AcceptInvitationForm
                        token={token}
                        email={invitation.email}
                    />
                ) : (
                    <Alert variant="destructive">
                        <AlertCircle />
                        <AlertDescription>
                            {error} Ask an administrator for a new invitation.
                        </AlertDescription>
                    </Alert>
                )}
            </CardContent>
        </Card>
    );
}
