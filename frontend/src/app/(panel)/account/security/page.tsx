import type { Metadata } from 'next';
import { KeyRound, Laptop, ShieldAlert } from 'lucide-react';
import { SecurityScore } from './security-score';
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
                <Alert className="mb-6">
                    <ShieldAlert />
                    <AlertTitle>
                        {t('account.security.mfaRequiredTitle')}
                    </AlertTitle>
                    <AlertDescription>
                        {t('account.security.mfaRequiredBody')}
                    </AlertDescription>
                </Alert>
            )}
            {!me.mfa_required && (
                <SecurityScore
                    mfa={me.user.mfa_enabled}
                    sessions={sessions.length}
                    title={t('account.security.score')}
                    levels={{
                        strong: t('account.security.scoreLevels.strong'),
                        fair: t('account.security.scoreLevels.fair'),
                        weak: t('account.security.scoreLevels.weak'),
                    }}
                    checks={[
                        {
                            ok: me.user.mfa_enabled,
                            label: me.user.mfa_enabled
                                ? t('account.security.checks.twoFactor')
                                : t('account.security.checks.twoFactorOff'),
                            href: '#two-factor',
                        },
                        {
                            ok: sessions.length <= 1,
                            label:
                                sessions.length <= 1
                                    ? t('account.security.checks.sessions')
                                    : t(
                                          'account.security.checks.sessionsMany',
                                          {
                                              count: sessions.length,
                                          },
                                      ),
                            href: '#sessions',
                        },
                    ]}
                />
            )}
            <div className="grid items-start gap-6 xl:grid-cols-2">
                <TwoFactorCard
                    enabled={me.user.mfa_enabled}
                    required={me.mfa_required}
                />
                {!me.mfa_required && (
                    <Card id="password" className="scroll-mt-20">
                        <CardHeader className="flex flex-row items-start gap-4">
                            <span className="flex size-11 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary">
                                <KeyRound className="size-5" />
                            </span>
                            <div className="space-y-1.5">
                                <CardTitle>
                                    {t('account.password.title')}
                                </CardTitle>
                                <CardDescription>
                                    {t('account.password.description')}
                                </CardDescription>
                            </div>
                        </CardHeader>
                        <CardContent>
                            <PasswordForm />
                        </CardContent>
                    </Card>
                )}
                {!me.mfa_required && (
                    <Card id="sessions" className="scroll-mt-20 xl:col-span-2">
                        <CardHeader className="flex flex-row items-start gap-4">
                            <span className="flex size-11 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary">
                                <Laptop className="size-5" />
                            </span>
                            <div className="space-y-1.5">
                                <CardTitle>
                                    {t('account.sessions.title')}
                                </CardTitle>
                                <CardDescription>
                                    {t('account.sessions.description')}
                                </CardDescription>
                            </div>
                        </CardHeader>
                        <CardContent>
                            <SessionList sessions={sessions} />
                        </CardContent>
                    </Card>
                )}
            </div>
        </>
    );
}
