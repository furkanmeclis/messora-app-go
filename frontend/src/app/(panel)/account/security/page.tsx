import type { Metadata } from 'next';
import { ShieldAlert } from 'lucide-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { api } from '@/lib/server/api';
import { requireMe } from '@/lib/server/auth';
import type { SessionInfo } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { PasswordForm } from './password-form';
import { SessionList } from './session-list';
import { TwoFactorCard } from './two-factor';

export const metadata: Metadata = { title: 'Security' };

export default async function SecurityPage() {
    const me = await requireMe({ allowMfaPending: true });
    // Sessions are hidden while a mandatory second factor is pending (the API
    // restricts the session to enrolment).
    const sessions = me.mfa_required
        ? []
        : (await api<SessionInfo[]>('/me/sessions')).data;

    return (
        <>
            <PageHeading
                title="Security"
                description="Password, two-factor authentication and active sessions."
            />
            {me.mfa_required && (
                <Alert className="mb-6 max-w-3xl">
                    <ShieldAlert />
                    <AlertTitle>
                        Two-factor authentication is required
                    </AlertTitle>
                    <AlertDescription>
                        Your role requires an authenticator app. Set it up below
                        to continue to the panel.
                    </AlertDescription>
                </Alert>
            )}
            <div className="grid max-w-3xl gap-6">
                <TwoFactorCard
                    enabled={me.user.mfa_enabled}
                    required={me.mfa_required}
                />
                {!me.mfa_required && (
                    <>
                        <Card>
                            <CardHeader>
                                <CardTitle>Password</CardTitle>
                                <CardDescription>
                                    Changing it signs you out everywhere else.
                                </CardDescription>
                            </CardHeader>
                            <CardContent>
                                <PasswordForm />
                            </CardContent>
                        </Card>
                        <Card>
                            <CardHeader>
                                <CardTitle>Active sessions</CardTitle>
                                <CardDescription>
                                    Devices currently signed in to your account.
                                </CardDescription>
                            </CardHeader>
                            <CardContent>
                                <SessionList sessions={sessions} />
                            </CardContent>
                        </Card>
                    </>
                )}
            </div>
        </>
    );
}
