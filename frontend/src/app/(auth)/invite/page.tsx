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
import { getT } from '@/i18n/server';
import { api, ApiError } from '@/lib/server/api';
import { errorMessage } from '@/lib/server/actions';
import type { Schemas } from '@/lib/types';
import { AcceptInvitationForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.invite.metaTitle') };
}

export default async function InvitePage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const t = await getT();
    const token = (await searchParams).token ?? '';
    let invitation: Schemas['InvitationPreview'] | null = null;
    let error = t('auth.invite.incomplete');
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
            error = await errorMessage(err);
        }
    }
    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('auth.invite.title')}</CardTitle>
                <CardDescription>
                    {invitation
                        ? t('auth.invite.invitedAs', {
                              role: invitation.role.name,
                          })
                        : t('auth.invite.accept')}
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
                            {error} {t('auth.invite.askAdmin')}
                        </AlertDescription>
                    </Alert>
                )}
            </CardContent>
        </Card>
    );
}
