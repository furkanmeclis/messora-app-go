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
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requireMe } from '@/lib/server/auth';
import type { SessionInfo } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { PasswordForm } from './password-form';
import { SessionList } from './session-list';
import { TwoFactorCard } from './two-factor';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('account.security.title') };
}

export default async function SecurityPage() {
    const [me, t] = await Promise.all([
        requireMe({ allowMfaPending: true }),
        getT(),
    ]);
    // Sessions are hidden while a mandatory second factor is pending (the API
    // restricts the session to enrolment).
    const sessions = me.mfa_required
        ? []
        : (await api<SessionInfo[]>('/me/sessions')).data;

    return (
        <>
            <PageHeading
                title={t('account.security.title')}
                description={t('account.security.description')}
            />
            {me.mfa_required && (
                <Alert className="mb-6 max-w-3xl">
                    <ShieldAlert />
                    <AlertTitle>
                        {t('account.security.mfaRequiredTitle')}
                    </AlertTitle>
                    <AlertDescription>
                        {t('account.security.mfaRequiredBody')}
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
                                <CardTitle>
                                    {t('account.password.title')}
                                </CardTitle>
                                <CardDescription>
                                    {t('account.password.description')}
                                </CardDescription>
                            </CardHeader>
                            <CardContent>
                                <PasswordForm />
                            </CardContent>
                        </Card>
                        <Card>
                            <CardHeader>
                                <CardTitle>
                                    {t('account.sessions.title')}
                                </CardTitle>
                                <CardDescription>
                                    {t('account.sessions.description')}
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
